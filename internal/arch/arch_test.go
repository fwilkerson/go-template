// Package arch holds the test that enforces the repository layout: which
// packages may import which, and that the standard library is used where it
// has replaced a third-party module. It has no code of its own; the rules are
// the test.
package arch_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The layout, as the test sees it:
//
//   - cmd/<name>          binaries; may import anything.
//   - internal/web        the shell; imports no other internal package.
//   - internal/<feature>  a package that exports Routes; it takes a *http.ServeMux
//     and imports the shell and concept packages, never another feature.
//   - internal/<concept>  any other internal package; imports neither features nor the shell.
//   - internal/cmd/<tool> repository tooling; imports no other internal package.
//
// Package names are not what the rules key on, so a feature can be called
// anything; registering routes is what makes it a feature.
const shell = "internal/web"

var forbiddenNames = []string{"pkg", "utils", "util", "common", "models", "helpers"}

// stdlibNames are package names the standard library has taken over from
// third-party modules since the go directive in go.mod. A package outside the
// standard library with one of these names is the module it replaced, whoever
// publishes it.
var stdlibNames = []string{"uuid", "errors", "slices", "maps"}

type pkg struct {
	ImportPath string   `json:"ImportPath"`
	Name       string   `json:"Name"`
	Standard   bool     `json:"Standard"`
	Dir        string   `json:"Dir"`
	GoFiles    []string `json:"GoFiles"`
	Imports    []string `json:"Imports"`
}

type kind int

const (
	other kind = iota
	binary
	shellPkg
	feature
	concept
	tool
	arch
)

func TestLayout(t *testing.T) {
	t.Parallel()

	module, pkgs, deps := load(t)
	kinds := make(map[string]kind, len(pkgs))
	for _, p := range pkgs {
		kinds[p.ImportPath] = classify(t, module, p)
	}

	for _, p := range pkgs {
		rel := strings.TrimPrefix(p.ImportPath, module+"/")
		for elem := range strings.SplitSeq(rel, "/") {
			if slices.Contains(forbiddenNames, elem) {
				t.Errorf("%s: %q is not a package name; name packages for what they provide", rel, elem)
			}
		}
		from := kinds[p.ImportPath]
		for _, imp := range p.Imports {
			if d := deps[imp]; !d.Standard && slices.Contains(stdlibNames, d.Name) {
				t.Errorf("%s imports %s: the standard library %s package replaces it", rel, imp, d.Name)
			}
			to, internal := kinds[imp]
			if !internal || imp == p.ImportPath {
				continue
			}
			if reason := violation(from, to); reason != "" {
				t.Errorf("%s imports %s: %s", rel, strings.TrimPrefix(imp, module+"/"), reason)
			}
		}
	}
}

// violation says why a package of kind from may not import one of kind to.
func violation(from, to kind) string {
	switch from {
	case shellPkg:
		return "the shell imports no other internal package"
	case feature:
		if to == feature {
			return "features do not import each other; move the shared type to a concept package"
		}
	case concept:
		if to == feature || to == shellPkg {
			return "concept packages import neither features nor the shell"
		}
	case tool:
		return "repository tooling imports no other internal package"
	}
	return ""
}

func classify(t *testing.T, module string, p pkg) kind {
	t.Helper()
	rel := strings.TrimPrefix(p.ImportPath, module+"/")
	switch {
	case rel == "internal/arch":
		return arch
	case strings.HasPrefix(rel, "cmd/"):
		return binary
	case rel == shell:
		return shellPkg
	case strings.HasPrefix(rel, "internal/cmd/"):
		return tool
	case strings.HasPrefix(rel, "internal/"):
		fn := routesFunc(t, p)
		if fn == nil {
			return concept
		}
		if !takesServeMux(fn) {
			t.Errorf("%s: Routes takes %s; features register on a *http.ServeMux, the standard library router", rel, paramTypes(fn))
		}
		return feature
	}
	return other
}

