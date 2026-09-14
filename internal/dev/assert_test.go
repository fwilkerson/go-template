package main

import (
	"net/http"
	"strings"
	"testing"
)

var sample = response{
	Status:  200,
	Body:    "<h1>Hello, Go!</h1>",
	MIME:    "text/html",
	Headers: http.Header{"X-Id": {"abc"}},
}

func TestRunScriptExpressions(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		`response.status === 200`:                         true,
		`response.status !== 200`:                         false,
		`response.status >= 200 && response.status < 300`: true,
		`response.status > 200 || response.status <= 100`: false,
		`!(response.status === 404)`:                      true,
		`response.body.includes("Hello, Go!")`:            true,
		`response.body.startsWith("<h1>")`:                true,
		`response.body.endsWith('</p>')`:                  false,
		`response.contentType.mimeType === "text/html"`:   true,
		`response.headers.valueOf("X-Id") === "abc"`:      true,
		`response.headers.valueOf("x-id").includes("b")`:  true,
		`response.headers.valueOf("Missing") === null`:    true,
		`response.headers.valueOf("Missing") !== null && response.headers.valueOf("Missing").includes("x")`: false,
		`"a".includes("a") === true`: true,
		`response.status === "200"`:  false, // === does not coerce
	}
	for expr, want := range tests {
		t.Run(expr, func(t *testing.T) {
			t.Parallel()

			got, err := runScript(`client.assert(`+expr+`, "m")`, 1, sample)
			if err != nil {
				t.Fatalf("runScript: %v", err)
			}
			if len(got) != 1 || got[0].OK != want || got[0].Expr != expr || got[0].Message != "m" {
				t.Errorf("got %+v, want OK=%v", got, want)
			}
		})
	}
}

func TestRunScriptShapes(t *testing.T) {
	t.Parallel()

	src := `
// arrow function, two asserts
client.test("arrow", () => {
    client.assert(response.status === 200, "status");
    client.assert(response.body.includes("nope"), "body")
});
/* classic function, no semicolons */
client.test("classic", function() {
    client.assert(true)
})
client.assert(false, "bare")
`
	got, err := runScript(src, 10, sample)
	if err != nil {
		t.Fatalf("runScript: %v", err)
	}
	want := []assertion{
		{Test: "arrow", Message: "status", Expr: "response.status === 200", Line: 13, OK: true},
		{Test: "arrow", Message: "body", Expr: `response.body.includes("nope")`, Line: 14, OK: false},
		{Test: "classic", Expr: "true", Line: 18, OK: true},
		{Message: "bare", Expr: "false", Line: 20, OK: false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d assertions, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("assertion %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRunScriptRejectsWhatItCannotRun(t *testing.T) {
	t.Parallel()

	tests := map[string]struct{ src, want string }{
		"client.log":      {`client.log("x")`, "client.log is not supported"},
		"client.global":   {`client.test("t", () => { client.global.set("a", 1) })`, "only client.assert"},
		"json body":       {`client.assert(response.body.title === "x")`, ".title is not supported"},
		"headers.all":     {`client.assert(response.headers.all() === null)`, "headers.all is not supported"},
		"charset":         {`client.assert(response.contentType.charset === "utf-8")`, "contentType.charset is not supported"},
		"loose equality":  {`client.assert(response.status == 200)`, `unexpected "="`},
		"non-boolean":     {`client.assert(response.status)`, "not a boolean"},
		"includes on num": {`client.assert(response.status.includes("2"))`, "expected"},
		"unclosed string": {`client.assert(response.body.includes("x))`, "string is not closed"},
		"trailing junk":   {`client.assert(true) foo`, "expected client.test or client.assert"},
		"line numbers":    {"\n\nclient.assert(response.nothing)", "line 7:"},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			_, err := runScript(tc.src, 5, sample)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
