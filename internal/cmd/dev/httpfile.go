package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// request is one request block of a .http file: everything between two ###
// separators.
type request struct {
	Name       string
	Line       int // line of the request line, 1-based
	Method     string
	URL        string
	Headers    [][2]string
	Body       string
	Script     string // response handler source, without the {% %} markers
	ScriptLine int    // line the handler opens on, for messages
}

var methods = []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "TRACE"}

// parseHTTPFile reads the subset of the JetBrains .http format that just e2e
// runs: ### separators with an optional name, # @name, comments, a request
// line, headers, a body and one inline response handler. Anything else the
// IDE would accept but this runner would silently ignore is an error, so a
// file that passes here behaves the same in GoLand.
func parseHTTPFile(src string) ([]request, error) {
	lines := strings.Split(src, "\n")
	var reqs []request
	i := 0
	for i < len(lines) {
		name, next, err := parsePreamble(lines, i)
		if err != nil {
			return nil, err
		}
		i = next
		if i >= len(lines) {
			break
		}
		req, next, err := parseRequest(lines, i)
		if err != nil {
			return nil, err
		}
		req.Name = name
		reqs = append(reqs, req)
		i = next
	}
	return reqs, nil
}

// parsePreamble consumes separators, comments and blank lines before a
// request and returns the name they gave it, if any.
func parsePreamble(lines []string, i int) (string, int, error) {
	var name string
	for ; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		switch {
		case l == "":
		case strings.HasPrefix(l, "###"):
			if n := strings.TrimSpace(strings.TrimLeft(l, "#")); n != "" {
				name = n
			}
		case strings.HasPrefix(l, "#") || strings.HasPrefix(l, "//"):
			rest := strings.TrimSpace(strings.TrimLeft(l, "#/"))
			if !strings.HasPrefix(rest, "@") {
				continue
			}
			tag, value, _ := strings.Cut(rest, " ")
			if tag != "@name" {
				return "", 0, fmt.Errorf("line %d: %s is not supported by just e2e", i+1, tag)
			}
			name = strings.TrimSpace(strings.TrimPrefix(value, "="))
		default:
			return name, i, nil
		}
	}
	return name, i, nil
}

// parseRequest reads one request starting at the request line.
func parseRequest(lines []string, i int) (request, int, error) {
	req := request{Line: i + 1, Method: "GET"}
	fields := strings.Fields(lines[i])
	if len(fields) >= 2 && slices.Contains(methods, fields[0]) {
		req.Method, req.URL = fields[0], fields[1]
	} else {
		req.URL = fields[0]
	}
	i++

	// Headers run to the first blank line.
	for ; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if l == "" || strings.HasPrefix(l, "###") || strings.HasPrefix(l, ">") {
			break
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			return req, 0, fmt.Errorf("line %d: expected a header or a blank line before the body", i+1)
		}
		req.Headers = append(req.Headers, [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
	}

	// The body runs to the handler or the next separator.
	var body []string
	for ; i < len(lines); i++ {
		l := strings.TrimSpace(lines[i])
		if strings.HasPrefix(l, "###") || strings.HasPrefix(l, ">") {
			break
		}
		if strings.HasPrefix(l, "<") {
			return req, 0, fmt.Errorf("line %d: a body read from a file is not supported by just e2e", i+1)
		}
		body = append(body, lines[i])
	}
	req.Body = strings.Trim(strings.Join(body, "\n"), "\r\n")

	if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">") {
		script, next, err := parseHandler(lines, i)
		if err != nil {
			return req, 0, err
		}
		req.Script, req.ScriptLine = script, i+1
		i = next
		for ; i < len(lines); i++ {
			l := strings.TrimSpace(lines[i])
			if strings.HasPrefix(l, "###") {
				break
			}
			if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "//") {
				continue
			}
			return req, 0, fmt.Errorf("line %d: requests are separated by ###", i+1)
		}
	}
	return req, i, nil
}

// parseHandler reads an inline `> {% ... %}` block and returns its source.
func parseHandler(lines []string, i int) (string, int, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"))
	if !strings.HasPrefix(rest, "{%") {
		return "", 0, fmt.Errorf("line %d: only an inline {%% %%} handler is supported by just e2e", i+1)
	}
	rest = strings.TrimPrefix(rest, "{%")
	var script []string
	for ; i < len(lines); i++ {
		if before, _, ok := strings.Cut(rest, "%}"); ok {
			script = append(script, before)
			return strings.Join(script, "\n"), i + 1, nil
		}
		script = append(script, rest)
		if i+1 < len(lines) {
			rest = lines[i+1]
		}
	}
	return "", 0, fmt.Errorf("line %d: handler is not closed with %%}", i+1)
}

var variable = regexp.MustCompile(`\{\{\s*([^}]*?)\s*\}\}`)

// expand replaces {{name}} references from vars. JetBrains dynamic variables
// such as {{$uuid}} and unknown names are errors rather than passed through.
func expand(s string, vars map[string]string) (string, error) {
	var err error
	out := variable.ReplaceAllStringFunc(s, func(m string) string {
		name := variable.FindStringSubmatch(m)[1]
		v, ok := vars[name]
		switch {
		case strings.HasPrefix(name, "$"):
			err = fmt.Errorf("dynamic variable {{%s}} is not supported by just e2e", name)
		case !ok:
			err = fmt.Errorf("variable {{%s}} is not defined in http-client.env.json", name)
		}
		return v
	})
	return out, err
}
