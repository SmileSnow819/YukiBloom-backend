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
		switch key {
		case "DATABASE_URL":
			return "postgres://user:pass@127.0.0.1:1/blog?connect_timeout=1"
		case "COS_BUCKET":
			return "test-bucket"
		case "COS_REGION":
			return "ap-shanghai"
		case "COS_SECRET_ID":
			return "test-secret-id"
		case "COS_SECRET_KEY":
			return "test-secret-key"
		}
		return ""
	})
	if err == nil || !strings.Contains(err.Error(), "数据库") {
		t.Fatalf("expected database connection error, got %v", err)
	}
	if strings.Contains(err.Error(), "pass") {
		t.Fatalf("error leaked database password: %v", err)
	}
}
