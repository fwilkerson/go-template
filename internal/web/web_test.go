package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fwilkerson/go-template/internal/web"
)

func TestRoutesServeStatic(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	web.Routes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "htmx") {
		t.Errorf("static asset: status %d, body %.40q", rec.Code, rec.Body.String())
	}
}

func TestLayout(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	web.Render(rec, req, web.Layout("Title"))
	for _, want := range []string{"<title>Title</title>", "/static/app.css", "/static/htmx.min.js"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("layout lacks %q", want)
		}
	}
}
