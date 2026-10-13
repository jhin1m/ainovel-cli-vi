package server

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Limits gom mọi giới hạn tài nguyên của server ở một chỗ.
type Limits struct {
	MaxJobs          int     // worker sống đồng thời toàn server
	MaxJobsPerUser   int     // worker sống đồng thời mỗi user
	MaxBooksPerUser  int     // số truyện tối đa mỗi user
	MaxBookBudgetUSD float64 // trần ngân sách mỗi truyện
	MaxSSEPerUser    int     // kết nối SSE mỗi user
	MaxSSETotal      int     // kết nối SSE toàn server
	MaxExports       int     // export chạy đồng thời toàn server
}

// Config là cấu hình của `ainovel-cli serve`. Secret (DB URL, master key) chỉ
// đọc từ file: env của server đọc được qua /proc/<pid>/environ bởi cùng UID.
type Config struct {
	Listen         string
	DataDir        string
	PublicURL      string
	TrustedProxies []netip.Prefix
	ClientIPHeader string // header chứa IP client do proxy tin cậy đặt
	Dev            bool

	DatabaseURL string
	MasterKey   []byte // đúng 32 byte

	Limits Limits

	// DevWorkerBin: chỉ ở dev — chạy chương trình này thay cho chính binary làm
	// worker (worker giả cho e2e, không cần LLM).
	DevWorkerBin string

	// Warnings là các cảnh báo không chặn khởi động (vd. file secret quyền rộng).
	Warnings []string
}

// LoadConfig đọc cấu hình từ env (getenv) và các file secret.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Listen:    envOr(getenv, "AINOVEL_LISTEN", "127.0.0.1:8080"),
		DataDir:   envOr(getenv, "AINOVEL_DATA_DIR", "/var/lib/ainovel"),
		PublicURL: strings.TrimRight(strings.TrimSpace(getenv("AINOVEL_PUBLIC_URL")), "/"),
	}
	var err error
	if cfg.Dev, err = envBool(getenv, "AINOVEL_DEV", false); err != nil {
		return Config{}, err
	}
	if cfg.PublicURL != "" {
		u, err := url.Parse(cfg.PublicURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
			return Config{}, fmt.Errorf("AINOVEL_PUBLIC_URL không hợp lệ: cần dạng https://ten-mien (không có đường dẫn)")
		}
	}
	if cfg.TrustedProxies, err = parseCIDRs(getenv("AINOVEL_TRUSTED_PROXIES")); err != nil {
		return Config{}, err
	}
	switch h := envOr(getenv, "AINOVEL_CLIENT_IP_HEADER", "X-Forwarded-For"); strings.ToLower(h) {
	case "x-forwarded-for":
		cfg.ClientIPHeader = "X-Forwarded-For"
	case "cf-connecting-ip":
		cfg.ClientIPHeader = "CF-Connecting-IP"
	default:
		return Config{}, fmt.Errorf("AINOVEL_CLIENT_IP_HEADER chỉ nhận X-Forwarded-For hoặc CF-Connecting-IP")
	}

	// Database URL: file là đường chính; env thẳng chỉ cho phép ở dev (env của
	// server đọc được qua /proc bởi cùng UID, kể cả khi đã có file).
	if v := strings.TrimSpace(getenv("AINOVEL_DATABASE_URL")); v != "" && !cfg.Dev {
		return Config{}, errors.New("AINOVEL_DATABASE_URL chỉ dùng được khi AINOVEL_DEV=true; ở production hãy dùng AINOVEL_DATABASE_URL_FILE")
	}
	if path := strings.TrimSpace(getenv("AINOVEL_DATABASE_URL_FILE")); path != "" {
		data, warn, err := readSecretFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("AINOVEL_DATABASE_URL_FILE: %w", err)
		}
		cfg.addWarning(warn)
		cfg.DatabaseURL = strings.TrimSpace(string(data))
	} else if v := strings.TrimSpace(getenv("AINOVEL_DATABASE_URL")); v != "" {
		cfg.DatabaseURL = v
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("thiếu AINOVEL_DATABASE_URL_FILE (đường dẫn file chứa chuỗi kết nối Postgres)")
	}

	path := strings.TrimSpace(getenv("AINOVEL_MASTER_KEY_FILE"))
	if path == "" {
		return Config{}, errors.New("thiếu AINOVEL_MASTER_KEY_FILE (file chứa master key 32 byte, base64); tạo bằng: openssl rand -base64 32")
	}
	data, warn, err := readSecretFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("AINOVEL_MASTER_KEY_FILE: %w", err)
	}
	cfg.addWarning(warn)
	if cfg.MasterKey, err = parseMasterKey(data); err != nil {
		return Config{}, fmt.Errorf("AINOVEL_MASTER_KEY_FILE: %w", err)
	}

	if cfg.Limits, err = loadLimits(getenv); err != nil {
		return Config{}, err
	}
	if v := strings.TrimSpace(getenv("AINOVEL_DEV_WORKER_BIN")); v != "" {
		if !cfg.Dev {
			return Config{}, errors.New("AINOVEL_DEV_WORKER_BIN chỉ dùng được khi AINOVEL_DEV=true")
		}
		cfg.DevWorkerBin = v
	}
	return cfg, nil
}

