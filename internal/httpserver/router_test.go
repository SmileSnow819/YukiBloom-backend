package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SmileSnow819/YukiBloom-backend/internal/apiresponse"
	"github.com/gin-gonic/gin"
)

func TestHealthReturnsOK(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	NewRouter(nil, nil, nil, nil, nil, nil).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 0 || body.Message != "成功" || body.Data.Status != "正常" {
		t.Fatalf("健康检查响应结构错误：code=%d message=%q status=%q", body.Code, body.Message, body.Data.Status)
	}
}

func TestUnknownRouteReturnsNotFound(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter(nil, nil, nil, nil, nil, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != apiresponse.CodeNotFound || body.Message == "" {
		t.Fatalf("未知路由应返回统一错误结构，实际 code=%d message=%q", body.Code, body.Message)
	}
}

func TestClientIPIgnoresUntrustedForwardedHeader(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil)
	router.GET("/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})
	request := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.123")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "192.0.2.10" {
		t.Fatalf("客户端地址不能由转发请求头伪造：状态 %d，地址 %q", response.Code, response.Body.String())
	}
}
