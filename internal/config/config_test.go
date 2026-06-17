package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MELLO_CONFIG_DIR", dir)
	// clear envs that could leak from the host
	t.Setenv("MELLO_TOKEN", "")
	t.Setenv("MELLO_BASE_URL", "")
	t.Setenv("MELLO_PROFILE", "")
	t.Setenv("MELLO_WORKSPACE", "")

	if err := SetProfile("default", "https://x.example/api/v1", "tok_file", "ws_file", true); err != nil {
		t.Fatal(err)
	}

	// file values win when no env
	r, err := Resolve("")
	if err != nil {
		t.Fatal(err)
	}
	if r.Token != "tok_file" || r.BaseURL != "https://x.example/api/v1" || r.WorkspaceID != "ws_file" {
		t.Fatalf("file resolve = %+v", r)
	}

	// env overrides file
	t.Setenv("MELLO_TOKEN", "tok_env")
	t.Setenv("MELLO_BASE_URL", "https://env.example/api/v1/")
	r, _ = Resolve("")
	if r.Token != "tok_env" {
		t.Errorf("env token not honored: %q", r.Token)
	}
	if r.BaseURL != "https://env.example/api/v1" { // trailing slash trimmed
		t.Errorf("base url = %q", r.BaseURL)
	}
}

func TestSaveMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix perms")
	}
	dir := t.TempDir()
	t.Setenv("MELLO_CONFIG_DIR", dir)
	if err := SetProfile("default", "", "secret", "", true); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("config perms = %v, want 0600", fi.Mode().Perm())
	}
}

func TestClearToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MELLO_CONFIG_DIR", dir)
	t.Setenv("MELLO_TOKEN", "")
	_ = SetProfile("default", "https://x/api", "tok", "", true)
	if err := ClearToken("default"); err != nil {
		t.Fatal(err)
	}
	r, _ := Resolve("")
	if r.Token != "" {
		t.Fatalf("token not cleared: %q", r.Token)
	}
	if r.BaseURL != "https://x/api" {
		t.Fatalf("base url should survive logout: %q", r.BaseURL)
	}
}
