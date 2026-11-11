package harness

import (
	"math"
	"strings"
	"testing"
)

func TestCost(t *testing.T) {
	// 1M input + 1M output on Sonnet = $2 + $10.
	if got := cost(Sonnet, 1e6, 1e6, 0, 0); math.Abs(got-12) > 1e-9 {
		t.Fatalf("sonnet cost %v", got)
	}
	// Cache reads are 20x cheaper than fresh input on Opus.
	if fresh, cached := cost(Opus, 1e6, 0, 0, 0), cost(Opus, 0, 0, 1e6, 0); fresh/cached != 20 {
		t.Fatalf("cache ratio %v", fresh/cached)
	}
}

func TestRoutes(t *testing.T) {
	// Decisions go to the strong model; extraction and writing do not.
	if Routes[Architect].Model != Opus || Routes[QA].Model != Opus {
		t.Error("decision roles must use the strong model")
	}
	if Routes[Analyst].Model != Haiku || Routes[Docs].Model != Haiku {
		t.Error("analyst and docs should use the cheap model")
	}
	if Routes[Developer].Model != Sonnet || Routes[Developer].Escalate != Opus {
		t.Error("developer starts mid-tier and escalates to the strong model")
	}
	for r, rt := range Routes {
		if _, ok := Prices[rt.Model]; !ok {
			t.Errorf("%s routed to a model with no price", r)
		}
	}
}

func TestFailuresFeedback(t *testing.T) {
	long := strings.Repeat("line\n", 100)
	gs := []Gate{{"gofmt", true, ""}, {"go test -race", false, long}}
	f := failures(gs)
	if strings.Contains(f, "gofmt") {
		t.Error("passing gates must not be sent back")
	}
	if n := strings.Count(f, "line"); n != 40 {
		t.Errorf("feedback should keep the last 40 lines, kept %d", n)
	}
	if allOK(gs) || !allOK(gs[:1]) {
		t.Error("allOK wrong")
	}
	if gateLine(gs) != "✓ gofmt  ✗ go test -race" {
		t.Errorf("gateLine: %q", gateLine(gs))
	}
}

func TestSchemaStrict(t *testing.T) {
	for name, s := range map[string]map[string]any{"plan": planSchema, "change": changeSchema, "review": reviewSchema} {
		if s["additionalProperties"] != false {
			t.Errorf("%s schema must forbid extra fields", name)
		}
		if len(s["required"].([]string)) != len(s["properties"].(map[string]any)) {
			t.Errorf("%s: every field must be required", name)
		}
	}
	if got := uniq([]string{"a", "", "b", "a"}); strings.Join(got, ",") != "a,b" {
		t.Errorf("uniq: %v", got)
	}
}

func TestWriteInRootBlocksEscape(t *testing.T) {
	dir := t.TempDir()
	if err := writeInRoot(dir, File{Path: "a/b.go", Content: "package a"}); err != nil {
		t.Fatal(err)
	}
	if err := writeInRoot(dir, File{Path: "../escape.go", Content: "x"}); err == nil {
		t.Fatal("write outside the root must fail")
	}
	if got := ReadFiles(dir, []string{"a/b.go", "new.go"}); !strings.Contains(got, "package a") || !strings.Contains(got, "new.go (new file)") {
		t.Errorf("ReadFiles: %q", got)
	}
}

func TestApplyChange(t *testing.T) {
	dir := t.TempDir()
	if err := writeInRoot(dir, File{Path: "a.go", Content: "package a\n\nfunc A() {}\n\nfunc B() {}\n"}); err != nil {
		t.Fatal(err)
	}
	allowed := []string{"a.go", "a_test.go"}
	ch := Change{
		Edits:    []Edit{{Path: "a.go", Old: "func A() {}", New: "func A() int { return 1 }"}, {Path: "a.go", Old: "func", New: "x"}},
		NewFiles: []File{{Path: "a_test.go", Content: "package a"}, {Path: "b.go", Content: "package a"}},
	}
	changed, problems := applyChange(dir, ch, allowed)
	if strings.Join(changed, ",") != "a.go,a_test.go" {
		t.Errorf("changed: %v", changed)
	}
	if len(problems) != 2 || !strings.Contains(problems[0], "found 2 times") || !strings.Contains(problems[1], "not in the architect's plan") {
		t.Errorf("problems: %v", problems)
	}
	if got := ReadFiles(dir, []string{"a.go"}); !strings.Contains(got, "return 1") {
		t.Errorf("edit not applied: %s", got)
	}
}
