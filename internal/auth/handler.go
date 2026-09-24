package auth

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const sessionCookieName = "yb_session"

type loginAttempts struct {
	count int
	until time.Time
}

type Handler struct {
	store  *Store
	secure bool
	mu     sync.Mutex
	limits map[string]loginAttempts
}

func NewHandler(store *Store, secure bool) *Handler {
	return &Handler{store: store, secure: secure, limits: make(map[string]loginAttempts)}
}

func (h *Handler) Login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Username) == "" || input.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写用户名和密码"})
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	key := c.ClientIP() + ":" + input.Username
	if h.isBlocked(key) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过多，请稍后再试"})
		return
	}
	adminID, hash, err := h.store.FindAdmin(c.Request.Context(), input.Username)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录暂时不可用，请稍后再试"})
		return
	}
	if err != nil || !VerifyPassword(hash, input.Password) {
		h.recordFailure(key)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	token, csrf, err := h.store.CreateSession(c.Request.Context(), adminID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录暂时不可用，请稍后再试"})
		return
	}
	h.clearFailures(key)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, token, 7*24*60*60, "/api/v1", "", h.secure, true)
	c.JSON(http.StatusOK, gin.H{"csrfToken": csrf})
}

func (h *Handler) RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		mutation := c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions
		adminID, err := h.store.Authenticate(c.Request.Context(), cookie, c.GetHeader("X-CSRF-Token"), mutation)
		if errors.Is(err, ErrUnauthenticated) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			return
		}
		if errors.Is(err, ErrCSRF) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "请求校验失败，请刷新页面后重试"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "验证登录状态失败，请稍后再试"})
			return
		}
		c.Set("adminID", adminID)
		c.Next()
	}
}

func (h *Handler) Logout(c *gin.Context) {
	token, _ := c.Cookie(sessionCookieName)
	if err := h.store.DeleteSession(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "退出登录失败，请稍后再试"})
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/api/v1", "", h.secure, true)
	c.Status(http.StatusNoContent)
}

func (h *Handler) Session(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"authenticated": true})
}

func (h *Handler) isBlocked(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	attempts, ok := h.limits[key]
	if !ok || time.Now().After(attempts.until) {
		delete(h.limits, key)
		return false
	}
	return attempts.count >= 5
}

func (h *Handler) recordFailure(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	attempts := h.limits[key]
	if now.After(attempts.until) {
		attempts = loginAttempts{until: now.Add(5 * time.Minute)}
	}
	attempts.count++
	h.limits[key] = attempts
	// 定期清理过期记录，避免长期运行时持续占用内存。
	if len(h.limits) > 1000 {
		for k, value := range h.limits {
			if now.After(value.until) {
				delete(h.limits, k)
			}
		}
	}
}

func (h *Handler) clearFailures(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.limits, key)
}
