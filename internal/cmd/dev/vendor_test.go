package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tgz builds a gzipped tarball with one regular file.
func tgz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sri(data []byte) string {
	sum := sha512.Sum512(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

func respond(t *testing.T, w http.ResponseWriter, format string, args ...any) {
	t.Helper()
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		t.Error(err)
	}
}

func TestBlobHash(t *testing.T) {
	t.Parallel()

	// `printf 'hello\n' | git hash-object --stdin`
	if got, want := blobHash([]byte("hello\n")), "ce013625030ba8dba906f756967f9e9ca394464a"; got != want {
		t.Errorf("blobHash = %s, want %s", got, want)
	}
}

func TestVerifyIntegrity(t *testing.T) {
	t.Parallel()

	data := []byte("payload")
	if err := verifyIntegrity(data, sri(data)); err != nil {
		t.Errorf("matching digest: %v", err)
	}
	if err := verifyIntegrity([]byte("other"), sri(data)); err == nil {
		t.Error("mismatched digest: expected an error")
	}
	if err := verifyIntegrity(data, "sha256-abc"); err == nil {
		t.Error("unsupported algorithm: expected an error")
	}
}

func TestExtract(t *testing.T) {
	t.Parallel()

	tarball := tgz(t, "package/dist/lib.min.js", []byte("js"))
	got, err := extract(tarball, "package/dist/lib.min.js")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if string(got) != "js" {
		t.Errorf("extract = %q, want %q", got, "js")
	}
	if _, err := extract(tarball, "package/missing"); err == nil {
		t.Error("missing member: expected an error")
	}
}

// TestVendor drives the command against fake GitHub and npm servers: a github
// asset and an npm asset each have a newer release, a third is current.
func TestVendor(t *testing.T) {
	t.Parallel()

	ghLib := []byte("gh lib")
	npmLib := []byte("npm lib")
	tarball := tgz(t, "package/dist/lib.min.js", npmLib)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		respond(t, w, `{"tag_name":"v2.0.0"}`)
	})
	mux.HandleFunc("GET /repos/o/r/contents/dist/lib.min.js", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != "v2.0.0" {
			http.Error(w, "wrong ref", http.StatusNotFound)
			return
		}
		respond(t, w, `{"sha":%q,"encoding":"base64","content":%q}`, blobHash(ghLib), base64.StdEncoding.EncodeToString(ghLib))
	})
	mux.HandleFunc("GET /repos/o/same/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		respond(t, w, `{"tag_name":"v1.0.0"}`)
	})
	mux.HandleFunc("GET /lib/latest", func(w http.ResponseWriter, r *http.Request) {
		respond(t, w, `{"version":"2.0.0","dist":{"tarball":"http://%s/lib.tgz","integrity":%q}}`, r.Host, sri(tarball))
	})
	mux.HandleFunc("GET /lib.tgz", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(tarball); err != nil {
			t.Error(err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "vendor.json")
	err := os.WriteFile(manifestPath, []byte(`{"assets":[
		{"name":"gh","source":"github","repo":"o/r","path":"dist/lib.min.js","dest":"gh.min.js","version":"1.0.0"},
		{"name":"same","source":"github","repo":"o/same","path":"x.js","dest":"x.js","version":"1.0.0"},
		{"name":"npm","source":"npm","package":"lib","tag":"latest","file":"dist/lib.min.js","dest":"npm.min.js","version":"1.0.0"}]}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	flags := []string{"-manifest", manifestPath, "-github", srv.URL, "-npm", srv.URL}

	var out bytes.Buffer
	if err := vendor(t.Context(), flags, &out, &out); err != nil {
		t.Fatalf("vendor: %v", err)
	}
	for _, want := range []string{"gh         1.0.0 -> 2.0.0 available\n", "same       1.0.0 current\n", "npm        1.0.0 -> 2.0.0 available  (npm)\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("report lacks %q:\n%s", want, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "gh.min.js")); err == nil {
		t.Error("report-only run wrote an asset")
	}

	out.Reset()
	if err := vendor(t.Context(), append([]string{"-update"}, flags...), &out, &out); err != nil {
		t.Fatalf("vendor -update: %v", err)
	}
	for dest, want := range map[string][]byte{"gh.min.js": ghLib, "npm.min.js": npmLib} {
		got, err := os.ReadFile(filepath.Join(dir, dest))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s = %q, %v; want %q", dest, got, err, want)
		}
	}
	m, err := readManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if m.Assets[0].Version != "2.0.0" || m.Assets[0].Blob != blobHash(ghLib) {
		t.Errorf("github asset = %+v; want version 2.0.0 and blob %s", m.Assets[0], blobHash(ghLib))
	}
	if m.Assets[1].Version != "1.0.0" || m.Assets[2].Version != "2.0.0" {
		t.Errorf("versions = %s, %s; want 1.0.0, 2.0.0", m.Assets[1].Version, m.Assets[2].Version)
	}
}