// routesFunc returns the package's top-level Routes function, which is what
// makes it a feature, or nil when it has none.
func routesFunc(t *testing.T, p pkg) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	for _, name := range p.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "Routes" {
				return fn
			}
		}
	}
	return nil
}

// takesServeMux reports whether fn's only parameter is a *http.ServeMux.
func takesServeMux(fn *ast.FuncDecl) bool {
	params := fn.Type.Params.List
	if len(params) != 1 || len(params[0].Names) > 1 {
		return false
	}
	star, ok := params[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "http" && sel.Sel.Name == "ServeMux"
}

// paramTypes renders fn's parameter list for an error message.
func paramTypes(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("(")
	for i, f := range fn.Type.Params.List {
		if i > 0 {
			b.WriteString(", ")
		}
		if err := printer.Fprint(&b, token.NewFileSet(), f.Type); err != nil {
			b.WriteString("?")
		}
	}
	b.WriteString(")")
	return b.String()
}

// load returns the module path, every package in the module, and every
// package those depend on keyed by import path, from go list.
func load(t *testing.T) (string, []pkg, map[string]pkg) {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	root := strings.TrimSpace(string(out))
	touchTree(t, root)
	// -e keeps listing through a broken package, such as an import cycle, so the
	// rule that caused it is reported rather than the listing failure.
	cmd := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,Name,Standard,Dir,GoFiles,Imports", "./...")
	cmd.Dir = root
	out, err = cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("go list: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("go list: %v", err)
	}
	module, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	mod := strings.TrimSpace(string(module))
	var pkgs []pkg
	deps := map[string]pkg{}
	dec := jsontext.NewDecoder(strings.NewReader(string(out)))
	for {
		var p pkg
		err := json.UnmarshalDecode(dec, &p)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		deps[p.ImportPath] = p
		if p.ImportPath == mod || strings.HasPrefix(p.ImportPath, mod+"/") {
			pkgs = append(pkgs, p)
		}
	}
	return mod, pkgs, deps
}

// touchTree opens every directory and Go file under root. The test cache
// records what a test opens and reruns it when any of that changes; the go
// list output alone would leave a cached pass in place after an import or a
// package is added.
func touchTree(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); p != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			_, err := os.ReadDir(p)
			return err
		}
		if strings.HasSuffix(p, ".go") {
			_, err := os.ReadFile(p)
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func TestTakesServeMux(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"func Routes(mux *http.ServeMux)":         true,
		"func Routes(m *http.ServeMux, s *Store)": false,
		"func Routes(mux http.ServeMux)":          false,
		"func Routes(r chi.Router)":               false,
		"func Routes(r *gin.Engine)":              false,
		"func Routes()":                           false,
		"func Routes(a, b *http.ServeMux)":        false,
	}
	for src, want := range cases {
		f, err := parser.ParseFile(token.NewFileSet(), "", "package p\n"+src+" {}", parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		if got := takesServeMux(f.Decls[0].(*ast.FuncDecl)); got != want {
			t.Errorf("takesServeMux(%s) = %v, want %v", src, got, want)
		}
	}
}

// TestViolation pins the rule table itself.
func TestViolation(t *testing.T) {
	t.Parallel()

	allowed := []struct{ from, to kind }{
		{binary, feature},
		{binary, shellPkg},
		{binary, concept},
		{feature, shellPkg},
		{feature, concept},
		{concept, concept},
	}
	for _, c := range allowed {
		if v := violation(c.from, c.to); v != "" {
			t.Errorf("violation(%d, %d) = %q, want allowed", c.from, c.to, v)
		}
	}
	denied := []struct{ from, to kind }{
		{shellPkg, feature},
		{shellPkg, concept},
		{feature, feature},
		{concept, feature},
		{concept, shellPkg},
		{tool, shellPkg},
		{tool, concept},
	}
	for _, c := range denied {
		if violation(c.from, c.to) == "" {
			t.Errorf("violation(%d, %d) allowed, want denied", c.from, c.to)
		}
	}
}
