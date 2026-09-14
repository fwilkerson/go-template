package main

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// manifest lists the front-end assets copied into the repository and where
// each copy came from. It lives next to the files it describes.
type manifest struct {
	Assets []asset `json:"assets"`
}

// asset is one vendored file. Source is "github" or "npm"; github is preferred
// because it exposes the project only to GitHub, and is verified by git blob
// hash. npm is for projects that do not commit their built file, and is
// verified by the registry's sha512 of the package tarball.
type asset struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	// Dest is the file next to the manifest; Version the release installed.
	Dest    string `json:"dest"`
	Version string `json:"version"`

	// github: Repo is "owner/name", Path the file in the repository at the
	// release tag, Blob the git blob hash of the installed copy.
	Repo string `json:"repo,omitempty"`
	Path string `json:"path,omitempty"`
	Blob string `json:"blob,omitempty"`

	// npm: Package is the npm package, Tag the dist-tag to follow, File the
	// path inside the package tarball.
	Package string `json:"package,omitempty"`
	Tag     string `json:"tag,omitempty"`
	File    string `json:"file,omitempty"`
}

// release is the current release of an asset at its source. Ref is whatever
// the source needs to fetch that release again.
type release struct {
	Version   string
	Ref       string
	Published time.Time
}

// source resolves the current release of an asset and fetches a verified copy
// of its file.
type source interface {
	latest(ctx context.Context, client *http.Client) (release, error)
	fetch(ctx context.Context, client *http.Client, ref string) (data []byte, err error)
}

const (
	defaultManifest = "internal/web/static/vendor.json"
	// defaultCooldown is how long a release must have been public before
	// -update installs it, so a compromised publish has time to be noticed and
	// pulled. Matches npm's min-release-age practice and prov's cooldown_days.
	defaultCooldown = 7 * 24 * time.Hour
)

// vendor compares each vendored asset with its source's current release. With
// -update it fetches, verifies and installs releases older than the cooldown
// and records them in the manifest.
func vendor(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dev vendor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	update := fs.Bool("update", false, "download and install newer versions")
	force := fs.Bool("force", false, "with -update, install releases still in the cooldown")
	cooldown := fs.Duration("cooldown", defaultCooldown, "minimum age of a release before -update installs it")
	manifestPath := fs.String("manifest", defaultManifest, "manifest to read")
	github := fs.String("github", "https://api.github.com", "GitHub API base URL")
	npm := fs.String("npm", "https://registry.npmjs.org", "npm registry base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	now := time.Now()

	m, err := readManifest(*manifestPath)
	if err != nil {
		return err
	}
	client := &http.Client{}
	changed := false
	for i := range m.Assets {
		a := &m.Assets[i]
		src, err := a.source(*github, *npm)
		if err != nil {
			return err
		}
		rel, err := src.latest(ctx, client)
		if err != nil {
			return fmt.Errorf("%s: %w", a.Name, err)
		}
		age := now.Sub(rel.Published)
		switch {
		case rel.Version == a.Version:
			err = report(stdout, a, "%s current", a.Version)
		case age < *cooldown && !*force:
			err = report(stdout, a, "%s -> %s released %s ago, in cooldown for %s more", a.Version, rel.Version,
				days(age), days(*cooldown-age+24*time.Hour-time.Nanosecond))
		case !*update:
			err = report(stdout, a, "%s -> %s available, released %s ago", a.Version, rel.Version, days(age))
		default:
			err = install(ctx, client, src, a, filepath.Dir(*manifestPath), rel, stdout)
			changed = true
		}
		if err != nil {
			return err
		}
	}
	if changed {
		return writeManifest(*manifestPath, m)
	}
	return nil
}

// install fetches a verified copy of the release, writes it next to the
// manifest and updates the asset's record.
func install(ctx context.Context, client *http.Client, src source, a *asset, dir string, rel release, stdout io.Writer) error {
	data, err := src.fetch(ctx, client, rel.Ref)
	if err != nil {
		return fmt.Errorf("%s %s: %w", a.Name, rel.Version, err)
	}
	if err := writeAtomic(filepath.Join(dir, a.Dest), data); err != nil {
		return err
	}
	if err := report(stdout, a, "%s -> %s installed", a.Version, rel.Version); err != nil {
		return err
	}
	a.Version = rel.Version
	if a.Source == "github" {
		a.Blob = blobHash(data)
	}
	return nil
}

// days renders a duration in whole days, rounded down.
func days(d time.Duration) string {
	n := int(d / (24 * time.Hour))
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// report writes one status line; npm-sourced assets are marked so the
// exposure stays visible.
func report(w io.Writer, a *asset, format string, args ...any) error {
	note := ""
	if a.Source == "npm" {
		note = "  (npm)"
	}
	_, err := fmt.Fprintf(w, "%-10s "+format+"%s\n", append(append([]any{a.Name}, args...), note)...)
	return err
}

func (a *asset) source(github, npm string) (source, error) {
	switch a.Source {
	case "github":
		if a.Repo == "" || a.Path == "" {
			return nil, fmt.Errorf("%s: github source needs repo and path", a.Name)
		}
		return &githubSource{api: github, repo: a.Repo, path: a.Path}, nil
	case "npm":
		if a.Package == "" || a.Tag == "" || a.File == "" {
			return nil, fmt.Errorf("%s: npm source needs package, tag and file", a.Name)
		}
		return &npmSource{registry: npm, pkg: a.Package, tag: a.Tag, file: a.File}, nil
	}
	return nil, fmt.Errorf("%s: unknown source %q", a.Name, a.Source)
}

func readManifest(p string) (*manifest, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &m, nil
}

func writeManifest(p string, m *manifest) error {
	data, err := json.Marshal(m, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	return writeAtomic(p, append(data, '\n'))
}

func get(ctx context.Context, client *http.Client, url string, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header = header
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err := errors.Join(err, resp.Body.Close()); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return body, nil
}

// writeAtomic writes through a temp file in the same directory and renames it
// into place, so the destination is never half-written.
func writeAtomic(dest string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err := errors.Join(err, tmp.Close()); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	return os.Rename(tmp.Name(), dest)
}
