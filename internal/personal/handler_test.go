package personal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
	"github.com/gin-gonic/gin"
)

func TestPersonalReplaceHTTPVersionResponses(t *testing.T) {
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
	beforeFootprints, err := store.Footprints(ctx)
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
		_ = store.ReplaceAll(cleanupCtx, beforeFootprints, beforeTimeline)
	})
	handler := NewHandler(store)
	router := gin.New()
	router.GET("/footprints", handler.GetFootprints)
	router.PUT("/footprints", handler.ReplaceFootprints)
	router.GET("/timeline", handler.GetTimeline)
	router.PUT("/timeline", handler.ReplaceTimeline)
	for _, path := range []string{"/footprints", "/timeline"} {
		get := httptest.NewRecorder()
		router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, path, nil))
		if get.Code != http.StatusOK {
			t.Fatalf("读取 %s 失败：%d", path, get.Code)
		}
		var body struct {
			Data struct {
				Version int64 `json:"version"`
			} `json:"data"`
		}
		if err := json.Unmarshal(get.Body.Bytes(), &body); err != nil || body.Data.Version < 1 {
			t.Fatalf("%s 缺少有效版本号：%v", path, err)
		}
		invalid := putPersonal(router, path, `{"items":[]}`)
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("%s 缺少版本号应返回 400，实际 %d", path, invalid.Code)
		}
		payload := `{"version":` + strconv.FormatInt(body.Data.Version, 10) + `,"items":[]}`
		if path == "/footprints" {
			payload = `{"version":` + strconv.FormatInt(body.Data.Version, 10) + `,"locations":[],"stays":[],"routes":[]}`
		}
		if response := putPersonal(router, path, payload); response.Code != http.StatusOK {
			t.Fatalf("%s 当前版本保存应成功，实际 %d：%s", path, response.Code, response.Body.String())
		}
		if response := putPersonal(router, path, payload); response.Code != http.StatusConflict {
			t.Fatalf("%s 旧版本保存应返回 409，实际 %d", path, response.Code)
		}
	}
}

// putPersonal 发送个人内容整体替换请求并返回记录的响应。
// 参数：router 是测试路由；path 是请求路径；body 是 JSON 请求体。
// 返回：*httptest.ResponseRecorder 是 HTTP 响应记录。
func putPersonal(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}
