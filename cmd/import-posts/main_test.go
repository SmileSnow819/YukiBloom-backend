package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
)

func TestApplyRollsBackAllPostsWhenLaterPostFails(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	first := fmt.Sprintf("cr-atomic-first-%d", time.Now().UnixNano())
	second := first + "-fail"
	var mediaBefore int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM media`).Scan(&mediaBefore); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE posts ADD CONSTRAINT cr_import_posts_test_block CHECK (slug <> '`+second+`')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `ALTER TABLE posts DROP CONSTRAINT IF EXISTS cr_import_posts_test_block`)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM posts WHERE slug IN ($1,$2)`, first, second)
	})
	source := t.TempDir()
	assets := t.TempDir()
	if err := os.MkdirAll(filepath.Join(assets, "img"), 0o700); err != nil {
		t.Fatal(err)
	}
	coverFile, err := os.Create(filepath.Join(assets, "img", "cover.png"))
	if err != nil {
		t.Fatal(err)
	}
	cover := image.NewRGBA(image.Rect(0, 0, 1, 1))
	cover.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(coverFile, cover); err != nil {
		t.Fatal(err)
	}
	if err := coverFile.Close(); err != nil {
		t.Fatal(err)
	}
	for name, slug := range map[string]string{"01.md": first, "02.md": second} {
		coverField := ""
		if name == "01.md" {
			coverField = "cover: /img/cover.png\n"
		}
		markdown := fmt.Sprintf("---\ntitle: 测试文章\nlink: %s\ndate: '2026-09-28'\n%s---\n正文\n", slug, coverField)
		if err := os.WriteFile(filepath.Join(source, name), []byte(markdown), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	uploadDir := t.TempDir()
	getenv := func(key string) string {
		switch key {
		case "DATABASE_URL":
			return url
		case "UPLOAD_DIR":
			return uploadDir
		default:
			return ""
		}
	}
	if err := run(ctx, []string{"-source", source, "-assets", assets, "-apply"}, getenv, io.Discard); err == nil {
		t.Fatal("第二篇文章应触发数据库错误")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM posts WHERE slug IN ($1,$2)`, first, second).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("整批导入失败后仍留下 %d 篇文章", count)
	}
	var mediaAfter int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM media`).Scan(&mediaAfter); err != nil {
		t.Fatal(err)
	}
	if mediaAfter != mediaBefore {
		t.Fatalf("整批导入失败后图片记录从 %d 变成 %d", mediaBefore, mediaAfter)
	}
	files, err := os.ReadDir(uploadDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("整批导入失败后仍留下 %d 个图片文件", len(files))
	}
}
