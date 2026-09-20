package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPromoteLegacyEnv(t *testing.T) {
	t.Setenv("RESIM_CLIENT_ID", "legacy-id")
	t.Setenv("RESIM_CLIENT_SECRET", "legacy-secret")
	t.Setenv("RESIM_PROJECT", "legacy-project")
	t.Setenv("SIGNALFLAG_PROJECT", "new-project")
	os.Unsetenv("SIGNALFLAG_CLIENT_ID")
	os.Unsetenv("SIGNALFLAG_CLIENT_SECRET")
	t.Cleanup(func() {
		os.Unsetenv("SIGNALFLAG_CLIENT_ID")
		os.Unsetenv("SIGNALFLAG_CLIENT_SECRET")
	})

	var out bytes.Buffer
	PromoteLegacyEnv(&out)

	if got := os.Getenv("SIGNALFLAG_CLIENT_ID"); got != "legacy-id" {
		t.Errorf("SIGNALFLAG_CLIENT_ID = %q, want %q", got, "legacy-id")
	}
	if got := os.Getenv("SIGNALFLAG_CLIENT_SECRET"); got != "legacy-secret" {
		t.Errorf("SIGNALFLAG_CLIENT_SECRET = %q, want %q", got, "legacy-secret")
	}
	// An explicitly set SIGNALFLAG_* value wins over the legacy one.
	if got := os.Getenv("SIGNALFLAG_PROJECT"); got != "new-project" {
		t.Errorf("SIGNALFLAG_PROJECT = %q, want %q", got, "new-project")
	}

	warning := out.String()
	if !strings.Contains(warning, "deprecated") {
		t.Errorf("expected deprecation warning, got %q", warning)
	}
	for _, name := range []string{"RESIM_CLIENT_ID", "RESIM_CLIENT_SECRET"} {
		if !strings.Contains(warning, name) {
			t.Errorf("warning should mention %s: %q", name, warning)
		}
	}
	if strings.Contains(warning, "RESIM_PROJECT") {
		t.Errorf("warning should not mention RESIM_PROJECT, which was not migrated: %q", warning)
	}
}

func TestPromoteLegacyEnv_NoLegacyVars(t *testing.T) {
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "RESIM_") {
			key, _, _ := strings.Cut(kv, "=")
			t.Setenv(key, "")
			os.Unsetenv(key)
		}
	}
	var out bytes.Buffer
	PromoteLegacyEnv(&out)
	if out.Len() != 0 {
		t.Errorf("expected no warning, got %q", out.String())
	}
}
