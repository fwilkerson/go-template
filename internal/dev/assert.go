package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

// The response handler of a .http request is JavaScript run by the JetBrains
// HTTP Client. just e2e runs the subset below with the same meaning, so one
// file serves GoLand and `just check`. A script outside the subset fails
// with a message rather than passing vacuously.
//
//	client.test("name", () => { ... })      // or function() { ... }
//	client.assert(<expr>, "message")        // inside a test or bare
//
//	response.status                         // number
//	response.body                           // string
//	response.contentType.mimeType           // string, e.g. "text/html"
//	response.headers.valueOf("Name")        // string, or null when absent
//	<string>.includes | startsWith | endsWith (<expr>)
//	===  !==  <  <=  >  >=  &&  ||  !  ( )  null  true  false  "..." '...' 123

// response is what the handler script can inspect.
type response struct {
	Status  int
	Body    string
	MIME    string
	Headers http.Header
}

// assertion is one client.assert call and its outcome.
type assertion struct {
	Test    string // enclosing client.test name; empty for a bare assert
	Message string
	Expr    string // source text of the asserted expression
	Line    int    // 1-based line within the file
	OK      bool
}

// runScript parses and evaluates a handler; line is the file line the
// handler opened on, so reported lines match the .http file.
func runScript(src string, line int, resp response) ([]assertion, error) {
	toks, err := tokenize(src, line)
	if err != nil {
		return nil, err
	}
	p := &parser{src: src, toks: toks, resp: resp}
	for !p.at(tEOF) {
		if err := p.statement(); err != nil {
			return nil, err
		}
	}
	return p.asserts, nil
}

type tkind int

const (
	tEOF tkind = iota
	tIdent
	tString
	tNumber
	tPunct
)

type token struct {
	kind tkind
	text string
	line int
	off  int // byte offset in the script source
	end  int
}

var puncts = []string{"===", "!==", "&&", "||", "=>", "<=", ">=", "(", ")", "{", "}", ",", ";", ".", "!", "<", ">"}

func tokenize(src string, line int) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case unicode.IsSpace(rune(c)):
			i++
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("line %d: comment is not closed", line)
			}
			line += strings.Count(src[i:i+end], "\n")
			i += end + 2
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(src) && src[j] != c && src[j] != '\n' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(src) || src[j] != c {
				return nil, fmt.Errorf("line %d: string is not closed", line)
			}
			text, err := unquote(src[i+1 : j])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			toks = append(toks, token{tString, text, line, i, j + 1})
			i = j + 1
		case c >= '0' && c <= '9':
			j := i
			for j < len(src) && (src[j] >= '0' && src[j] <= '9' || src[j] == '.') {
				j++
			}
			toks = append(toks, token{tNumber, src[i:j], line, i, j})
			i = j
		case c == '_' || c == '$' || unicode.IsLetter(rune(c)):
			j := i
			for j < len(src) && (src[j] == '_' || src[j] == '$' || unicode.IsLetter(rune(src[j])) || unicode.IsDigit(rune(src[j]))) {
				j++
			}
			toks = append(toks, token{tIdent, src[i:j], line, i, j})
			i = j
		default:
			p := ""
			for _, cand := range puncts {
				if strings.HasPrefix(src[i:], cand) {
					p = cand
					break
				}
			}
			if p == "" {
				return nil, fmt.Errorf("line %d: unexpected %q", line, string(c))
			}
			toks = append(toks, token{tPunct, p, line, i, i + len(p)})
			i += len(p)
		}
	}
	toks = append(toks, token{tEOF, "", line, len(src), len(src)})
	return toks, nil
}

