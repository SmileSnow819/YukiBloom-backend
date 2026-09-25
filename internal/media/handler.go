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

// NewHandler 创建图片接口处理器。
// 参数：store 是图片存储。
// 返回：使用该存储的 Handler。
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// Upload 接收、校验并保存上传的图片。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取上传文件和写入响应。
// 返回：无。
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

// List 返回分页的图片列表。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取分页参数和写入响应。
// 返回：无。
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

// Delete 删除未被引用的图片。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取图片 ID 和写入响应。
// 返回：无。
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

// PublicFile 根据存储键向客户端提供图片文件。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取存储键和写入文件响应。
// 返回：无。
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

// parsePage 读取并校验图片列表的分页参数。
// 参数：c 是请求上下文，用于读取查询参数和报告错误。
// 返回：页码、每页数量，以及参数是否有效。
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
