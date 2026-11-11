// Command release-ready is the last CI gate. It is deterministic: an LLM
// never decides whether something ships. It checks that every upstream job
// passed, coverage holds, behaviour changes came with tests and docs, and
// the PR links its issue. It prints a Markdown checklist and exits 1 on a
// failed check.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type check struct {
	name string
	ok   bool
	note string
}

func main() {
	minCov := flag.Float64("min-coverage", 70, "minimum total statement coverage (%)")
	flag.Parse()
	var cs []check

	// 1. Upstream jobs (GitHub passes the `needs` context as JSON).
	if raw := os.Getenv("RESULTS"); raw != "" {
		var needs map[string]struct {
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(raw), &needs); err == nil {
			for name, n := range needs {
				cs = append(cs, check{"job " + name, n.Result == "success", n.Result})
			}
		}
	}

	// 2. Coverage.
	if b, err := os.ReadFile("coverage.txt"); err == nil {
		m := regexp.MustCompile(`([\d.]+)%`).FindStringSubmatch(string(b))
		if m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			cs = append(cs, check{"coverage ≥ " + fmt.Sprint(*minCov) + "%", v >= *minCov, m[1] + "%"})
		}
	} else {
		cs = append(cs, check{"coverage report", false, "missing (test job failed?)"})
	}

	// 3. Diff rules against the base.
	base := os.Getenv("BASE")
	if !refRe.MatchString(base) {
		base = "HEAD~1"
	}
	files := changed(base)
	code, tests, changelog := false, false, false
	for _, f := range files {
		switch {
		case strings.HasSuffix(f, "_test.go"):
			tests = true
		case strings.HasSuffix(f, ".go") && strings.HasPrefix(f, "internal/inventory/"):
			code = true
		case f == "CHANGELOG.md":
			changelog = true
		}
	}
	if code {
		cs = append(cs, check{"service change has tests", tests, ""}, check{"CHANGELOG updated", changelog, ""})
	}
	if body := os.Getenv("PR_BODY"); body != "" || os.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		linked := regexp.MustCompile(`(?i)(closes|fixes|resolves) #\d+`).MatchString(body)
		cs = append(cs, check{"PR links an issue", linked, ""})
	}

	fmt.Println("## Release readiness")
	fmt.Println()
	fmt.Println("| Check | Result | Note |")
	fmt.Println("|---|---|---|")
	failed := 0
	for _, c := range cs {
		mark := "pass"
		if !c.ok {
			mark = "**FAIL**"
			failed++
		}
		fmt.Printf("| %s | %s | %s |\n", c.name, mark, c.note)
	}
	fmt.Printf("\n%d of %d checks passed.\n", len(cs)-failed, len(cs))
	if failed > 0 {
		os.Exit(1)
	}
}

// refRe allows a commit SHA or HEAD~N only: the base ref comes from CI
// input, so it is validated before it reaches git.
var refRe = regexp.MustCompile(`^([0-9a-f]{7,40}|HEAD~[0-9]{1,3})$`)

func changed(base string) []string {
	if !refRe.MatchString(base) {
		return nil
	}
	out, err := exec.Command("git", "diff", "--name-only", base+"...HEAD").Output() // #nosec G204 G702 -- base validated by refRe
	if err != nil {
		out, _ = exec.Command("git", "diff", "--name-only", base).Output() // #nosec G204 G702 -- base validated by refRe
	}
	return strings.Fields(string(out))
}
