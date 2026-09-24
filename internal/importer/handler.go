package importer

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
	"github.com/gin-gonic/gin"
)

const maxMarkdownUpload = 2 << 20

type Handler struct{ store *posts.Store }

func NewHandler(store *posts.Store) *Handler { return &Handler{store: store} }

func (h *Handler) Preview(c *gin.Context) {
	post, ok := h.parseUpload(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"post": post.Input, "status": "draft", "coverPath": post.CoverPath})
}

func (h *Handler) CreateDraft(c *gin.Context) {
	post, ok := h.parseUpload(c)
	if !ok {
		return
	}
	coverMediaID := strings.TrimSpace(c.PostForm("coverMediaId"))
	if post.CoverPath != "" && coverMediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章包含封面，请先上传封面图片，并通过 coverMediaId 字段关联"})
		return
	}
	if coverMediaID != "" {
		post.Input.CoverMediaID = &coverMediaID
	}
	if err := posts.ValidateInput(post.Input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	created, err := h.store.Create(c.Request.Context(), post.Input)
	if errors.Is(err, posts.ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": posts.ErrConflict.Error()})
		return
	}
	if errors.Is(err, posts.ErrMediaNotFound) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "选择的封面图片不存在"})
		return
	}
	if err != nil {
		log.Printf("保存 Markdown 草稿失败：%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存草稿失败，请稍后再试"})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) parseUpload(c *gin.Context) (MarkdownPost, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMarkdownUpload+(64<<10))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Markdown 文件不能超过 2 MiB"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请在 file 字段中选择 Markdown 文件"})
		}
		return MarkdownPost{}, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMarkdownUpload+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取 Markdown 文件失败"})
		return MarkdownPost{}, false
	}
	if len(data) > maxMarkdownUpload {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Markdown 文件不能超过 2 MiB"})
		return MarkdownPost{}, false
	}
	locale := c.PostForm("locale")
	if locale == "" {
		locale = "zh-CN"
	}
	post, err := ParseMarkdown(header.Filename, locale, data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return MarkdownPost{}, false
	}
	if err := posts.ValidateInput(post.Input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return MarkdownPost{}, false
	}
	return post, true
}
