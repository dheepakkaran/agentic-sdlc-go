// Command sdlc runs a GitHub issue through the AI-SDLC role agents.
//
//	sdlc run   -issue 3 [-pr]   analyst → architect → developer ⇄ gates → QA → docs → PR
//	sdlc naive -issue 3         baseline: one strong-model call with the whole repo
//	sdlc map                    print the repo map the agents see
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/dheepakkaran/go-ai-sdlc-pipeline/internal/harness"
)

var mapDirs = []string{"internal/inventory", "cmd/inventory"}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sdlc run|naive|map [flags]")
		os.Exit(2)
	}
	loadEnv(".env")
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	num := fs.Int("issue", 0, "GitHub issue number")
	pr := fs.Bool("pr", false, "commit, push and open a pull request when QA approves")
	budget := fs.Float64("budget", 0.60, "USD cap for this run")
	attempts := fs.Int("attempts", 3, "developer attempts (last one escalated to the strong model)")
	must(fs.Parse(os.Args[2:]))

	switch os.Args[1] {
	case "map":
		m, err := harness.RepoMap(".", mapDirs)
		must(err)
		fmt.Print(m)
		return
	case "run", "naive":
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(2)
	}
	is := fetchIssue(*num)
	ctx := context.Background()
	runID := fmt.Sprintf("%s-%d-%s", os.Args[1], is.Number, time.Now().UTC().Format("20060102T150405"))
	llm := &harness.LLM{Client: anthropic.NewClient(), Run: runID, Version: harness.Version, Issue: is.Number, BudgetUSD: *budget, Ledger: "results/ledger.jsonl"}
	logf := func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) }

	if os.Args[1] == "naive" {
		dir := worktree(fmt.Sprintf(".work/naive-%d", is.Number), "")
		defer func() { _ = exec.Command("git", "worktree", "remove", "--force", dir).Run() }() // #nosec G204 -- dir is ".work/naive-<int>", built here
		fmt.Printf("naive baseline · issue #%d %s\n", is.Number, is.Title)
		r, err := harness.RunNaive(ctx, llm, dir, mapDirs, is)
		save(fmt.Sprintf("results/runs/naive-%d.json", is.Number), r)
		must(err)
		fmt.Printf("  passed=%v  cost=$%.4f\n", r.Passed, r.CostUSD)
		return
	}

	branch := fmt.Sprintf("ai/issue-%d", is.Number)
	dir := worktree(fmt.Sprintf(".work/issue-%d", is.Number), branch)
	fmt.Printf("issue #%d · %s\n", is.Number, is.Title)
	p := &harness.Pipeline{LLM: llm, Dir: dir, MapDirs: mapDirs, MaxAttempts: *attempts, Log: logf}
	r, err := p.Run(ctx, is)
	save(fmt.Sprintf("results/runs/issue-%d.json", is.Number), r)
	must(err)
	fmt.Printf("  result: gates=%v qa=%v cost=$%.4f %s\n", r.Passed, r.Approved, r.CostUSD, r.Stopped)
	if !*pr || !r.Passed || !r.Approved {
		return
	}
	if r.Docs.Changelog != "" {
		appendFile(filepath.Join(dir, "CHANGELOG.md"), "- "+strings.TrimPrefix(r.Docs.Changelog, "- ")+" (#"+fmt.Sprint(is.Number)+")\n")
	}
	sh(dir, "git", "add", "-A")
	msg := fmt.Sprintf("%s\n\nCloses #%d\n\nAI-Assisted: sdlc-harness (%s)", r.Docs.PRTitle, is.Number, strings.Join(r.DevModels, ", "))
	sh(dir, "git", "commit", "-q", "-m", msg)
	sh(dir, "git", "push", "-q", "-u", "origin", branch)
	body := r.Docs.PRBody + fmt.Sprintf("\n\n---\nRun `%s` · models %s · %d developer attempt(s) · cost $%.4f · ledger in `results/ledger.jsonl`", runID, strings.Join(r.DevModels, " → "), len(r.Attempts), r.CostUSD)
	out := sh(dir, "gh", "pr", "create", "--title", r.Docs.PRTitle, "--body", body, "--label", "ai-authored", "--base", "main", "--head", branch)
	fmt.Println("  PR:", strings.TrimSpace(out))
}

func fetchIssue(n int) harness.Issue {
	out := sh(".", "gh", "issue", "view", fmt.Sprint(n), "--json", "number,title,body")
	var is harness.Issue
	must(json.Unmarshal([]byte(out), &is))
	return is
}

// worktree makes a clean checkout of main so a run never touches the main tree.
func worktree(dir, branch string) string {
	_ = exec.Command("git", "worktree", "remove", "--force", dir).Run() // #nosec G204 -- dir built here; error = did not exist
	if branch == "" {
		sh(".", "git", "worktree", "add", "-q", "--detach", dir, "main")
	} else {
		_ = exec.Command("git", "branch", "-D", branch).Run() // #nosec G204 -- branch is "ai/issue-<int>", built here
		sh(".", "git", "worktree", "add", "-q", "-b", branch, dir, "main")
	}
	abs, _ := filepath.Abs(dir)
	return abs
}

func sh(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...) // #nosec G204 G702 -- fixed tools (git, gh), args built in this file, no shell
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
		os.Exit(1)
	}
	return string(out)
}

func save(path string, v any) {
	must(os.MkdirAll(filepath.Dir(path), 0o750))
	b, err := json.MarshalIndent(v, "", "  ")
	must(err)
	must(os.WriteFile(path, b, 0o600))
}

func appendFile(path, s string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 -- CHANGELOG.md inside the run's own worktree
	must(err)
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(s)
	must(err)
}

func loadEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- fixed path ".env"
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if ok && !strings.HasPrefix(k, "#") && os.Getenv(k) == "" {
			must(os.Setenv(k, strings.Trim(v, `"'`)))
		}
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
