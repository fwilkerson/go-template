package greet

import (
	"net/http"

	"github.com/fwilkerson/go-template/internal/web"
)

// Routes mounts the feature: its page and the htmx fragment the page posts to.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", handlePage)
	mux.HandleFunc("POST /greet", handleGreet)
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	web.Render(w, r, page())
}

// handleGreet returns the fragment htmx swaps into the page.
func handleGreet(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	web.Render(w, r, greeting(Greeting(r.PostForm.Get("name"))))
}