func (c *Config) addWarning(w string) {
	if w != "" {
		c.Warnings = append(c.Warnings, w)
	}
}

func loadLimits(getenv func(string) string) (Limits, error) {
	var l Limits
	ints := []struct {
		name string
		dst  *int
		def  int
	}{
		{"AINOVEL_MAX_JOBS", &l.MaxJobs, 4},
		{"AINOVEL_MAX_JOBS_PER_USER", &l.MaxJobsPerUser, 1},
		{"AINOVEL_MAX_BOOKS_PER_USER", &l.MaxBooksPerUser, 20},
		{"AINOVEL_MAX_SSE_PER_USER", &l.MaxSSEPerUser, 4},
		{"AINOVEL_MAX_SSE_TOTAL", &l.MaxSSETotal, 200},
		{"AINOVEL_MAX_EXPORTS", &l.MaxExports, 2},
	}
	for _, it := range ints {
		v, err := envInt(getenv, it.name, it.def)
		if err != nil {
			return Limits{}, err
		}
		*it.dst = v
	}
	budget := strings.TrimSpace(getenv("AINOVEL_MAX_BOOK_BUDGET_USD"))
	l.MaxBookBudgetUSD = 50
	if budget != "" {
		f, err := strconv.ParseFloat(budget, 64)
		if err != nil || !(f > 0) {
			return Limits{}, fmt.Errorf("AINOVEL_MAX_BOOK_BUDGET_USD phải là số > 0")
		}
		l.MaxBookBudgetUSD = f
	}
	return l, nil
}

func envOr(getenv func(string) string, name, def string) string {
	if v := strings.TrimSpace(getenv(name)); v != "" {
		return v
	}
	return def
}

func envBool(getenv func(string) string, name string, def bool) (bool, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s phải là true/false", name)
	}
	return b, nil
}

func envInt(getenv func(string) string, name string, def int) (int, error) {
	v := strings.TrimSpace(getenv(name))
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s phải là số nguyên > 0", name)
	}
	return n, nil
}

func parseCIDRs(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			addr, err := netip.ParseAddr(part)
			if err != nil {
				return nil, fmt.Errorf("AINOVEL_TRUSTED_PROXIES: %q không phải IP/CIDR", part)
			}
			out = append(out, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("AINOVEL_TRUSTED_PROXIES: %q không phải CIDR", part)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// readSecretFile đọc file secret; quyền rộng hơn 0600 chỉ cảnh báo (systemd
// LoadCredential tự đặt quyền chặt, còn ở dev người dùng có thể để 0644).
func readSecretFile(path string) ([]byte, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, "", err
	}
	if fi.IsDir() {
		return nil, "", fmt.Errorf("%s là thư mục", path)
	}
	var warn string
	if fi.Mode().Perm()&0o077 != 0 {
		warn = fmt.Sprintf("file secret %s có quyền %o, nên đặt 0600 hoặc 0400", path, fi.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	return data, warn, nil
}

// parseMasterKey nhận 32 byte dạng base64 (chuẩn hoặc URL-safe) hoặc hex.
func parseMasterKey(data []byte) ([]byte, error) {
	s := strings.TrimSpace(string(data))
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if key, err := enc.DecodeString(s); err == nil && len(key) == 32 {
			return key, nil
		}
	}
	if key, err := hex.DecodeString(s); err == nil && len(key) == 32 {
		return key, nil
	}
	return nil, errors.New("master key phải là 32 byte mã hoá base64 (openssl rand -base64 32)")
}
