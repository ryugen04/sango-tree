package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	// 正常パース
	content := `DB_HOST=localhost
DB_PORT=5432
# コメント行

DB_NAME=mydb
SECRET_KEY="my secret"
SINGLE_QUOTED='value'
WITH_EQUALS=a=b=c
`
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	os.WriteFile(envPath, []byte(content), 0o644)

	env, err := LoadEnvFile(envPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := map[string]string{
		"DB_HOST":       "localhost",
		"DB_PORT":       "5432",
		"DB_NAME":       "mydb",
		"SECRET_KEY":    "my secret",
		"SINGLE_QUOTED": "value",
		"WITH_EQUALS":   "a=b=c",
	}
	for k, want := range tests {
		if got := env[k]; got != want {
			t.Errorf("%s: got %q, want %q", k, got, want)
		}
	}
}

func TestLoadEnvFileNotExists(t *testing.T) {
	_, err := LoadEnvFile("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
