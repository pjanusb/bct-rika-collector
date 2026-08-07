package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := writeConfig(t, "IP=192.168.222.2\nUSER=root\nPASSWRD=!C0vF3F3\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.IP != "192.168.222.2" || cfg.User != "root" || cfg.Password != "!C0vF3F3" {
		t.Fatalf("Load() = %#v", cfg)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := writeConfig(t, "IP=192.168.222.2\nUSER=root\nPASSWRD=x\nPORT=22\n")

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown configuration key") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsNonRootUser(t *testing.T) {
	path := writeConfig(t, "IP=192.168.222.2\nUSER=admin\nPASSWRD=x\n")

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "USER must be root") {
		t.Fatalf("Load() error = %v", err)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
