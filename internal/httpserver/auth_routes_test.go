package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/SmileSnow819/YukiBloom-backend/internal/auth"
	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
)

func TestAuthRoutesRequireLogin(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}
	pool, err := database.Open(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	router := NewRouter(auth.NewHandler(auth.NewStore(pool), true), nil, nil, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/session", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问后台会话应返回 401，实际为 %d", response.Code)
	}
}
