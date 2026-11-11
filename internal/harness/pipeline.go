package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dheepakkaran/go-ai-sdlc-pipeline/prompts"
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
	repoMap, err := RepoMap(p.Dir, p.MapDirs)
	if err != nil {
		return r, err
	}
	shared := prompts.Shared + repoMap
	issueText := fmt.Sprintf("Issue #%d: %s\n\n%s", is.Number, is.Title, is.Body)

	// 1. Analyst (cheap model, issue text only: no repo map needed).
	p.Log("analyst   %s  → requirements", Routes[Analyst].Model)
	if err := p.LLM.Ask(ctx, Analyst, "", "", prompts.Analyst, []Msg{{Text: issueText}}, requirementsSchema, 1, &r.Requirements); err != nil {
		return r, err
	}
	if !r.Requirements.Valid() {
		// Cascade: the cheap model's answer failed the check → one retry on
		// the next model up. Cheap by default, escalate only on evidence.
		up := Routes[Analyst].Escalate
		p.Log("analyst   output failed the check → escalate to %s", up)
		r.Escalations = append(r.Escalations, "analyst→"+up)
		if err := p.LLM.Ask(ctx, Analyst, up, "", prompts.Analyst, []Msg{{Text: issueText}}, requirementsSchema, 2, &r.Requirements); err != nil {
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
		if allOK(gs) {
			r.Passed = true
			// 4. QA (strong model, decision) sees only the diff.
			diff, _ := run(p.Dir, "git", "diff", "--no-color")
			var rv Review
			p.Log("qa        %s  → merge decision", Routes[QA].Model)
			if err := p.LLM.Ask(ctx, QA, "", shared, prompts.QA, []Msg{{Text: reqText + "\n\nGate results: all passed (gofmt, vet, test -race).\n\nDiff:\n" + diff}}, reviewSchema, len(r.Review)+1, &rv); err != nil {
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
		turns = append(turns, Msg{Assistant: true, Text: js(ch)}, Msg{Text: "Your change failed the gates. Fix it.\n" + failures(gs)})
	}
	if !r.Passed {
		r.Stopped = "gates still failing after retries"
		return r, nil
	}
	// 5. Docs (cheap model): PR text + changelog from the diffstat.
	stat, _ := run(p.Dir, "git", "diff", "--stat")
	p.Log("docs      %s  → PR body + changelog", Routes[Docs].Model)
	if err := p.LLM.Ask(ctx, Docs, "", "", prompts.Docs, []Msg{{Text: issueText + "\n\n" + reqText + "\n\nDiffstat:\n" + stat}}, docsSchema, 1, &r.Docs); err != nil {
		return r, err
	}
	return r, nil
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
