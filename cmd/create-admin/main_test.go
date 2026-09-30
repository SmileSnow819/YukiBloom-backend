package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunRequiresAdminCredentials(t *testing.T) {
	var output bytes.Buffer
	getenv := func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://localhost/test"
		}
		return ""
	}
	err := run(context.Background(), getenv, &output)
	if err == nil || !strings.Contains(err.Error(), "ADMIN_USERNAME") || !strings.Contains(err.Error(), "ADMIN_PASSWORD") {
		t.Fatalf("缺少管理员账号信息时应返回中文提示，实际为 %v", err)
	}
}
