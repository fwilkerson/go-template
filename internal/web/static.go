package web

import "embed"

// static holds the vendored htmx and Alpine builds and the generated
// Tailwind stylesheet, so the binary serves them without a CDN.
//
//go:embed static
var static embed.FS
