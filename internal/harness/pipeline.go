package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dheepakkaran/agentic-sdlc-go/prompts"
)

// Issue is the input: a GitHub issue.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// Result is everything one run produced; saved to results/runs/.
type Result struct {
	Issue        Issue        `json:"issue"`
	Requirements Requirements `json:"requirements"`
	Plan         Plan         `json:"plan"`
	Attempts     [][]Gate     `json:"attempts"`
	DevModels    []string     `json:"developer_models"`
	Review       []Review     `json:"reviews"`
	Docs         DocsOut      `json:"docs"`
	Changed      []string     `json:"changed_files"`
	Passed       bool         `json:"gates_passed"`
	Approved     bool         `json:"qa_approved"`
	Stopped      string       `json:"stopped,omitempty"`
	Escalations  []string     `json:"escalations,omitempty"`
	Rollbacks    int          `json:"rollbacks,omitempty"`
	CostUSD      float64      `json:"cost_usd"`
	Calls        []Call       `json:"calls"`
}

// Pipeline runs the role agents on one issue inside Dir (a git worktree).
type Pipeline struct {
	LLM         *LLM
	Dir         string
	MapDirs     []string
	MaxAttempts int // developer attempts before giving up (last one escalated)
	Log         func(format string, a ...any)
}

func js(v any) string { b, _ := json.MarshalIndent(v, "", " "); return string(b) }

