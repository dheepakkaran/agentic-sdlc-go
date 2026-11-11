package main

import "testing"

func TestRefRe(t *testing.T) {
	for ref, ok := range map[string]bool{
		"HEAD~1": true, "3f2a9c1": true, "0123456789abcdef0123456789abcdef01234567": true,
		"main; rm -rf /": false, "--output=/etc/x": false, "": false, "HEAD~1000": false,
	} {
		if refRe.MatchString(ref) != ok {
			t.Errorf("%q: want %v", ref, ok)
		}
	}
}
