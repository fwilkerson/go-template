package greet_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/fwilkerson/go-template/internal/greet"
)

func TestRoutes(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	greet.Routes(mux)

	tests := map[string]struct {
		method, path string
		form         url.Values
		wantStatus   int
		wantBody     string
	}{
		"page":       {method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: ">go-template</h1>"},
		"greet":      {method: http.MethodPost, path: "/greet", form: url.Values{"name": {"Go"}}, wantStatus: http.StatusOK, wantBody: "Hello, Go!"},
		"wrong verb": {method: http.MethodGet, path: "/greet", wantStatus: http.StatusMethodNotAllowed},
	}
	for label, tc := range tests {
		t.Run(label, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.form.Encode()))
			if tc.form != nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
