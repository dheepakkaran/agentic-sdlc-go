// Command sdlc runs a GitHub issue through the AI-SDLC role agents.
//
//	sdlc run   -issue 3 [-pr]   analyst → architect → developer ⇄ gates → QA → docs → PR
//	sdlc publish -issue 3       open the PR for an approved run without re-running it
//	sdlc redoc -issue 3         redo only the docs step on the agent branch and update the PR
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
	route := fs.String("route", "routed", "routed (default) or opus: every role on the strong model, for the routing ablation")
	must(fs.Parse(os.Args[2:]))

	switch os.Args[1] {
	case "map":
		m, err := harness.RepoMap(".", mapDirs)
		must(err)
		fmt.Print(m)
		return
	case "run", "naive", "publish", "redoc":
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(2)
	}
	version := harness.Version
	if *route == "opus" {
		for role, rt := range harness.Routes {
			rt.Model, rt.Escalate = harness.Opus, ""
			harness.Routes[role] = rt
		}
		version += "-all-opus"
	}
	is := fetchIssue(*num)
	ctx := context.Background()
	runID := fmt.Sprintf("%s-%d-%s", os.Args[1], is.Number, time.Now().UTC().Format("20060102T150405"))
	if *route == "opus" {
		runID = "opus" + runID
	}
	llm := &harness.LLM{Client: anthropic.NewClient(), Run: runID, Version: version, Issue: is.Number, BudgetUSD: *budget, Ledger: "results/ledger.jsonl"}
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
	if os.Args[1] == "publish" {
		var r harness.Result
		b, err := os.ReadFile(filepath.Clean(fmt.Sprintf("results/runs/issue-%d.json", is.Number)))
		must(err)
		must(json.Unmarshal(b, &r))
		publish(&r, is, branch, filepath.Join(".work", fmt.Sprintf("issue-%d", is.Number)), r.Calls[0].Run)
		return
	}
	if os.Args[1] == "redoc" {
		redoc(ctx, llm, is, branch, logf)
		return
	}
	wt := fmt.Sprintf(".work/issue-%d", is.Number)
	if *route == "opus" {
		branch, wt = fmt.Sprintf("ai/opus-issue-%d", is.Number), fmt.Sprintf(".work/opus-issue-%d", is.Number)
	}
	dir := worktree(wt, branch)
	fmt.Printf("issue #%d · %s\n", is.Number, is.Title)
	p := &harness.Pipeline{LLM: llm, Dir: dir, MapDirs: mapDirs, MaxAttempts: *attempts, Log: logf}
	r, err := p.Run(ctx, is)
	name := fmt.Sprintf("results/runs/issue-%d.json", is.Number)
	if *route == "opus" {
		name = fmt.Sprintf("results/runs/issue-%d.all-opus.json", is.Number)
		*pr = false
	}
	save(name, r)
	must(err)
	fmt.Printf("  result: gates=%v qa=%v cost=$%.4f %s\n", r.Passed, r.Approved, r.CostUSD, r.Stopped)
	if *pr {
		publish(r, is, branch, dir, runID)
	}
}

// redoc re-runs only the docs role on the existing agent branch, adds the
// changelog entry as a new commit and updates the open PR.
func redoc(ctx context.Context, llm *harness.LLM, is harness.Issue, branch string, logf func(string, ...any)) {
	path := fmt.Sprintf("results/runs/issue-%d.json", is.Number)
	var r harness.Result
	b, err := os.ReadFile(filepath.Clean(path))
	must(err)
	must(json.Unmarshal(b, &r))
	dir, _ := filepath.Abs(filepath.Join(".work", fmt.Sprintf("issue-%d", is.Number)))
	base := strings.TrimSpace(sh(dir, "git", "merge-base", "HEAD", "origin/main"))
	p := &harness.Pipeline{LLM: llm, Dir: dir, MapDirs: mapDirs, Log: logf}
	r.Stopped = ""
	must(p.Document(ctx, &r, base))
	r.Calls = append(r.Calls, llm.Calls...)
	r.CostUSD += llm.Spent()
	save(path, &r)
	if r.Stopped != "" {
		fmt.Println("  ", r.Stopped)
		return
	}
	appendFile(filepath.Join(dir, "CHANGELOG.md"), "- "+strings.TrimPrefix(r.Docs.Changelog, "- ")+" (#"+fmt.Sprint(is.Number)+")\n")
	sh(dir, "git", "add", "CHANGELOG.md")
	sh(dir, "git", "commit", "-q", "-m", "docs: changelog for #"+fmt.Sprint(is.Number)+"\n\nAI-Assisted: sdlc-harness (docs re-run)")
	sh(dir, "git", "push", "-q", "origin", branch)
	sh(dir, "gh", "pr", "edit", branch, "--title", r.Docs.PRTitle, "--body", r.Docs.PRBody+fmt.Sprintf("\n\n---\nDocs re-run after the release gate blocked an empty changelog · total cost $%.4f · ledger in `results/ledger.jsonl`", r.CostUSD))
	fmt.Printf("  PR updated · docs cost $%.4f\n", llm.Spent())
}

// publish commits an approved run on its branch, pushes it and opens the PR.
func publish(r *harness.Result, is harness.Issue, branch, dir, runID string) {
	if !r.Passed || !r.Approved {
		fmt.Println("  not published: gates or QA did not pass")
		return
	}
	if r.Docs.Changelog != "" {
		appendFile(filepath.Join(dir, "CHANGELOG.md"), "- "+strings.TrimPrefix(r.Docs.Changelog, "- ")+" (#"+fmt.Sprint(is.Number)+")\n")
	}
	sh(dir, "git", "add", "-A")
	msg := fmt.Sprintf("%s\n\nCloses #%d\n\nAI-Assisted: sdlc-harness (%s)", r.Docs.PRTitle, is.Number, strings.Join(r.DevModels, ", "))
	sh(dir, "git", "commit", "-q", "-m", msg)
	sh(dir, "git", "push", "-q", "--force-with-lease", "-u", "origin", branch) // agent branches only: a re-run replaces the branch
	body := r.Docs.PRBody + fmt.Sprintf("\n\n---\nRun `%s` · models %s · %d developer attempt(s) · cost $%.4f · ledger in `results/ledger.jsonl`", runID, strings.Join(r.DevModels, " → "), len(r.Attempts), r.CostUSD)
	if url, err := exec.Command("gh", "pr", "view", branch, "--json", "url", "-q", ".url").Output(); err == nil && len(url) > 0 { // #nosec G204 -- branch built here
		_ = exec.Command("gh", "pr", "edit", branch, "--body", body).Run() // #nosec G204 -- branch built here
		fmt.Println("  PR updated:", strings.TrimSpace(string(url)))
		return
	}
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
