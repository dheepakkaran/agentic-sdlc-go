package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go/v2"
)

// Call is one ledger row: every model call is recorded so cost and cache
// hit rate are measured, not guessed.
type Call struct {
	Run        string  `json:"run"`
	Version    string  `json:"harness_version"`
	Issue      int     `json:"issue"`
	Role       Role    `json:"role"`
	Model      string  `json:"model"`
	Attempt    int     `json:"attempt"`
	Input      int64   `json:"input_tokens"`
	Output     int64   `json:"output_tokens"`
	CacheRead  int64   `json:"cache_read_tokens"`
	CacheWrite int64   `json:"cache_write_tokens"`
	CostUSD    float64 `json:"cost_usd"`
	Seconds    float64 `json:"seconds"`
	Stop       string  `json:"stop_reason"`
	Time       string  `json:"time"`
}

// LLM wraps the Anthropic client with routing, caching, structured output,
// a budget cap and an append-only ledger.
type LLM struct {
	Client    anthropic.Client
	Run       string
	Version   string
	Issue     int
	BudgetUSD float64
	Ledger    string // path to results/ledger.jsonl

	mu    sync.Mutex
	spent float64
	Calls []Call
}

var ErrBudget = errors.New("budget cap reached")

// Msg is a turn in a multi-turn role conversation (used by the developer
// retry loop so the earlier prefix stays cached).
type Msg struct {
	Assistant bool
	Text      string
}

// Ask sends shared context + role prompt + turns and decodes the JSON answer
// into out. schema is a JSON Schema for structured output.
//
// Caching (measured, see docs): the output schema is part of the cached
// prefix, so different roles never share a cache entry. One breakpoint at
// the end of the role's system prompt lets the same role reuse it on the
// next issue and on retries. The conversation itself is cached only for
// multi-turn calls (the developer's retry loop), where it is re-sent.
func (l *LLM) Ask(ctx context.Context, role Role, model string, shared, rolePrompt string, turns []Msg, schema map[string]any, attempt int, out any) error {
	r := Routes[role]
	if model == "" {
		model = r.Model
	}
	if l.BudgetUSD > 0 && l.spent >= l.BudgetUSD {
		return ErrBudget
	}
	// Stable content first so the prefix caches: shared project contract +
	// repo map (identical for every role on the same model), then the role.
	system := []anthropic.TextBlockParam{}
	if shared != "" {
		system = append(system, anthropic.TextBlockParam{Text: shared})
	}
	system = append(system, anthropic.TextBlockParam{Text: rolePrompt, CacheControl: anthropic.NewCacheControlEphemeralParam()})

	msgs := make([]anthropic.MessageParam, 0, len(turns))
	for _, t := range turns {
		if t.Assistant {
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(t.Text)))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(t.Text)))
		}
	}
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: r.MaxTokens,
		System:    system,
		Messages:  msgs,
		OutputConfig: anthropic.OutputConfigParam{
			Effort: r.Effort,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	}
	if len(turns) > 1 {
		params.CacheControl = anthropic.NewCacheControlEphemeralParam() // retry loop: cache the growing conversation
	}
	start := time.Now()
	stream := l.Client.Messages.NewStreaming(ctx, params)
	var resp anthropic.Message
	for stream.Next() {
		if err := resp.Accumulate(stream.Current()); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("%s on %s: %w", role, model, err)
	}
	u := resp.Usage
	c := Call{Run: l.Run, Version: l.Version, Issue: l.Issue, Role: role, Model: model, Attempt: attempt,
		Input: u.InputTokens, Output: u.OutputTokens, CacheRead: u.CacheReadInputTokens, CacheWrite: u.CacheCreationInputTokens,
		Seconds: time.Since(start).Seconds(), Stop: string(resp.StopReason), Time: time.Now().UTC().Format(time.RFC3339)}
	c.CostUSD = cost(model, c.Input, c.Output, c.CacheRead, c.CacheWrite)
	l.record(c)

	// Refusal or truncation: re-route once to the escalation model.
	if resp.StopReason == anthropic.StopReasonRefusal || resp.StopReason == anthropic.StopReasonMaxTokens {
		if r.Escalate != "" && model != r.Escalate {
			return l.Ask(ctx, role, r.Escalate, shared, rolePrompt, turns, schema, attempt, out)
		}
		return fmt.Errorf("%s stopped: %s", role, resp.StopReason)
	}
	var text strings.Builder
	for _, b := range resp.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	if err := json.Unmarshal([]byte(text.String()), out); err != nil {
		return fmt.Errorf("%s: bad JSON: %w", role, err)
	}
	return nil
}

func (l *LLM) record(c Call) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.spent += c.CostUSD
	l.Calls = append(l.Calls, c)
	if l.Ledger == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.Ledger), 0o750); err != nil {
		fmt.Fprintln(os.Stderr, "ledger:", err)
		return
	}
	f, err := os.OpenFile(l.Ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ledger:", err)
		return
	}
	defer func() { _ = f.Close() }()
	b, _ := json.Marshal(c)
	if _, err := f.Write(append(b, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, "ledger:", err)
	}
}

// Spent returns the USD total for this run so far.
func (l *LLM) Spent() float64 { l.mu.Lock(); defer l.mu.Unlock(); return l.spent }
