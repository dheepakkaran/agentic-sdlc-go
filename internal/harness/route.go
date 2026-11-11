// Package harness runs an issue through role agents (analyst → architect →
// developer ⇄ tests → QA → docs) with model routing, context packing, prompt
// caching and a cost ledger.
package harness

import "github.com/anthropics/anthropic-sdk-go/v2"

// Version tags every ledger row, so harness changes are compared on data.
// v1: whole-file developer output, cache breakpoint on every call.
// v2: edit-based developer output, cache only where it is reused.
// v3: cheap-model output is checked and escalated on failure (cascade);
//
//	developer escalates only after a gate failure; QA sends back only
//	findings on planned code files.
//
// v3.1: golangci-lint added to the local gates (shift-left).
const Version = "v3.1"

// Role is one step of the AI-SDLC flow.
type Role string

const (
	Analyst   Role = "analyst"
	Architect Role = "architect"
	Developer Role = "developer"
	QA        Role = "qa"
	Docs      Role = "docs"
	Naive     Role = "naive" // single-call baseline, for comparison only
)

// Route says which model and effort a role gets. Decisions (scope approval,
// merge verdict) go to the strongest model; extraction and writing go to
// cheaper ones. The developer starts on the mid model and is escalated only
// when the test loop keeps failing.
type Route struct {
	Model     string
	Effort    anthropic.OutputConfigEffort
	MaxTokens int64
	Escalate  string // model used after repeated failure or a refusal
}

const (
	Opus   = "claude-opus-5-5"
	Sonnet = "claude-sonnet-5-5"
	Haiku  = "claude-haiku-5-5"
)

var Routes = map[Role]Route{
	Analyst:   {Haiku, anthropic.OutputConfigEffortLow, 4000, Sonnet},
	Architect: {Opus, anthropic.OutputConfigEffortHigh, 16000, ""},
	Developer: {Sonnet, anthropic.OutputConfigEffortMedium, 32000, Opus},
	QA:        {Opus, anthropic.OutputConfigEffortHigh, 16000, ""},
	Docs:      {Haiku, anthropic.OutputConfigEffortLow, 4000, Sonnet},
	Naive:     {Opus, anthropic.OutputConfigEffortHigh, 64000, ""},
}

// Price is USD per million tokens (first-party API list prices, Nov 2026).
type Price struct{ In, Out, CacheRead, CacheWrite float64 }

var Prices = map[string]Price{
	Opus:   {4.00, 20.00, 0.20, 5.00},
	Sonnet: {2.00, 10.00, 0.10, 2.50},
	Haiku:  {0.10, 0.50, 0.01, 0.125},
}

func cost(model string, in, out, cr, cw int64) float64 {
	p := Prices[model]
	return (float64(in)*p.In + float64(out)*p.Out + float64(cr)*p.CacheRead + float64(cw)*p.CacheWrite) / 1e6
}
