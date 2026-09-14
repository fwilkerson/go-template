// Package guidance holds the test that keeps the instructions written for
// Claude small and plainly worded. The files read on every turn stay under a
// word budget, each skill says what it is for, and nothing shouts. It has no
// code of its own; the rules are the test.
package guidance_test

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Budgets in words. Everything under alwaysLoaded is in context for every
// task, so growth there is paid on every turn; a skill is paid only when its
// description matches. When a budget is hit, move something into a tool, a
// test or a skill rather than raising it.
const (
	alwaysLoadedBudget = 800
	skillBudget        = 800
)

// shouting is emphasis that current models over-trigger on. State when a
// rule applies instead, in a normal register.
var shouting = []string{"MUST", "NEVER", "ALWAYS", "CRITICAL", "IMPORTANT", "DO NOT"}

func TestAlwaysLoadedBudget(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	files := withImports(t, filepath.Join(root, "CLAUDE.md"))
	total := 0
	var parts []string
	for _, f := range files {
		n := words(readFile(t, f))
		total += n
		parts = append(parts, fmt.Sprintf("%s %d", rel(root, f), n))
	}
	if total > alwaysLoadedBudget {
		t.Errorf("guidance read on every turn is %d words, budget %d (%s): move something into a tool, a test or a skill rather than raising the budget",
			total, alwaysLoadedBudget, strings.Join(parts, ", "))
	}
}

func TestSkills(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	dirs, err := os.ReadDir(filepath.Join(root, ".claude", "skills"))
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		path := filepath.Join(root, ".claude", "skills", d.Name(), "SKILL.md")
		src := readFile(t, path)
		front := frontmatter(src)
		if front["name"] != d.Name() {
			t.Errorf("%s: frontmatter name %q must match the directory", rel(root, path), front["name"])
		}
		if front["description"] == "" {
			t.Errorf("%s: frontmatter needs a description saying when the skill applies; that is how it gets loaded", rel(root, path))
		}
		if n := words(src); n > skillBudget {
			t.Errorf("%s is %d words, budget %d: keep the skill to what the task needs and link to references for the rest", rel(root, path), n, skillBudget)
		}
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	files := withImports(t, filepath.Join(root, "CLAUDE.md"))
	skills, err := filepath.Glob(filepath.Join(root, ".claude", "skills", "*", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range slices.Concat(files, skills) {
		for i, line := range prose(readFile(t, f)) {
			for _, word := range shouting {
				if strings.Contains(line, word) {
					t.Errorf("%s:%d: %q shouts; say when the rule applies instead", rel(root, f), i+1, word)
				}
			}
		}
	}
}

var importLine = regexp.MustCompile(`^@(\S+)`)

// withImports returns path and, recursively, the files it imports with a
// leading @, in reading order.
func withImports(t *testing.T, path string) []string {
	t.Helper()
	files := []string{path}
	for line := range strings.SplitSeq(readFile(t, path), "\n") {
		m := importLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		p := m[1]
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(path), p)
		}
		files = append(files, withImports(t, p)...)
	}
	return files
}

// frontmatter reads the key: value lines between the leading --- markers.
func frontmatter(src string) map[string]string {
	front := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(src))
	if !sc.Scan() || sc.Text() != "---" {
		return front
	}
	for sc.Scan() {
		if sc.Text() == "---" {
			break
		}
		if k, v, ok := strings.Cut(sc.Text(), ":"); ok {
			front[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return front
}

// prose returns the lines of src with fenced code blocks blanked, keeping
// line numbers.
func prose(src string) []string {
	lines := strings.Split(src, "\n")
	inFence := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			lines[i] = ""
			continue
		}
		if inFence {
			lines[i] = ""
		}
	}
	return lines
}

func words(src string) int { return len(strings.Fields(src)) }

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	return strings.TrimSpace(string(out))
}
