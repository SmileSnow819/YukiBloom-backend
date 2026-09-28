package posts

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDeleteRejectsInvalidPostID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/posts/:id", NewHandler(nil).Delete)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/posts/not-a-uuid", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("无效文章 ID 应返回 400，实际为 %d", response.Code)
	}
}
