package config

import (
	"strings"
	"testing"
)

func loadTestConfig(getenv func(string) string) (Config, error) {
	return Load(func(key string) string {
		if value := getenv(key); value != "" {
			return value
		}
		switch key {
		case "COS_BUCKET":
			return "test-bucket"
		case "COS_REGION":
			return "ap-test-1"
		case "COS_SECRET_ID":
			return "test-secret-id"
		case "COS_SECRET_KEY":
			return "test-secret-key"
		default:
			return ""
		}
	})
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := loadTestConfig(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL 不能为空") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestLoadDefaultsPort(t *testing.T) {
	cfg, err := loadTestConfig(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://user:pass@localhost:5432/blog"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Port)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	_, err := loadTestConfig(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://user:pass@localhost:5432/blog"
		}
		if key == "PORT" {
			return "70000"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "PORT 必须") {
		t.Fatalf("expected PORT error, got %v", err)
	}
}

func TestLoadRequiresExplicitOptOutForInsecureCookie(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "", want: true},
		{value: "false", want: false},
	} {
		cfg, err := loadTestConfig(func(key string) string {
			if key == "DATABASE_URL" {
				return "postgres://user:pass@localhost:5432/blog"
			}
			if key == "COOKIE_SECURE" {
				return tc.value
			}
			return ""
		})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.CookieSecure != tc.want {
			t.Fatalf("COOKIE_SECURE=%q: expected %t, got %t", tc.value, tc.want, cfg.CookieSecure)
		}
	}
}

func TestLoadRejectsInvalidCookieSecure(t *testing.T) {
	_, err := loadTestConfig(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://user:pass@localhost:5432/blog"
		case "COOKIE_SECURE":
			return "maybe"
		default:
			return ""
		}
	})
	if err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE 必须") {
		t.Fatalf("expected COOKIE_SECURE error, got %v", err)
	}
}
