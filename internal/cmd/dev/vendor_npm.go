package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
)

// npmSource takes a file out of the package tarball a dist-tag points at.
type npmSource struct {
	registry, pkg, tag, file string
}

// npmRelease is the subset of a registry version document the tool uses.
type npmRelease struct {
	Version string `json:"version"`
	Dist    struct {
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
	} `json:"dist"`
}

func (n *npmSource) latest(ctx context.Context, client *http.Client) (string, string, error) {
	body, err := get(ctx, client, n.registry+"/"+n.pkg+"/"+n.tag, nil)
	if err != nil {
		return "", "", err
	}
	var rel npmRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", "", fmt.Errorf("decode registry response: %w", err)
	}
	if rel.Version == "" || rel.Dist.Tarball == "" {
		return "", "", errors.New("registry response has no version or tarball")
	}
	// The tarball URL and integrity travel as the ref so fetch does not
	// resolve the tag a second time.
	ref, err := json.Marshal(rel)
	return rel.Version, string(ref), err
}

// fetch downloads the tarball, checks it against the registry's integrity
// value and returns the one file.
func (n *npmSource) fetch(ctx context.Context, client *http.Client, ref string) ([]byte, error) {
	var rel npmRelease
	if err := json.Unmarshal([]byte(ref), &rel); err != nil {
		return nil, err
	}
	tarball, err := get(ctx, client, rel.Dist.Tarball, nil)
	if err != nil {
		return nil, err
	}
	if err := verifyIntegrity(tarball, rel.Dist.Integrity); err != nil {
		return nil, err
	}
	return extract(tarball, path.Join("package", n.file))
}

// verifyIntegrity checks data against an SRI value such as "sha512-<base64>".
func verifyIntegrity(data []byte, integrity string) error {
	algo, encoded, ok := strings.Cut(integrity, "-")
	if !ok || algo != "sha512" {
		return fmt.Errorf("unsupported integrity %q", integrity)
	}
	want, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode integrity: %w", err)
	}
	if got := sha512.Sum512(data); !bytes.Equal(got[:], want) {
		return errors.New("tarball does not match the registry's sha512")
	}
	return nil
}

// extract returns the named member of a gzipped tarball.
func extract(tarball []byte, member string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s not found in tarball", member)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == member && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}