// unquote handles the escapes a test string plausibly needs.
func unquote(s string) (string, error) {
	if !strings.Contains(s, `\`) {
		return s, nil
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case '\\', '"', '\'':
			b.WriteByte(s[i])
		default:
			return "", fmt.Errorf("unsupported escape \\%c", s[i])
		}
	}
	return b.String(), nil
}

type parser struct {
	src     string
	toks    []token
	pos     int
	resp    response
	asserts []assertion
}

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

func (p *parser) at(kind tkind) bool { return p.peek().kind == kind }

func (p *parser) is(punct string) bool {
	t := p.peek()
	return t.kind == tPunct && t.text == punct
}

func (p *parser) accept(punct string) bool {
	if p.is(punct) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) expect(punct string) error {
	if !p.accept(punct) {
		return p.errorf("expected %q", punct)
	}
	return nil
}

func (p *parser) ident() (string, error) {
	t := p.peek()
	if t.kind != tIdent {
		return "", p.errorf("expected a name")
	}
	p.pos++
	return t.text, nil
}

func (p *parser) errorf(format string, args ...any) error {
	t := p.peek()
	got := t.text
	if t.kind == tEOF {
		got = "end of script"
	}
	return fmt.Errorf("line %d: %s, got %q", t.line, fmt.Sprintf(format, args...), got)
}

// statement parses client.test(...) or a bare client.assert(...).
func (p *parser) statement() error {
	method, err := p.clientMethod()
	if err != nil {
		return err
	}
	switch method {
	case "test":
		return p.test()
	case "assert":
		return p.assert("")
	}
	return fmt.Errorf("line %d: client.%s is not supported by just e2e; use client.test and client.assert", p.peek().line, method)
}

func (p *parser) clientMethod() (string, error) {
	t := p.peek()
	if t.kind != tIdent || t.text != "client" {
		return "", p.errorf("expected client.test or client.assert")
	}
	p.pos++
	if err := p.expect("."); err != nil {
		return "", err
	}
	return p.ident()
}

func (p *parser) test() error {
	if err := p.expect("("); err != nil {
		return err
	}
	name := p.peek()
	if name.kind != tString {
		return p.errorf("expected the test name as a string")
	}
	p.pos++
	if err := p.expect(","); err != nil {
		return err
	}
	if p.peek().kind == tIdent && p.peek().text == "function" {
		p.pos++
		if err := p.expectAll("(", ")"); err != nil {
			return err
		}
	} else if err := p.expectAll("(", ")", "=>"); err != nil {
		return err
	}
	if err := p.expect("{"); err != nil {
		return err
	}
	for !p.is("}") {
		method, err := p.clientMethod()
		if err != nil {
			return err
		}
		if method != "assert" {
			return fmt.Errorf("line %d: only client.assert is supported inside a test", p.peek().line)
		}
		if err := p.assert(name.text); err != nil {
			return err
		}
	}
	if err := p.expectAll("}", ")"); err != nil {
		return err
	}
	p.accept(";")
	return nil
}

func (p *parser) expectAll(puncts ...string) error {
	for _, s := range puncts {
		if err := p.expect(s); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) assert(test string) error {
	if err := p.expect("("); err != nil {
		return err
	}
	start := p.peek()
	expr, err := p.or()
	if err != nil {
		return err
	}
	end := p.toks[p.pos-1]
	a := assertion{Test: test, Line: start.line, Expr: strings.TrimSpace(p.src[start.off:end.end])}
	if p.accept(",") {
		msg := p.peek()
		if msg.kind != tString {
			return p.errorf("expected the assertion message as a string")
		}
		p.pos++
		a.Message = msg.text
	}
	if err := p.expect(")"); err != nil {
		return err
	}
	p.accept(";")
	v, err := expr.eval(p.resp)
	if err != nil {
		return fmt.Errorf("line %d: %w", a.Line, err)
	}
	if v.kind != kBool {
		return fmt.Errorf("line %d: %s is not a boolean", a.Line, a.Expr)
	}
	a.OK = v.b
	p.asserts = append(p.asserts, a)
	return nil
}

// Expression grammar, lowest precedence first.

func (p *parser) or() (node, error) {
	left, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.accept("||") {
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		left = binary{"||", left, right}
	}
	return left, nil
}

func (p *parser) and() (node, error) {
	left, err := p.equality()
	if err != nil {
		return nil, err
	}
	for p.accept("&&") {
		right, err := p.equality()
		if err != nil {
			return nil, err
		}
		left = binary{"&&", left, right}
	}
	return left, nil
}

func (p *parser) equality() (node, error) {
	left, err := p.relational()
	if err != nil {
		return nil, err
	}
	for p.is("===") || p.is("!==") {
		op := p.next().text
		right, err := p.relational()
		if err != nil {
			return nil, err
		}
		left = binary{op, left, right}
	}
	return left, nil
}

func (p *parser) relational() (node, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for p.is("<") || p.is("<=") || p.is(">") || p.is(">=") {
		op := p.next().text
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		left = binary{op, left, right}
	}
	return left, nil
}

func (p *parser) unary() (node, error) {
	if p.accept("!") {
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return not{x}, nil
	}
	return p.primary()
}

func (p *parser) primary() (node, error) {
	t := p.peek()
	switch t.kind {
	case tString:
		p.pos++
		return p.methods(lit{value{kind: kStr, s: t.text}})
	case tNumber:
		p.pos++
		n, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, p.errorf("bad number")
		}
		return lit{value{kind: kNum, n: n}}, nil
	case tPunct:
		if err := p.expect("("); err != nil {
			return nil, err
		}
		x, err := p.or()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return p.methods(x)
	case tIdent:
		switch t.text {
		case "true", "false":
			p.pos++
			return lit{value{kind: kBool, b: t.text == "true"}}, nil
		case "null":
			p.pos++
			return lit{value{kind: kNull}}, nil
		case "response":
			return p.responseRef()
		}
	}
	return nil, p.errorf("expected an expression")
}

// responseRef parses response.<field> and any string method on it.
func (p *parser) responseRef() (node, error) {
	p.pos++
	if err := p.expect("."); err != nil {
		return nil, err
	}
	field, err := p.ident()
	if err != nil {
		return nil, err
	}
	switch field {
	case "status":
		return ref{field}, nil
	case "body":
		return p.methods(ref{field})
	case "contentType":
		if err := p.expect("."); err != nil {
			return nil, err
		}
		sub, err := p.ident()
		if err != nil {
			return nil, err
		}
		if sub != "mimeType" {
			return nil, fmt.Errorf("line %d: response.contentType.%s is not supported by just e2e; use mimeType", p.peek().line, sub)
		}
		return p.methods(ref{"mimeType"})
	case "headers":
		if err := p.expect("."); err != nil {
			return nil, err
		}
		sub, err := p.ident()
		if err != nil {
			return nil, err
		}
		if sub != "valueOf" {
			return nil, fmt.Errorf("line %d: response.headers.%s is not supported by just e2e; use valueOf", p.peek().line, sub)
		}
		if err := p.expect("("); err != nil {
			return nil, err
		}
		name := p.peek()
		if name.kind != tString {
			return nil, p.errorf("expected the header name as a string")
		}
		p.pos++
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return p.methods(header{name.text})
	}
	return nil, fmt.Errorf("line %d: response.%s is not supported by just e2e", p.peek().line, field)
}

// methods parses trailing .includes(...), .startsWith(...) and .endsWith(...).
func (p *parser) methods(recv node) (node, error) {
	for p.accept(".") {
		name, err := p.ident()
		if err != nil {
			return nil, err
		}
		switch name {
		case "includes", "startsWith", "endsWith":
		default:
			return nil, fmt.Errorf("line %d: .%s is not supported by just e2e; use includes, startsWith or endsWith", p.peek().line, name)
		}
		if err := p.expect("("); err != nil {
			return nil, err
		}
		arg, err := p.or()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		recv = call{name, recv, arg}
	}
	return recv, nil
}

// Values and evaluation.

type kind int

const (
	kNull kind = iota
	kStr
	kNum
	kBool
)

type value struct {
	kind kind
	s    string
	n    float64
	b    bool
}

func (v value) String() string {
	switch v.kind {
	case kStr:
		return strconv.Quote(v.s)
	case kNum:
		return strconv.FormatFloat(v.n, 'f', -1, 64)
	case kBool:
		return strconv.FormatBool(v.b)
	}
	return "null"
}

func (v value) equal(w value) bool {
	return v.kind == w.kind && v.s == w.s && v.n == w.n && v.b == w.b
}

type node interface {
	eval(resp response) (value, error)
}

type lit struct{ v value }

func (l lit) eval(response) (value, error) { return l.v, nil }

type ref struct{ field string }

func (r ref) eval(resp response) (value, error) {
	switch r.field {
	case "status":
		return value{kind: kNum, n: float64(resp.Status)}, nil
	case "body":
		return value{kind: kStr, s: resp.Body}, nil
	}
	return value{kind: kStr, s: resp.MIME}, nil
}

type header struct{ name string }

func (h header) eval(resp response) (value, error) {
	if v := resp.Headers.Values(h.name); len(v) > 0 {
		return value{kind: kStr, s: v[0]}, nil
	}
	return value{kind: kNull}, nil
}

type call struct {
	method string
	recv   node
	arg    node
}

func (c call) eval(resp response) (value, error) {
	recv, err := c.recv.eval(resp)
	if err != nil {
		return value{}, err
	}
	if recv.kind != kStr {
		return value{}, fmt.Errorf(".%s called on %s, not a string", c.method, recv)
	}
	arg, err := c.arg.eval(resp)
	if err != nil {
		return value{}, err
	}
	if arg.kind != kStr {
		return value{}, fmt.Errorf(".%s needs a string argument, got %s", c.method, arg)
	}
	var b bool
	switch c.method {
	case "includes":
		b = strings.Contains(recv.s, arg.s)
	case "startsWith":
		b = strings.HasPrefix(recv.s, arg.s)
	case "endsWith":
		b = strings.HasSuffix(recv.s, arg.s)
	}
	return value{kind: kBool, b: b}, nil
}

type not struct{ x node }

func (n not) eval(resp response) (value, error) {
	v, err := n.x.eval(resp)
	if err != nil {
		return value{}, err
	}
	if v.kind != kBool {
		return value{}, fmt.Errorf("! applied to %s, not a boolean", v)
	}
	return value{kind: kBool, b: !v.b}, nil
}

type binary struct {
	op          string
	left, right node
}

var errNotBool = errors.New("operand is not a boolean")

func (b binary) eval(resp response) (value, error) {
	l, err := b.left.eval(resp)
	if err != nil {
		return value{}, err
	}
	switch b.op {
	case "&&", "||":
		if l.kind != kBool {
			return value{}, fmt.Errorf("%s: %s: %w", b.op, l, errNotBool)
		}
		// Short-circuit as JavaScript does, so a guard like
		// valueOf("X") !== null && ... protects the right side.
		if (b.op == "&&") != l.b {
			return l, nil
		}
		r, err := b.right.eval(resp)
		if err != nil {
			return value{}, err
		}
		if r.kind != kBool {
			return value{}, fmt.Errorf("%s: %s: %w", b.op, r, errNotBool)
		}
		return r, nil
	}
	r, err := b.right.eval(resp)
	if err != nil {
		return value{}, err
	}
	switch b.op {
	case "===":
		return value{kind: kBool, b: l.equal(r)}, nil
	case "!==":
		return value{kind: kBool, b: !l.equal(r)}, nil
	}
	if l.kind != kNum || r.kind != kNum {
		return value{}, fmt.Errorf("%s compares numbers, got %s and %s", b.op, l, r)
	}
	var res bool
	switch b.op {
	case "<":
		res = l.n < r.n
	case "<=":
		res = l.n <= r.n
	case ">":
		res = l.n > r.n
	case ">=":
		res = l.n >= r.n
	}
	return value{kind: kBool, b: res}, nil
}
