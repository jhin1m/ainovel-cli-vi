package server

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSecret(t *testing.T, dir, name, content string, perm os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
	return p
}

func testMasterKeyB64() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfigFromFiles(t *testing.T) {
	dir := t.TempDir()
	env := map[string]string{
		"AINOVEL_DATABASE_URL_FILE": writeSecret(t, dir, "db", "postgres://u:p@h/db\n", 0o600),
		"AINOVEL_MASTER_KEY_FILE":   writeSecret(t, dir, "mk", testMasterKeyB64()+"\n", 0o400),
		"AINOVEL_TRUSTED_PROXIES":   "127.0.0.1/32, ::1",
		"AINOVEL_MAX_JOBS":          "8",
		"AINOVEL_PUBLIC_URL":        "https://novel.example.com/",
	}
	cfg, err := LoadConfig(envMap(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://u:p@h/db" || len(cfg.MasterKey) != 32 {
		t.Fatalf("secret sai: %q %d", cfg.DatabaseURL, len(cfg.MasterKey))
	}
	if cfg.Listen != "127.0.0.1:8080" || cfg.DataDir != "/var/lib/ainovel" || cfg.PublicURL != "https://novel.example.com" {
		t.Fatalf("mặc định sai: %+v", cfg)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.Limits.MaxJobs != 8 || cfg.Limits.MaxJobsPerUser != 1 ||
		cfg.Limits.MaxBooksPerUser != 20 || cfg.Limits.MaxBookBudgetUSD != 50 || cfg.Limits.MaxExports != 2 {
		t.Fatalf("giới hạn sai: %+v %+v", cfg.TrustedProxies, cfg.Limits)
	}
	if len(cfg.Warnings) != 0 {
		t.Fatalf("không được cảnh báo: %v", cfg.Warnings)
	}
	if cfg.ClientIPHeader != "X-Forwarded-For" {
		t.Fatalf("header IP mặc định sai: %q", cfg.ClientIPHeader)
	}
}

func TestLoadConfigErrors(t *testing.T) {
	dir := t.TempDir()
	db := writeSecret(t, dir, "db", "postgres://u:p@h/db", 0o600)
	mk := writeSecret(t, dir, "mk", testMasterKeyB64(), 0o600)
	cases := map[string]map[string]string{
		"thiếu file DB":                 {"AINOVEL_MASTER_KEY_FILE": mk},
		"DB qua env ngoài dev":          {"AINOVEL_DATABASE_URL": "postgres://x", "AINOVEL_MASTER_KEY_FILE": mk},
		"file DB không tồn tại":         {"AINOVEL_DATABASE_URL_FILE": filepath.Join(dir, "nope"), "AINOVEL_MASTER_KEY_FILE": mk},
		"thiếu master key":              {"AINOVEL_DATABASE_URL_FILE": db},
		"master key ngắn":               {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_MASTER_KEY_FILE": writeSecret(t, dir, "short", "YWJj", 0o600)},
		"trusted proxies sai":           {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_MASTER_KEY_FILE": mk, "AINOVEL_TRUSTED_PROXIES": "abc"},
		"giới hạn âm":                   {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_MASTER_KEY_FILE": mk, "AINOVEL_MAX_JOBS": "-1"},
		"public url không phải http(s)": {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_MASTER_KEY_FILE": mk, "AINOVEL_PUBLIC_URL": "ftp://x"},
		"public url có đường dẫn":       {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_MASTER_KEY_FILE": mk, "AINOVEL_PUBLIC_URL": "https://x.vn/app"},
		"DB env kèm file ngoài dev":     {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_DATABASE_URL": "postgres://x", "AINOVEL_MASTER_KEY_FILE": mk},
		"header IP lạ":                  {"AINOVEL_DATABASE_URL_FILE": db, "AINOVEL_MASTER_KEY_FILE": mk, "AINOVEL_CLIENT_IP_HEADER": "X-Real-IP"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadConfig(envMap(env)); err == nil {
				t.Fatal("phải lỗi")
			}
		})
	}
}

func TestLoadConfigDevAllowsEnvDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(envMap(map[string]string{
		"AINOVEL_DEV":             "true",
		"AINOVEL_DATABASE_URL":    "postgres://dev",
		"AINOVEL_MASTER_KEY_FILE": writeSecret(t, dir, "mk", strings.Repeat("ab", 32), 0o644),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Dev || cfg.DatabaseURL != "postgres://dev" || len(cfg.MasterKey) != 32 {
		t.Fatalf("dev config sai: %+v", cfg)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], "0600") {
		t.Fatalf("file 0644 phải bị cảnh báo: %v", cfg.Warnings)
	}
}
