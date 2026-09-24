package importer

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMarkdownCoverMustBeAssociatedBeforeSaving(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "article.md")
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write([]byte("---\ntitle: 测试文章\ndate: 2026-09-24\ncover: /img/cover/example.jpg\n---\n正文"))
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/markdown", NewHandler(nil).CreateDraft)
	request := httptest.NewRequest(http.MethodPost, "/markdown", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "封面") {
		t.Fatalf("未关联封面的文章应得到明确提示，状态 %d，响应 %s", response.Code, response.Body.String())
	}
}
