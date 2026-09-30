package personal

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
)

func TestReplaceRejectsStalePersonalContent(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	footprints, err := store.Footprints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	beforeTimeline, _, err := store.Timeline(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = store.ReplaceAll(cleanupCtx, footprints, beforeTimeline)
	})
	if err := store.ReplaceFootprints(ctx, footprints); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceFootprints(ctx, footprints); !errors.Is(err, ErrConflict) {
		t.Fatalf("旧版本足迹保存应返回冲突，实际为 %v", err)
	}
	timeline, version, err := store.Timeline(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceTimeline(ctx, timeline, version); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceTimeline(ctx, timeline, version); !errors.Is(err, ErrConflict) {
		t.Fatalf("旧版本实习经历保存应返回冲突，实际为 %v", err)
	}
}
