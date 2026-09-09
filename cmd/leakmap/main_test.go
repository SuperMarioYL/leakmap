package main

import (
	"os"
	"strings"
	"testing"
)

// TestVersionMatchesVersionFile is the lockstep drift guard: the CLI version
// const and the repo-root VERSION file must always agree, since the release
// tag (v<version>) is cut from the VERSION file while the CLI banner prints
// the const.
func TestVersionMatchesVersionFile(t *testing.T) {
	b, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatalf("read VERSION file: %v", err)
	}
	want := strings.TrimSpace(string(b))
	if version != want {
		t.Fatalf("version const %q != VERSION file %q — bump both in lockstep", version, want)
	}
}
