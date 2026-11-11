package harness

import (
	"strings"
	"testing"
)

func TestCheckWrite(t *testing.T) {
	allowed := []string{"internal/inventory/store.go", "internal/inventory/store_test.go"}
	cases := []struct {
		path, content string
		ok            bool
	}{
		{"internal/inventory/store.go", "package inventory", true},
		{"internal/inventory/http.go", "package inventory", false},                        // not planned
		{".github/workflows/ci.yml", "on: push", false},                                   // protected
		{"../etc/passwd", "x", false},                                                     // escape
		{"internal/inventory/store_test.go", "key := \"sk-ant-abcdefghijklmnop\"", false}, // secret
	}
	for _, c := range cases {
		// ci.yml is in the allowed list on purpose: protected paths win over the plan.
		err := CheckWrite(c.path, c.content, append(allowed, ".github/workflows/ci.yml"))
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v want ok=%v", c.path, err, c.ok)
		}
	}
}

func TestRepoMap(t *testing.T) {
	m, err := RepoMap("../..", []string{"internal/inventory"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"func (s *Store) Create(d Device) (Device, error)", "type Device struct"} {
		if !contains(m, want) {
			t.Errorf("repo map missing %q", want)
		}
	}
	if contains(m, "s.mu.Lock()") {
		t.Error("repo map must not include function bodies")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
