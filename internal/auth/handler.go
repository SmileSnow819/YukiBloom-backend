package auth

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/apiresponse"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const sessionCookieName = "yb_session"

type loginAttempts struct {
	count int
	until time.Time
}

// LoginInput 是管理员登录接口接收的凭据。
type LoginInput struct {
	// Username 是管理员用户名。
	Username string `json:"username"`
	// Password 是管理员密码。
	Password string `json:"password"`
}

type Handler struct {
	store  *Store
	secure bool
	mu     sync.Mutex
	limits map[string]loginAttempts
}

// NewHandler 创建登录处理器，并初始化登录失败次数记录。
// 参数：store 是会话存储；secure 指定会话 Cookie 是否仅通过 HTTPS 发送。
// 返回：配置好的 Handler。
func NewHandler(store *Store, secure bool) *Handler {
	return &Handler{store: store, secure: secure, limits: make(map[string]loginAttempts)}
}

// Login 校验管理员凭据，建立会话并写入登录响应。
// 参数：h 是登录处理器；c 是请求上下文，用于读取登录信息和写入响应。
// 返回：无。
// @Summary 管理员登录
// @Description 验证管理员账号并设置 HttpOnly 会话 Cookie。成功响应中的 csrfToken 用于后续写请求。
// @Tags 管理员会话
// @Accept json
// @Produce json
// @Param request body LoginInput true "登录凭据"
// @Success 200 {object} apiresponse.Envelope{data=map[string]string} "登录成功，返回 csrfToken，并通过 Set-Cookie 设置会话"
// @Failure 400 {object} apiresponse.Envelope "code=10001，用户名或密码为空或格式错误"
// @Failure 401 {object} apiresponse.Envelope "code=10002，用户名或密码错误"
// @Failure 429 {object} apiresponse.Envelope "code=10007，登录尝试过多"
// @Failure 500 {object} apiresponse.Envelope "code=50000，登录服务暂不可用"
// @Router /api/v1/admin/login [post]
func (h *Handler) Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Username) == "" || input.Password == "" {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "请填写用户名和密码")
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	key := c.ClientIP() + ":" + input.Username
	if h.isBlocked(key) {
		apiresponse.Failure(c, apiresponse.RateLimited, "登录尝试过多，请稍后再试")
		return
	}
	adminID, hash, err := h.store.FindAdmin(c.Request.Context(), input.Username)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		apiresponse.Failure(c, apiresponse.InternalError, "登录暂时不可用，请稍后再试")
		return
	}
	if err != nil || !VerifyPassword(hash, input.Password) {
		h.recordFailure(key)
		apiresponse.Failure(c, apiresponse.Unauthenticated, "用户名或密码错误")
		return
	}
	token, csrf, err := h.store.CreateSession(c.Request.Context(), adminID)
	if err != nil {
		apiresponse.Failure(c, apiresponse.InternalError, "登录暂时不可用，请稍后再试")
		return
	}
	h.clearFailures(key)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, token, 7*24*60*60, "/api/v1", "", h.secure, true)
	apiresponse.Success(c, http.StatusOK, gin.H{"csrfToken": csrf})
}

// RequireSession 创建校验会话及写操作 CSRF 令牌的中间件。
// 参数：h 是登录处理器。
// 返回：用于保护路由的 gin.HandlerFunc。
func (h *Handler) RequireSession() gin.HandlerFunc {
	return h.requireSession(true)
}

// RequireSessionWithoutCSRF 校验会话但不要求 CSRF 令牌，适用于仅结束当前会话的登出请求。
// 参数：h 是登录处理器。
// 返回：用于保护登出路由的 gin.HandlerFunc。
func (h *Handler) RequireSessionWithoutCSRF() gin.HandlerFunc {
	return h.requireSession(false)
}

func (h *Handler) requireSession(requireCSRF bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil {
			apiresponse.Abort(c, apiresponse.Unauthenticated, "请先登录")
			return
		}
		mutation := requireCSRF && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions
		adminID, err := h.store.Authenticate(c.Request.Context(), cookie, c.GetHeader("X-CSRF-Token"), mutation)
		if errors.Is(err, ErrUnauthenticated) {
			apiresponse.Abort(c, apiresponse.Unauthenticated, "请先登录")
			return
		}
		if errors.Is(err, ErrCSRF) {
			apiresponse.Abort(c, apiresponse.Forbidden, "请求校验失败，请刷新页面后重试")
			return
		}
		if err != nil {
			apiresponse.Abort(c, apiresponse.InternalError, "验证登录状态失败，请稍后再试")
			return
		}
		c.Set("adminID", adminID)
		c.Next()
	}
}

// Logout 删除当前会话并清除会话 Cookie。
// 参数：h 是登录处理器；c 是请求上下文，用于读取 Cookie 和写入响应。
// 返回：无。
// @Summary 管理员退出
// @Description 删除当前登录会话并清除会话 Cookie。
// @Tags 管理员会话
// @Produce json
// @Success 200 {object} apiresponse.Envelope "退出成功，data 为 null"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 500 {object} apiresponse.Envelope "code=50000，退出失败"
// @Router /api/v1/admin/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	token, _ := c.Cookie(sessionCookieName)
	if err := h.store.DeleteSession(c.Request.Context(), token); err != nil {
		apiresponse.Failure(c, apiresponse.InternalError, "退出登录失败，请稍后再试")
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/api/v1", "", h.secure, true)
	apiresponse.Success(c, http.StatusOK, nil)
}

// Session 返回当前已认证状态。
// 参数：h 是登录处理器；c 是请求上下文，用于写入响应。
// 返回：无。
// @Summary 查询管理员会话
// @Description 浏览器需携带登录接口设置的 yb_session Cookie。
// @Tags 管理员会话
// @Produce json
// @Success 200 {object} apiresponse.Envelope{data=map[string]bool} "当前会话已认证"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录或会话已过期"
// @Router /api/v1/admin/session [get]
func (h *Handler) Session(c *gin.Context) {
	apiresponse.Success(c, http.StatusOK, gin.H{"authenticated": true})
}

// isBlocked 判断指定登录来源是否达到失败次数限制。
// 参数：h 是登录处理器；key 是客户端 IP 与用户名组成的标识。
// 返回：是否暂时禁止该来源继续登录。
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

// recordFailure 记录指定来源的一次登录失败。
// 参数：h 是登录处理器；key 是客户端 IP 与用户名组成的标识。
// 返回：无。
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

// clearFailures 清除指定来源的登录失败记录。
// 参数：h 是登录处理器；key 是客户端 IP 与用户名组成的标识。
// 返回：无。
func (h *Handler) clearFailures(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.limits, key)
}
