package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
)

func TestSessionStoresOnlyTokenHashAndChecksCSRF(t *testing.T) {
	ctx, store := testStore(t)
	adminID, err := store.CreateAdmin(ctx, fmt.Sprintf("session-test-%d", time.Now().UnixNano()), "long test password")
	if err != nil {
		t.Fatal(err)
	}
	token, csrf, err := store.CreateSession(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if token == csrf || len(token) < 32 || len(csrf) < 32 {
		t.Fatal("session secrets were not independent random values")
	}

	var storedHash []byte
	if err := store.pool.QueryRow(ctx, `SELECT token_hash FROM sessions WHERE admin_id = $1`, adminID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256([]byte(token))
	if string(storedHash) != string(wantHash[:]) {
		t.Fatal("session token was not stored as a SHA-256 digest")
	}
	if _, err := store.Authenticate(ctx, token, csrf, true); err != nil {
		t.Fatalf("valid session and CSRF were rejected: %v", err)
	}
	if _, err := store.Authenticate(ctx, token, "wrong", true); !errors.Is(err, ErrCSRF) {
		t.Fatalf("expected CSRF error, got %v", err)
	}
	if _, err := store.Authenticate(ctx, "wrong", csrf, false); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated error, got %v", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	ctx, store := testStore(t)
	adminID, err := store.CreateAdmin(ctx, fmt.Sprintf("expired-test-%d", time.Now().UnixNano()), "long test password")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := store.CreateSession(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE sessions SET expires_at = now() - interval '1 second' WHERE admin_id = $1`, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, token, "", false); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected expired session to fail, got %v", err)
	}
}

func TestFindAdminReturnsStoredPasswordHash(t *testing.T) {
	ctx, store := testStore(t)
	username := fmt.Sprintf("lookup-test-%d", time.Now().UnixNano())
	id, err := store.CreateAdmin(ctx, username, "long test password")
	if err != nil {
		t.Fatal(err)
	}
	foundID, hash, err := store.FindAdmin(ctx, username)
	if err != nil {
		t.Fatal(err)
	}
	if foundID != id || !VerifyPassword(hash, "long test password") {
		t.Fatal("admin lookup returned wrong account or hash")
	}
	if hash == "long test password" {
		t.Fatal("admin password was returned in plain text")
	}
}

func TestDeleteSessionInvalidatesToken(t *testing.T) {
	ctx, store := testStore(t)
	adminID, err := store.CreateAdmin(ctx, fmt.Sprintf("logout-test-%d", time.Now().UnixNano()), "long test password")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := store.CreateSession(ctx, adminID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, token, "", false); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected deleted session to fail, got %v", err)
	}
}

func testStore(t *testing.T) (context.Context, *Store) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return ctx, NewStore(pool)
}
