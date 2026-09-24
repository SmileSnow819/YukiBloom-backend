package database

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationNamesAreSorted(t *testing.T) {
	names, err := migrationNames()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"0001_initial.sql", "0002_content.sql"}) {
		t.Fatalf("unexpected migration files: %v", names)
	}
}

func TestPostsRejectDuplicateLocaleSlug(t *testing.T) {
	ctx, pool := testContentDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	slug := "duplicate-" + t.Name()
	for attempt := 0; attempt < 2; attempt++ {
		_, err = tx.Exec(ctx, `INSERT INTO posts (locale, slug, title, body_markdown) VALUES ('zh', $1, '标题', '正文')`, slug)
		if attempt == 0 && err != nil {
			t.Fatal(err)
		}
	}
	if err == nil {
		t.Fatal("expected duplicate locale and slug to fail")
	}
}

func TestPostsRejectInvalidStatus(t *testing.T) {
	ctx, pool := testContentDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO posts (locale, slug, title, body_markdown, status) VALUES ('zh', 'invalid-status', '标题', '正文', 'hidden')`)
	if err == nil {
		t.Fatal("expected invalid status to fail")
	}
}

func TestMediaCannotBeDeletedWhileCoverIsReferenced(t *testing.T) {
	ctx, pool := testContentDatabase(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var mediaID string
	if err := tx.QueryRow(ctx, `INSERT INTO media (storage_key, mime_type, size_bytes) VALUES ('test-cover.png', 'image/png', 10) RETURNING id`).Scan(&mediaID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO posts (locale, slug, title, body_markdown, cover_media_id) VALUES ('zh', 'media-test', '标题', '正文', $1)`, mediaID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM media WHERE id = $1`, mediaID); err == nil {
		t.Fatal("expected referenced media deletion to fail")
	}
}

func testContentDatabase(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return ctx, pool
}

func TestMigrateIsRepeatable(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = '0001_initial.sql'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one migration record, got %d", count)
	}
}

func TestFailedMigrationDoesNotRecordVersion(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	const version = "test_invalid_migration.sql"
	if err := runMigration(ctx, pool, version, "THIS IS INVALID SQL;"); err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = $1`, version).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed migration was recorded: %d", count)
	}
}