func (p *Pipeline) Run(ctx context.Context, is Issue) (*Result, error) {
	r := &Result{Issue: is}
	defer func() { r.CostUSD = p.LLM.Spent(); r.Calls = p.LLM.Calls }()
	// Preflight: the base must pass the gates before any tokens are spent.
	// Otherwise the developer is asked to fix failures it is not allowed to
	// touch (FAILURES.md #10).
	if gs := LocalGates(p.Dir); !allOK(gs) {
		p.Log("preflight %s", gateLine(gs))
		r.Attempts = append(r.Attempts, gs)
		r.Stopped = "base branch fails the gates before any change: fix main first"
		return r, nil
	}
	repoMap, err := RepoMap(p.Dir, p.MapDirs)
	if err != nil {
		return r, err
	}
	shared := prompts.Shared + repoMap
	issueText := fmt.Sprintf("Issue #%d: %s\n\n%s", is.Number, is.Title, is.Body)

	// 1. Analyst (cheap model, issue text only: no repo map needed).
	p.Log("analyst   %s  → requirements", Routes[Analyst].Model)
	if err := p.LLM.Ask(ctx, Analyst, "", "", prompts.Analyst, []Msg{{Text: issueText}}, RequirementsSchema, 1, &r.Requirements); err != nil {
		return r, err
	}
	if !r.Requirements.Valid() {
		// Cascade: the cheap model's answer failed the check → one retry on
		// the next model up. Cheap by default, escalate only on evidence.
		up := Routes[Analyst].Escalate
		p.Log("analyst   output failed the check → escalate to %s", up)
		r.Escalations = append(r.Escalations, "analyst→"+up)
		if err := p.LLM.Ask(ctx, Analyst, up, "", prompts.Analyst, []Msg{{Text: issueText}}, RequirementsSchema, 2, &r.Requirements); err != nil {
			return r, err
		}
		if !r.Requirements.Valid() {
			r.Stopped = "requirements still incomplete: needs a human"
			return r, nil
		}
	}
	reqText := "Requirements:\n" + js(r.Requirements)

	// 2. Architect (strong model, decision): scope + file plan from the map.
	p.Log("architect %s  → plan + scope decision", Routes[Architect].Model)
	if err := p.LLM.Ask(ctx, Architect, "", shared, prompts.Architect, []Msg{{Text: issueText + "\n\n" + reqText}}, planSchema, 1, &r.Plan); err != nil {
		return r, err
	}
	if !r.Plan.Approve {
		r.Stopped = "architect rejected: " + r.Plan.Reason
		return r, nil
	}
	allowed := r.Plan.FilesToWrite

	// 3. Developer ⇄ gates loop. Only planned files are sent in full.
	files := ReadFiles(p.Dir, uniq(append(append([]string{}, r.Plan.FilesToRead...), r.Plan.FilesToWrite...)))
	turns := []Msg{{Text: reqText + "\n\nPlan:\n" + js(r.Plan) + "\n\nFiles:\n" + files}}
	lastGatesFailed := false
	var green map[string]string // last version that passed every gate
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		model := Routes[Developer].Model
		if attempt == p.MaxAttempts && lastGatesFailed {
			// Escalate only on evidence: the mid model failed the gates.
			// A QA revision stays on the same model (and keeps its cache).
			model = Routes[Developer].Escalate
			r.Escalations = append(r.Escalations, "developer→"+model)
		}
		r.DevModels = append(r.DevModels, model)
		p.Log("developer %s  → attempt %d", model, attempt)
		var ch Change
		if err := p.LLM.Ask(ctx, Developer, model, shared, prompts.Developer, turns, changeSchema, attempt, &ch); err != nil {
			return r, err
		}
		changed, guardErr := applyChange(p.Dir, ch, allowed)
		r.Changed = uniq(append(r.Changed, changed...))
		gs := LocalGates(p.Dir)
		if len(guardErr) > 0 {
			gs = append(gs, Gate{"guardrails", false, strings.Join(guardErr, "\n")})
		}
		r.Attempts = append(r.Attempts, gs)
		p.Log("gates     %s", gateLine(gs))
		lastGatesFailed = !allOK(gs)
		if !allOK(gs) && green != nil {
			// Roll back: a revision broke a version that was green. The next
			// attempt starts from the working files, not from the wreck
			// (FAILURES.md #12).
			restore(p.Dir, green)
			p.Log("rollback  restored the last green version")
			r.Rollbacks++
		}
		if allOK(gs) {
			green = snapshot(p.Dir, r.Changed)
			r.Passed = true
			// 4. QA (strong model, decision) sees only the diff.
			_, _ = run(p.Dir, "git", "add", "-N", ".")
			diff, _ := run(p.Dir, "git", "diff", "--no-color")
			var rv Review
			p.Log("qa        %s  → merge decision", Routes[QA].Model)
			qaMsg := issueText + "\n\nApproved plan (scope):\n" + js(r.Plan.Steps) + "\n\n" + reqText + "\n\nGate results: all passed (gofmt, vet, test -race, golangci-lint).\n\nDiff:\n" + diff
			if err := p.LLM.Ask(ctx, QA, "", shared, prompts.QA, []Msg{{Text: qaMsg}}, reviewSchema, len(r.Review)+1, &rv); err != nil {
				return r, err
			}
			r.Review = append(r.Review, rv)
			r.Approved = rv.Verdict == "approve"
			code := codeFindings(rv.Findings, allowed)
			if r.Approved || len(r.Review) >= 2 || attempt == p.MaxAttempts || len(code) == 0 {
				if !r.Approved && len(code) == 0 {
					r.Stopped = "QA findings are outside the planned code: needs a human"
				}
				break
			}
			// One QA revision round: only findings on planned code files go back.
			r.Passed = false
			turns = append(turns, Msg{Assistant: true, Text: js(ch)}, Msg{Text: "QA requested changes (edits apply to the files as they are now):\n" + js(code)})
			continue
		}
		fb := "Your change failed the gates. Fix it.\n" + failures(gs)
		if green != nil {
			fb = "Your last edits broke a version that passed every gate, so they were reverted: the files are back to the last passing version. Make only the requested change, with smaller edits.\n" + failures(gs) + "\nCurrent files:\n" + ReadFiles(p.Dir, r.Changed)
		}
		turns = append(turns, Msg{Assistant: true, Text: js(ch)}, Msg{Text: fb})
	}
	if !r.Passed && green != nil {
		// The green version stays; QA's open findings go to a human.
		restore(p.Dir, green)
		r.Passed = true
		r.Stopped = "QA revision did not land cleanly: last green version kept, findings need a human"
		return r, nil
	}
	if !r.Passed {
		r.Stopped = "gates still failing after retries"
		return r, nil
	}
	// 5. Docs (cheap model): PR text + changelog from the diffstat.
	return r, p.Document(ctx, r, "diff")
}

// Document runs the docs role (cheap model, cascade on a failed check). It
// is separate so `sdlc redoc` can redo only this step on an existing branch.
// base is what to diff against: "diff" for the working tree, or a commit.
func (p *Pipeline) Document(ctx context.Context, r *Result, base string) error {
	args := []string{"diff", "--stat"}
	if base != "diff" {
		args = append(args, base)
	}
	stat, _ := run(p.Dir, "git", args...)
	repoMap, err := RepoMap(p.Dir, p.MapDirs)
	if err != nil {
		return err
	}
	is := r.Issue
	issueText := fmt.Sprintf("Issue #%d: %s\n\n%s", is.Number, is.Title, is.Body)
	// The type lines keep the docs agent from inventing field names
	// (FAILURES.md #5); ~60 tokens.
	msg := []Msg{{Text: issueText + "\n\nRequirements:\n" + js(r.Requirements) + "\n\nTypes (use these exact field names):\n" + typeLines(repoMap) + "\nDiffstat:\n" + stat}}
	p.Log("docs      %s  → PR body + changelog", Routes[Docs].Model)
	if err := p.LLM.Ask(ctx, Docs, "", "", prompts.Docs, msg, docsSchema, 1, &r.Docs); err != nil {
		return err
	}
	if !r.Docs.Valid(is.Number) {
		up := Routes[Docs].Escalate
		p.Log("docs      output failed the check → escalate to %s", up)
		r.Escalations = append(r.Escalations, "docs→"+up)
		if err := p.LLM.Ask(ctx, Docs, up, "", prompts.Docs, msg, docsSchema, 2, &r.Docs); err != nil {
			return err
		}
		if !r.Docs.Valid(is.Number) {
			r.Stopped = "PR text still incomplete: needs a human"
			r.Approved = false
		}
	}
	return nil
}

