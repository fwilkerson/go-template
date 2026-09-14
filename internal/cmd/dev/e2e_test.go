package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<p>Hello</p>")
	})
	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Retries", r.Header.Get("X-Retries"))
		_, _ = io.Copy(w, r.Body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRunFile(t *testing.T) {
	t.Parallel()

	srv := testServer(t)
	got, err := runFile(context.Background(), srv.Client(), "testdata/e2e/passing.http", "dev", srv.URL)
	if err != nil {
		t.Fatalf("runFile: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d assertions, want 5: %+v", len(got), got)
	}
	for _, a := range got {
		if !a.OK {
			t.Errorf("%d: %s: %s failed", a.Line, a.label(), a.Expr)
		}
	}
}

func TestRunFileReportsFailures(t *testing.T) {
	t.Parallel()

	srv := testServer(t)
	got, err := runFile(context.Background(), srv.Client(), "testdata/e2e/failing.http", "dev", srv.URL)
	if err != nil {
		t.Fatalf("runFile: %v", err)
	}
	if len(got) != 2 || got[0].OK || !got[1].OK {
		t.Fatalf("got %+v, want the first assertion failed and the second passed", got)
	}
	if got[0].Line != 6 || got[0].label() != "page: wrong status" {
		t.Errorf("failure reported as line %d %q", got[0].Line, got[0].label())
	}
}

func TestRunFileRequiresAssertions(t *testing.T) {
	t.Parallel()

	srv := testServer(t)
	_, err := runFile(context.Background(), srv.Client(), "testdata/e2e/silent.http", "dev", srv.URL)
	if err == nil || !strings.Contains(err.Error(), "No handler has no assertions") {
		t.Fatalf("err = %v, want a request without a handler to be refused", err)
	}
}

func TestLoadEnv(t *testing.T) {
	t.Parallel()

	vars, err := loadEnv("testdata/e2e", "dev")
	if err != nil {
		t.Fatalf("loadEnv: %v", err)
	}
	if vars["greeting"] != "Hello" || vars["retries"] != "3" {
		t.Errorf("vars = %v", vars)
	}
	if vars, err := loadEnv("testdata/e2e", "prod"); err != nil || len(vars) != 0 {
		t.Errorf("unknown env: %v, %v; want empty", vars, err)
	}
	if vars, err := loadEnv("testdata", "dev"); err != nil || len(vars) != 0 {
		t.Errorf("no env file: %v, %v; want empty", vars, err)
	}
}

func TestOnlyBinary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := onlyBinary(root); err == nil {
		t.Error("onlyBinary: expected an error for an empty directory")
	}
	if err := os.MkdirAll(filepath.Join(root, "memo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := onlyBinary(root)
	if err != nil {
		t.Fatalf("onlyBinary: %v", err)
	}
	if want := "./" + filepath.Join(root, "memo"); got != want {
		t.Errorf("onlyBinary = %q, want %q", got, want)
	}
	if err := os.MkdirAll(filepath.Join(root, "worker"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := onlyBinary(root); err == nil || !strings.Contains(err.Error(), "-server") {
		t.Errorf("onlyBinary with two packages: err = %v, want a hint to pass -server", err)
	}
}
