package main

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// githubSource takes a file from a repository at its latest release tag.
// Prereleases are excluded, as the releases/latest endpoint does.
type githubSource struct {
	api, repo, path string
}

func (g *githubSource) latest(ctx context.Context, client *http.Client) (release, error) {
	body, err := get(ctx, client, g.api+"/repos/"+g.repo+"/releases/latest", githubHeader())
	if err != nil {
		return release{}, err
	}
	var rel struct {
		TagName     string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return release{}, fmt.Errorf("decode release: %w", err)
	}
	if rel.TagName == "" || rel.PublishedAt.IsZero() {
		return release{}, errors.New("release has no tag or publish time")
	}
	return release{Version: strings.TrimPrefix(rel.TagName, "v"), Ref: rel.TagName, Published: rel.PublishedAt}, nil
}

// fetch reads the file through the contents API at the tag and checks the
// bytes against the blob hash the API reports for them.
func (g *githubSource) fetch(ctx context.Context, client *http.Client, tag string) ([]byte, error) {
	body, err := get(ctx, client, g.api+"/repos/"+g.repo+"/contents/"+g.path+"?ref="+tag, githubHeader())
	if err != nil {
		return nil, err
	}
	var file struct {
		SHA      string `json:"sha"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("decode contents: %w", err)
	}
	if file.Encoding != "base64" {
		return nil, fmt.Errorf("contents API returned encoding %q; files over 1MB are not supported", file.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(file.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}
	if got := blobHash(data); got != file.SHA {
		return nil, fmt.Errorf("blob hash %s does not match the repository's %s", got, file.SHA)
	}
	return data, nil
}

// githubHeader carries a token when one is in the environment; the anonymous
// limit is 60 requests an hour.
func githubHeader() http.Header {
	h := http.Header{"Accept": {"application/vnd.github+json"}}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	return h
}

// blobHash is git's object id for a blob: sha1 over "blob <size>\0<data>",
// the same value `git hash-object` prints.
func blobHash(data []byte) string {
	h := sha1.New()
	h.Write(fmt.Appendf(nil, "blob %d\x00", len(data)))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
