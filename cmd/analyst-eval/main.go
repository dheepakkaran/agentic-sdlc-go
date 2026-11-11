// Command analyst-eval measures which model and effort level should do the
// analyst job: each config answers every issue N times and is scored by the
// same deterministic check the pipeline uses (Requirements.Valid). Results
// go to results/analyst_eval.json and the ledger.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/dheepakkaran/agentic-sdlc-go/internal/harness"
	"github.com/dheepakkaran/agentic-sdlc-go/prompts"
)

type cfg struct {
	Name   string                       `json:"name"`
	Model  string                       `json:"model"`
	Effort anthropic.OutputConfigEffort `json:"effort"`
	Reps   int                          `json:"reps"`
}

type row struct {
	Config  string               `json:"config"`
	Issue   int                  `json:"issue"`
	Rep     int                  `json:"rep"`
	Valid   bool                 `json:"valid"`
	Output  int64                `json:"output_tokens"`
	CostUSD float64              `json:"cost_usd"`
	Answer  harness.Requirements `json:"answer"`
}

func main() {
	configs := []cfg{
		{"haiku-low", harness.Haiku, anthropic.OutputConfigEffortLow, 3},
		{"haiku-medium", harness.Haiku, anthropic.OutputConfigEffortMedium, 3},
		{"sonnet-low", harness.Sonnet, anthropic.OutputConfigEffortLow, 1},
	}
	var rows []row
	for _, c := range configs {
		rt := harness.Routes[harness.Analyst]
		rt.Effort = c.Effort
		harness.Routes[harness.Analyst] = rt
		for n := 1; n <= 3; n++ {
			is := issue(n)
			text := fmt.Sprintf("Issue #%d: %s\n\n%s", is.Number, is.Title, is.Body)
			for rep := 1; rep <= c.Reps; rep++ {
				l := &harness.LLM{Client: anthropic.NewClient(), Run: "analyst-eval-" + c.Name, Version: harness.Version, Issue: n, BudgetUSD: 0.5, Ledger: "results/ledger.jsonl"}
				var req harness.Requirements
				err := l.Ask(context.Background(), harness.Analyst, c.Model, "", prompts.Analyst, []harness.Msg{{Text: text}}, harness.RequirementsSchema, rep, &req)
				if err != nil {
					fmt.Fprintln(os.Stderr, c.Name, n, rep, err)
					continue
				}
				call := l.Calls[len(l.Calls)-1] // refusal/max_tokens re-route would add a call: score the last
				r := row{c.Name, n, rep, req.Valid(), call.Output, l.Spent(), req}
				rows = append(rows, r)
				fmt.Printf("%-13s issue %d rep %d valid=%-5v out=%5d $%.4f\n", c.Name, n, rep, r.Valid, r.Output, r.CostUSD)
			}
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"configs": configs, "rows": rows}, "", "  ")
	if err := os.WriteFile("results/analyst_eval.json", b, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func issue(n int) harness.Issue {
	out, err := exec.Command("gh", "issue", "view", fmt.Sprint(n), "--json", "number,title,body").Output() // #nosec G204 -- n is an int
	if err != nil {
		panic(err)
	}
	var is harness.Issue
	if err := json.Unmarshal(out, &is); err != nil {
		panic(err)
	}
	return is
}
