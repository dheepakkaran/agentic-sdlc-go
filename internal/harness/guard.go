package harness

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Guardrails on what an agent may write. The developer can only touch the
// paths the architect approved, never CI, prompts, templates or go.mod, and
// output is scanned for secrets before it reaches disk.
var denied = []string{".github/", "prompts/", "templates/", "internal/harness/", "cmd/sdlc/", "go.mod", "go.sum", ".env"}

var secretRe = regexp.MustCompile(`(sk-ant-[A-Za-z0-9_-]{10,}|ghp_[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY)`)

// CheckWrite returns an error if path/content break a guardrail.
func CheckWrite(path, content string, allowed []string) error {
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, "../") || filepath.IsAbs(path) {
		return fmt.Errorf("path escapes repo: %s", path)
	}
	for _, d := range denied {
		if clean == d || strings.HasPrefix(clean, d) {
			return fmt.Errorf("path is protected: %s", path)
		}
	}
	ok := false
	for _, a := range allowed {
		if clean == filepath.ToSlash(filepath.Clean(a)) {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("path not in the architect's plan: %s", path)
	}
	if secretRe.MatchString(content) {
		return fmt.Errorf("possible secret in %s", path)
	}
	return nil
}