// RunNaive is the baseline: one strong-model call with the whole repo.
func RunNaive(ctx context.Context, l *LLM, dir string, srcDirs []string, is Issue) (*Result, error) {
	r := &Result{Issue: is}
	defer func() { r.CostUSD = l.Spent(); r.Calls = l.Calls }()
	var all []string
	for _, d := range srcDirs {
		ents, _ := os.ReadDir(filepath.Join(dir, d))
		for _, e := range ents {
			if strings.HasSuffix(e.Name(), ".go") {
				all = append(all, filepath.Join(d, e.Name()))
			}
		}
	}
	msg := fmt.Sprintf("Repository:\n%s\n\nIssue #%d: %s\n\n%s", ReadFiles(dir, all), is.Number, is.Title, is.Body)
	var ch FullChange
	if err := l.Ask(ctx, Naive, "", "", prompts.Naive, []Msg{{Text: msg}}, fullChangeSchema, 1, &ch); err != nil {
		return r, err
	}
	for _, f := range ch.Files {
		if err := writeInRoot(dir, f); err != nil {
			return r, err
		}
		r.Changed = append(r.Changed, f.Path)
	}
	gs := LocalGates(dir)
	r.Attempts = [][]Gate{gs}
	r.Passed = allOK(gs)
	return r, nil
}

func typeLines(repoMap string) string {
	var b strings.Builder
	for _, l := range strings.Split(repoMap, "\n") {
		if strings.HasPrefix(l, "- type ") && strings.Contains(l, "json:") {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

func snapshot(dir string, paths []string) map[string]string {
	m := map[string]string{}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return m
	}
	defer func() { _ = root.Close() }()
	for _, p := range paths {
		if b, err := root.ReadFile(p); err == nil {
			m[p] = string(b)
		}
	}
	return m
}

func restore(dir string, snap map[string]string) {
	for p, c := range snap {
		_ = writeInRoot(dir, File{Path: p, Content: c})
	}
}

// codeFindings keeps the QA findings the developer can act on: those on a
// planned file. Others (requirements, process) go to a human.
func codeFindings(fs []Finding, allowed []string) []Finding {
	var out []Finding
	for _, f := range fs {
		for _, a := range allowed {
			if f.File == a {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// applyChange applies edits and new files after the guardrails. Problems
// (guardrail, an old snippet that is missing or not unique) are returned as
// feedback for the developer instead of aborting the run.
func applyChange(dir string, ch Change, allowed []string) (changed, problems []string) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, []string{err.Error()}
	}
	defer func() { _ = root.Close() }()
	for _, e := range ch.Edits {
		data, err := root.ReadFile(e.Path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("edit %s: file not found (use new_files for new files)", e.Path))
			continue
		}
		src := string(data)
		if n := strings.Count(src, e.Old); n != 1 || e.Old == "" {
			problems = append(problems, fmt.Sprintf("edit %s: old snippet found %d times, it must match exactly once", e.Path, n))
			continue
		}
		next := strings.Replace(src, e.Old, e.New, 1)
		if err := CheckWrite(e.Path, next, allowed); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if err := root.WriteFile(e.Path, []byte(next), 0o600); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		changed = append(changed, e.Path)
	}
	for _, f := range ch.NewFiles {
		if err := CheckWrite(f.Path, f.Content, allowed); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if err := writeInRoot(dir, f); err != nil {
			problems = append(problems, err.Error())
			continue
		}
		changed = append(changed, f.Path)
	}
	return changed, problems
}

// writeInRoot writes a model-produced file through os.Root, so the write
// cannot escape dir even if a guardrail were bypassed.
func writeInRoot(dir string, f File) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.MkdirAll(filepath.Dir(f.Path), 0o750); err != nil {
		return err
	}
	return root.WriteFile(f.Path, []byte(f.Content), 0o600)
}

func gateLine(gs []Gate) string {
	var parts []string
	for _, g := range gs {
		m := "✗"
		if g.OK {
			m = "✓"
		}
		parts = append(parts, m+" "+g.Name)
	}
	return strings.Join(parts, "  ")
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
