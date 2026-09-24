package media

import (
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
)

var mediaIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type Handler struct{ store *Store }

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadBytes+(64<<10))
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": ErrTooLarge.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请在 file 字段中选择一张图片"})
		}
		return
	}
	defer file.Close()
	item, err := h.store.Save(c.Request.Context(), file)
	if errors.Is(err, ErrTooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, ErrInvalidImage) || errors.Is(err, ErrDimensions) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Printf("图片上传失败：%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "图片上传失败，请稍后再试"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) List(c *gin.Context) {
	page, limit, ok := parsePage(c)
	if !ok {
		return
	}
	items, total, err := h.store.List(c.Request.Context(), page, limit)
	if err != nil {
		log.Printf("读取图片列表失败：%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图片列表失败，请稍后再试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if !mediaIDPattern.MatchString(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片 ID 格式不正确"})
		return
	}
	err := h.store.DeleteUnused(c.Request.Context(), id)
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
	case errors.Is(err, ErrInUse):
		c.JSON(http.StatusConflict, gin.H{"error": ErrInUse.Error()})
	case err != nil:
		log.Printf("删除图片失败：%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除图片失败，请稍后再试"})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handler) PublicFile(c *gin.Context) {
	path, mimeType, err := h.store.PublicFile(c.Request.Context(), c.Param("key"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片不存在"})
		return
	}
	if err != nil {
		log.Printf("读取图片信息失败：%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图片失败，请稍后再试"})
		return
	}
	file, err := os.Open(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "图片文件不存在"})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取图片失败，请稍后再试"})
		return
	}
	c.Header("Content-Type", mimeType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(c.Writer, c.Request, c.Param("key"), info.ModTime(), file)
}

func parsePage(c *gin.Context) (int, int, bool) {
	page, limit := 1, 20
	var err error
	if raw := c.Query("page"); raw != "" {
		page, err = strconv.Atoi(raw)
	}
	if err != nil || page < 1 || page > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page 必须是有效的正整数"})
		return 0, 0, false
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
	}
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit 必须在 1 到 100 之间"})
		return 0, 0, false
	}
	return page, limit, true
}
