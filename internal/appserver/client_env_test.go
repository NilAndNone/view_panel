package appserver

import (
	"os"
	"strings"
	"testing"
)

func TestBuildSealedLaunchEnvironmentDoesNotInheritParentHomeValues(t *testing.T) {
	t.Setenv("HOME", "/parent/home")
	t.Setenv("CODEX_HOME", "/parent/.codex")
	t.Setenv("PATH", "/usr/bin")

	env := buildSealedLaunchEnvironment(AppServerLaunchContext{
		Environment: map[string]string{
			"HOME":       "/wrong/home",
			"CODEX_HOME": "/wrong/.codex",
			"FOO":        "bar",
		},
		HomeDir:      "/sealed/home",
		CodexHomeDir: "/sealed/home/.codex",
	})

	got := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			t.Fatalf("invalid environment entry %q", entry)
		}
		got[key] = value
	}

	if got["HOME"] != "/sealed/home" {
		t.Fatalf("HOME = %q, want %q", got["HOME"], "/sealed/home")
	}
	if got["CODEX_HOME"] != "/sealed/home/.codex" {
		t.Fatalf("CODEX_HOME = %q, want %q", got["CODEX_HOME"], "/sealed/home/.codex")
	}
	if got["HOME"] == os.Getenv("HOME") {
		t.Fatalf("HOME leaked from parent env")
	}
	if got["CODEX_HOME"] == os.Getenv("CODEX_HOME") {
		t.Fatalf("CODEX_HOME leaked from parent env")
	}
	if got["HOME"] == "/wrong/home" {
		t.Fatalf("HOME should prefer typed sealed field over generic env map")
	}
	if got["CODEX_HOME"] == "/wrong/.codex" {
		t.Fatalf("CODEX_HOME should prefer typed sealed field over generic env map")
	}
	if got["PATH"] != "/usr/bin" {
		t.Fatalf("PATH = %q, want %q", got["PATH"], "/usr/bin")
	}
	if got["FOO"] != "bar" {
		t.Fatalf("FOO = %q, want %q", got["FOO"], "bar")
	}
}
