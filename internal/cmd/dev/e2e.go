package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// e2e runs .http files against the server: by default every file beside the
// server's main package, against a server it builds and starts itself, or
// with -addr against one already listening. A request without an assertion fails, so the file
// records what was verified, not only what was sent.
func e2e(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("e2e", flag.ContinueOnError)
	fs.SetOutput(stderr)
	server := fs.String("server", "", "package to build and run for the duration of the run; the only one under cmd/ by default")
	addr := fs.String("addr", "", "host:port of a running server to target instead of starting one")
	env := fs.String("env", "dev", "environment to take variables from in http-client.env.json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		var err error
		if *server, err = onlyBinary("cmd"); err != nil {
			return fmt.Errorf("e2e: %w", err)
		}
	}
	files := fs.Args()
	if len(files) == 0 {
		var err error
		if files, err = filepath.Glob(filepath.Join(*server, "*.http")); err != nil || len(files) == 0 {
			return fmt.Errorf("e2e: no .http files in %s", *server)
		}
	}

	host := "http://" + *addr
	var log *bytes.Buffer
	var stop func()
	if *addr == "" {
		var err error
		host, log, stop, err = startServer(ctx, *server)
		if err != nil {
			return err
		}
		defer stop()
	}

	client := &http.Client{Timeout: 10 * time.Second}
	failed := 0
	for _, file := range files {
		results, err := runFile(ctx, client, file, *env, host)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		n, err := reportFile(stdout, file, results)
		if err != nil {
			return err
		}
		failed += n
	}
	if failed > 0 {
		if stop != nil {
			stop() // the buffer is safe to read once the server has exited
			if log.Len() > 0 {
				if _, err := fmt.Fprintf(stderr, "server output:\n%s", log); err != nil {
					return err
				}
			}
		}
		return fmt.Errorf("e2e: %d assertions failed", failed)
	}
	return nil
}

// onlyBinary returns the single package directory under dir, so the runner
// finds the project's binary without being told its name.
func onlyBinary(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, "./"+filepath.Join(dir, e.Name()))
		}
	}
	switch len(dirs) {
	case 0:
		return "", fmt.Errorf("no package under %s/", dir)
	case 1:
		return dirs[0], nil
	}
	return "", fmt.Errorf("%d packages under %s/ (%s); pass -server", len(dirs), dir, strings.Join(dirs, ", "))
}

// reportFile prints one file's failures and its summary line, and returns the
// number of failed assertions.
func reportFile(w io.Writer, file string, results []assertion) (int, error) {
	n := 0
	for _, a := range results {
		if a.OK {
			continue
		}
		n++
		if _, err := fmt.Fprintf(w, "%s:%d: %s\n    %s\n", file, a.Line, a.label(), a.Expr); err != nil {
			return 0, err
		}
	}
	var err error
	if n == 0 {
		_, err = fmt.Fprintf(w, "ok   %s  %d assertions\n", file, len(results))
	} else {
		_, err = fmt.Fprintf(w, "FAIL %s  %d of %d assertions failed\n", file, n, len(results))
	}
	return n, err
}

func (a assertion) label() string {
	switch {
	case a.Test != "" && a.Message != "":
		return a.Test + ": " + a.Message
	case a.Test != "":
		return a.Test
	case a.Message != "":
		return a.Message
	}
	return "assertion failed"
}

// runFile sends every request in file and evaluates its handler. The host
// variable is set by the runner; the rest come from http-client.env.json.
func runFile(ctx context.Context, client *http.Client, file, env, host string) ([]assertion, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	reqs, err := parseHTTPFile(string(src))
	if err != nil {
		return nil, err
	}
	vars, err := loadEnv(filepath.Dir(file), env)
	if err != nil {
		return nil, err
	}
	vars["host"] = host

	var all []assertion
	for _, r := range reqs {
		if r.Script == "" {
			return nil, fmt.Errorf("line %d: %s has no assertions; add a `> {%% client.test(...) %%}` handler", r.Line, r.describe())
		}
		resp, err := send(ctx, client, r, vars)
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", r.Line, r.describe(), err)
		}
		results, err := runScript(r.Script, r.ScriptLine, resp)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.describe(), err)
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("line %d: %s: the handler makes no assertions", r.Line, r.describe())
		}
		all = append(all, results...)
	}
	return all, nil
}

func (r request) describe() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Method + " " + r.URL
}

func send(ctx context.Context, client *http.Client, r request, vars map[string]string) (response, error) {
	url, err := expand(r.URL, vars)
	if err != nil {
		return response{}, err
	}
	body, err := expand(r.Body, vars)
	if err != nil {
		return response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, url, strings.NewReader(body))
	if err != nil {
		return response{}, err
	}
	for _, h := range r.Headers {
		v, err := expand(h[1], vars)
		if err != nil {
			return response{}, err
		}
		req.Header.Add(h[0], v)
	}
	res, err := client.Do(req)
	if err != nil {
		return response{}, err
	}
	data, err := io.ReadAll(res.Body)
	if err := errors.Join(err, res.Body.Close()); err != nil {
		return response{}, err
	}
	mimeType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	return response{Status: res.StatusCode, Body: string(data), MIME: mimeType, Headers: res.Header}, nil
}

// loadEnv reads the named environment from http-client.env.json and
// http-client.private.env.json in dir, the private file winning. A missing
// file is an empty environment.
func loadEnv(dir, env string) (map[string]string, error) {
	vars := map[string]string{}
	for _, name := range []string{"http-client.env.json", "http-client.private.env.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var envs map[string]map[string]any
		if err := json.Unmarshal(data, &envs); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for k, v := range envs[env] {
			vars[k] = fmt.Sprint(v)
		}
	}
	return vars, nil
}

// startServer builds pkg, starts it on a free port and waits until it
// accepts connections. Its output is collected for reporting on failure.
func startServer(ctx context.Context, pkg string) (host string, log *bytes.Buffer, stop func(), err error) {
	dir, err := os.MkdirTemp("", "e2e")
	if err != nil {
		return "", nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) } // best effort; the run's result matters more
	bin := filepath.Join(dir, "server")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, pkg)
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		cleanup()
		return "", nil, nil, fmt.Errorf("go build %s: %w", pkg, err)
	}

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		cleanup()
		return "", nil, nil, err
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		cleanup()
		return "", nil, nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	log = &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, bin, "-addr", addr)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		cancel()
		cleanup()
		return "", nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop = sync.OnceFunc(func() {
		cancel()
		<-done
		cleanup()
	})

	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case err := <-done:
			cleanup()
			return "", nil, nil, fmt.Errorf("server exited before listening: %w\n%s", err, log)
		default:
		}
		if c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond); err == nil {
			_ = c.Close()
			return "http://" + addr, log, stop, nil
		}
		if time.Now().After(deadline) {
			stop()
			return "", nil, nil, fmt.Errorf("server did not listen on %s within 30s\n%s", addr, log)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
