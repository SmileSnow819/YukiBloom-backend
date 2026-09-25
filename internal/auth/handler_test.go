package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLoginAndProtectedMutation(t *testing.T) {
	ctx, store := testStore(t)
	username := fmt.Sprintf("login-test-%d", time.Now().UnixNano())
	if _, err := store.CreateAdmin(ctx, username, "long test password"); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(store, true)
	router := gin.New()
	router.POST("/login", handler.Login)
	router.GET("/protected", handler.RequireSession(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/protected", handler.RequireSession(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	wrong := postJSON(router, "/login", fmt.Sprintf(`{"username":%q,"password":"wrong"}`, username), nil, "")
	if wrong.Code != http.StatusUnauthorized || len(wrong.Result().Cookies()) != 0 {
		t.Fatalf("wrong password response: status %d, cookies %d", wrong.Code, len(wrong.Result().Cookies()))
	}
	login := postJSON(router, "/login", fmt.Sprintf(`{"username":%q,"password":"long test password"}`, username), nil, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "yb_session" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
	var loginBody struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginBody); err != nil || loginBody.Code != 0 || loginBody.Message != "成功" || loginBody.Data.CSRFToken == "" {
		t.Fatalf("missing CSRF token: %v", err)
	}

	withoutSession := httptest.NewRecorder()
	router.ServeHTTP(withoutSession, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if withoutSession.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", withoutSession.Code)
	}
	withoutCSRF := postJSON(router, "/protected", `{}`, cookies[0], "")
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", withoutCSRF.Code)
	}
	withCSRF := postJSON(router, "/protected", `{}`, cookies[0], loginBody.Data.CSRFToken)
	if withCSRF.Code != http.StatusNoContent {
		t.Fatalf("expected 204 with session and CSRF, got %d", withCSRF.Code)
	}
}

func TestLoginLimitsRepeatedAttempts(t *testing.T) {
	_, store := testStore(t)
	handler := NewHandler(store, false)
	router := gin.New()
	router.POST("/login", handler.Login)
	for i := 0; i < 5; i++ {
		response := postJSON(router, "/login", `{"username":"missing","password":"wrong"}`, nil, "")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, response.Code)
		}
	}
	blocked := postJSON(router, "/login", `{"username":"missing","password":"wrong"}`, nil, "")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after repeated attempts, got %d", blocked.Code)
	}
}

func postJSON(router *gin.Engine, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if strings.TrimSpace(csrf) != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
