// Command metrics turns the cost ledger, run results and GitHub history into
// results/metrics.json (and docs/data.js for the project page): token and
// cost per role and model, cache hit rate, harness vs naive baseline, AI
// adoption (share of merged PRs written by the harness) and lead time.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/dheepakkaran/go-ai-sdlc-pipeline/internal/harness"
)

type agg struct {
	Calls      int     `json:"calls"`
	Input      int64   `json:"input_tokens"`
	Output     int64   `json:"output_tokens"`
	CacheRead  int64   `json:"cache_read_tokens"`
	CacheWrite int64   `json:"cache_write_tokens"`
	CostUSD    float64 `json:"cost_usd"`
	Seconds    float64 `json:"seconds"`
}

func (a *agg) add(c harness.Call) {
	a.Calls++
	a.Input += c.Input
	a.Output += c.Output
	a.CacheRead += c.CacheRead
	a.CacheWrite += c.CacheWrite
	a.CostUSD += c.CostUSD
	a.Seconds += c.Seconds
}

func (a agg) promptTokens() int64 { return a.Input + a.CacheRead + a.CacheWrite }

type issueRow struct {
	Issue        int      `json:"issue"`
	Title        string   `json:"title"`
	Harness      agg      `json:"harness"`
	Naive        agg      `json:"naive"`
	HarnessPass  bool     `json:"harness_gates_passed"`
	HarnessQA    bool     `json:"harness_qa_approved"`
	NaivePass    bool     `json:"naive_gates_passed"`
	DevModels    []string `json:"developer_models"`
	DevAttempts  int      `json:"developer_attempts"`
	QARounds     int      `json:"qa_rounds"`
	FilesChanged int      `json:"files_changed"`
}

type pr struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	MergedAt  time.Time `json:"mergedAt"`
	Labels    []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Checks    []struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

func main() {
	calls := readLedger("results/ledger.jsonl")
	latest := map[string]string{} // latest run id per kind+issue, so reruns don't double count
	for _, c := range calls {
		k := fmt.Sprintf("%s-%d", kind(c.Run), c.Issue)
		if c.Run > latest[k] {
			latest[k] = c.Run
		}
	}
	byRole, byModel := map[string]*agg{}, map[string]*agg{}
	harnessTot, naiveTot := agg{}, agg{}
	rows := map[int]*issueRow{}
	for _, c := range calls {
		k := fmt.Sprintf("%s-%d", kind(c.Run), c.Issue)
		if c.Run != latest[k] {
			continue
		}
		row := rows[c.Issue]
		if row == nil {
			row = &issueRow{Issue: c.Issue}
			rows[c.Issue] = row
		}
		if kind(c.Run) == "naive" {
			row.Naive.add(c)
			naiveTot.add(c)
			continue
		}
		row.Harness.add(c)
		harnessTot.add(c)
		get(byRole, string(c.Role)).add(c)
		get(byModel, c.Model).add(c)
	}
	for n, row := range rows {
		var r harness.Result
		if readJSON(fmt.Sprintf("results/runs/issue-%d.json", n), &r) == nil {
			row.Title, row.HarnessPass, row.HarnessQA = r.Issue.Title, r.Passed, r.Approved
			row.DevModels, row.DevAttempts, row.QARounds, row.FilesChanged = r.DevModels, len(r.Attempts), len(r.Review), len(r.Changed)
		}
		var nr harness.Result
		if readJSON(fmt.Sprintf("results/runs/naive-%d.json", n), &nr) == nil {
			row.NaivePass = nr.Passed
		}
	}
	var list []issueRow
	for _, r := range rows {
		list = append(list, *r)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Issue < list[j].Issue })

	prs := githubPRs()
	ai, merged, leadH := 0, 0, []float64{}
	for _, p := range prs {
		if p.State != "MERGED" {
			continue
		}
		merged++
		for _, l := range p.Labels {
			if l.Name == "ai-authored" {
				ai++
				leadH = append(leadH, p.MergedAt.Sub(p.CreatedAt).Hours())
			}
		}
	}
	cacheRate := 0.0
	if t := harnessTot.promptTokens(); t > 0 {
		cacheRate = float64(harnessTot.CacheRead) / float64(t)
	}
	out := map[string]any{
		"generated_at":       time.Now().UTC().Format(time.RFC3339),
		"routes":             harness.Routes,
		"prices_per_mtok":    harness.Prices,
		"issues":             list,
		"harness_total":      harnessTot,
		"naive_total":        naiveTot,
		"by_role":            byRole,
		"by_model":           byModel,
		"cache_read_share":   cacheRate,
		"merged_prs":         merged,
		"ai_authored_merged": ai,
		"pr_open_to_merge_h": leadH,
		"prs":                prs,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	must(os.MkdirAll("results", 0o750))
	must(os.WriteFile("results/metrics.json", b, 0o600))
	must(os.MkdirAll("docs", 0o750))
	must(os.WriteFile("docs/data.js", append([]byte("window.METRICS = "), append(b, ';', '\n')...), 0o600))
	fmt.Printf("harness: %d calls $%.4f · naive: %d calls $%.4f · cache read share %.0f%% · AI-authored merged PRs %d/%d\n",
		harnessTot.Calls, harnessTot.CostUSD, naiveTot.Calls, naiveTot.CostUSD, cacheRate*100, ai, merged)
}

func kind(run string) string {
	if len(run) >= 5 && run[:5] == "naive" {
		return "naive"
	}
	return "run"
}

func get(m map[string]*agg, k string) *agg {
	if m[k] == nil {
		m[k] = &agg{}
	}
	return m[k]
}

func readLedger(path string) []harness.Call {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []harness.Call
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var c harness.Call
		if json.Unmarshal(sc.Bytes(), &c) == nil {
			out = append(out, c)
		}
	}
	return out
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func githubPRs() []pr {
	out, err := exec.Command("gh", "pr", "list", "--state", "all", "--limit", "100",
		"--json", "number,title,state,createdAt,mergedAt,labels,additions,deletions,statusCheckRollup").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gh pr list:", err)
		return nil
	}
	var prs []pr
	if err := json.Unmarshal(out, &prs); err != nil {
		fmt.Fprintln(os.Stderr, "gh pr list:", err)
	}
	return prs
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
