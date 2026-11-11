package harness

import (
	"os/exec"
	"strings"
)

// Gate is one deterministic check. LLMs never decide these.
type Gate struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Out  string `json:"out,omitempty"`
}

func run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...) // #nosec G204 -- fixed tools (git, go, gofmt), args built by the harness, no shell
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// LocalGates runs gofmt, go vet and go test in dir.
func LocalGates(dir string) []Gate {
	var gs []Gate
	out, _ := run(dir, "gofmt", "-l", ".")
	gs = append(gs, Gate{"gofmt", strings.TrimSpace(out) == "", tail(out, 20)})
	out, err := run(dir, "go", "vet", "./...")
	gs = append(gs, Gate{"go vet", err == nil, tail(out, 30)})
	out, err = run(dir, "go", "test", "-race", "-count=1", "./...")
	gs = append(gs, Gate{"go test -race", err == nil, tail(out, 40)})
	return gs
}

func allOK(gs []Gate) bool {
	for _, g := range gs {
		if !g.OK {
			return false
		}
	}
	return true
}

// failures is the compact feedback sent back to the developer: only failing
// gates, only the last lines. Full logs stay on disk.
func failures(gs []Gate) string {
	var b strings.Builder
	for _, g := range gs {
		if !g.OK {
			b.WriteString("### " + g.Name + " failed\n" + tail(g.Out, 40) + "\n")
		}
	}
	return b.String()
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
