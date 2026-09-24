package database

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenRejectsInvalidDatabaseURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := Open(ctx, "not a database URL")
	if err == nil || !strings.Contains(err.Error(), "数据库连接地址") {
		t.Fatalf("数据库连接地址错误时应返回中文提示，实际为 %v", err)
	}
}

func TestOpenPingsTestDatabase(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}
