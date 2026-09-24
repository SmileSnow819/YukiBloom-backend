package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunRejectsMissingDatabaseConfiguration(t *testing.T) {
	err := run(context.Background(), func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected DATABASE_URL error, got %v", err)
	}
}

func TestRunReportsDatabaseConnectionFailure(t *testing.T) {
	err := run(context.Background(), func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://user:pass@127.0.0.1:1/blog?connect_timeout=1"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatalf("expected database connection error, got %v", err)
	}
	if strings.Contains(err.Error(), "pass") {
		t.Fatalf("error leaked database password: %v", err)
	}
}
