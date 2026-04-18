package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempHome sets HOME (and USERPROFILE on Windows) to a temp dir for the
// duration of the test, ensuring Load/Save never touch the real home directory.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // Windows
	return dir
}

func TestLoad_Defaults_WhenNoFile(t *testing.T) {
	withTempHome(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "http://localhost:3000" {
		t.Errorf("expected default server, got %q", cfg.Server)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty token, got %q", cfg.Token)
	}
}

func TestLoad_ParsesFile(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".config", "lunar")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	content := "server: http://myserver:9000\ntoken: tok123\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server != "http://myserver:9000" {
		t.Errorf("got server %q", cfg.Server)
	}
	if cfg.Token != "tok123" {
		t.Errorf("got token %q", cfg.Token)
	}
}

func TestLoad_EmptyServer_FallsBackToDefault(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".config", "lunar")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("server: \ntoken: t\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, _ := Load()
	if cfg.Server != "http://localhost:3000" {
		t.Errorf("expected default server when empty, got %q", cfg.Server)
	}
}

func TestLoad_InvalidYAML_ReturnsDefaults(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".config", "lunar")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(":::not yaml:::"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, _ := Load()
	if cfg.Server != "http://localhost:3000" {
		t.Errorf("expected default server on parse error, got %q", cfg.Server)
	}
}

func TestSave_RoundTrip(t *testing.T) {
	withTempHome(t)
	want := &Config{
		Server: "http://remote:4000",
		Token:  "mytoken",
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Server != want.Server {
		t.Errorf("server: got %q, want %q", got.Server, want.Server)
	}
	if got.Token != want.Token {
		t.Errorf("token: got %q, want %q", got.Token, want.Token)
	}
}

func TestSave_CreatesDirectories(t *testing.T) {
	home := withTempHome(t)
	// The .config/lunar dir does not exist yet.
	if err := Save(&Config{Server: "http://x", Token: "t"}); err != nil {
		t.Fatalf("Save should create missing dirs: %v", err)
	}
	path := filepath.Join(home, ".config", "lunar", "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestSave_FilePermissions(t *testing.T) {
	withTempHome(t)
	if err := Save(&Config{Server: "http://x", Token: "t"}); err != nil {
		t.Fatal(err)
	}
	path, _ := configPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected file permission 0600, got %04o", perm)
	}
}
