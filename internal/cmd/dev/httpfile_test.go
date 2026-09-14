package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseHTTPFile(t *testing.T) {
	t.Parallel()

	src := strings.Join([]string{
		"# leading comment",
		"",
		"### The page",
		"GET {{host}}/",
		"",
		"> {%",
		`client.assert(response.status === 200, "ok");`,
		"%}",
		"",
		"###",
		"# @name greet",
		"POST {{host}}/greet HTTP/1.1",
		"Content-Type: application/x-www-form-urlencoded",
		"",
		"name=Go",
		"",
		"> {% client.assert(true) %}",
		"###",
		"{{host}}/bare",
		"",
	}, "\n")

	reqs, err := parseHTTPFile(src)
	if err != nil {
		t.Fatalf("parseHTTPFile: %v", err)
	}
	if len(reqs) != 3 {
		t.Fatalf("got %d requests, want 3", len(reqs))
	}
	page, greet, bare := reqs[0], reqs[1], reqs[2]

	if page.Name != "The page" || page.Method != "GET" || page.URL != "{{host}}/" || page.Line != 4 {
		t.Errorf("page = %+v", page)
	}
	if page.ScriptLine != 6 || !strings.Contains(page.Script, "client.assert(response.status === 200") {
		t.Errorf("page script = %q at line %d", page.Script, page.ScriptLine)
	}
	if greet.Name != "greet" || greet.Method != "POST" || greet.URL != "{{host}}/greet" {
		t.Errorf("greet = %+v", greet)
	}
	if !slices.Equal(greet.Headers, [][2]string{{"Content-Type", "application/x-www-form-urlencoded"}}) {
		t.Errorf("greet headers = %v", greet.Headers)
	}
	if greet.Body != "name=Go" {
		t.Errorf("greet body = %q, want trailing newlines trimmed", greet.Body)
	}
	if strings.TrimSpace(greet.Script) != "client.assert(true)" {
		t.Errorf("greet script = %q", greet.Script)
	}
	if bare.Method != "GET" || bare.URL != "{{host}}/bare" || bare.Script != "" || bare.Name != "" {
		t.Errorf("bare = %+v", bare)
	}
}

func TestParseHTTPFileRejectsWhatItCannotRun(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ src, want string }{
		"external script": {"GET /\n\n> handler.js\n", "inline"},
		"body from file":  {"POST /\n\n< ./body.json\n", "body read from a file"},
		"unknown tag":     {"# @no-redirect\nGET /\n", "@no-redirect is not supported"},
		"unclosed":        {"GET /\n\n> {%\nclient.assert(true)\n", "not closed"},
		"missing ###":     {"GET /\n\n> {% client.assert(true) %}\nGET /other\n", "separated by ###"},
		"bad header":      {"GET /\nnot a header\n", "expected a header"},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			_, err := parseHTTPFile(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestExpand(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"host": "http://h", "id": "7"}
	got, err := expand("{{host}}/x/{{ id }}", vars)
	if err != nil || got != "http://h/x/7" {
		t.Errorf("expand = %q, %v", got, err)
	}
	if _, err := expand("{{nope}}", vars); err == nil || !strings.Contains(err.Error(), "{{nope}}") {
		t.Errorf("undefined variable: err = %v", err)
	}
	if _, err := expand("{{$uuid}}", vars); err == nil || !strings.Contains(err.Error(), "dynamic") {
		t.Errorf("dynamic variable: err = %v", err)
	}
}
