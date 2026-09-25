package media

import (
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/SmileSnow819/YukiBloom-backend/internal/apiresponse"
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
// @Summary 上传图片
// @Description 需要登录和 CSRF 令牌。表单字段 file 接受 JPEG、PNG 或 WebP，文件最大 10 MiB。
// @Tags 管理图片
// @Accept mpfd
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param file formData file true "图片文件"
// @Success 201 {object} apiresponse.Envelope{data=Item} "图片记录"
// @Failure 400 {object} apiresponse.Envelope "code=10001，图片格式或请求无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 413 {object} apiresponse.Envelope "code=10006，图片超过大小限制"
// @Failure 500 {object} apiresponse.Envelope "code=50000，上传失败"
// @Router /api/v1/admin/media [post]
func (h *Handler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadBytes+(64<<10))
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			apiresponse.Failure(c, apiresponse.PayloadTooLarge, ErrTooLarge.Error())
		} else {
			apiresponse.Failure(c, apiresponse.InvalidRequest, "请在 file 字段中选择一张图片")
		}
		return
	}
	defer file.Close()
	item, err := h.store.Save(c.Request.Context(), file)
	if errors.Is(err, ErrTooLarge) {
		apiresponse.Failure(c, apiresponse.PayloadTooLarge, err.Error())
		return
	}
	if errors.Is(err, ErrInvalidImage) || errors.Is(err, ErrDimensions) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return
	}
	if err != nil {
		log.Printf("图片上传失败：%v", err)
		apiresponse.Failure(c, apiresponse.InternalError, "图片上传失败，请稍后再试")
		return
	}
	apiresponse.Success(c, http.StatusCreated, item)
}

// List 返回分页的图片列表。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取分页参数和写入响应。
// 返回：无。
// @Summary 查询图片库
// @Description 需要先通过管理员登录接口登录。
// @Tags 管理图片
// @Produce json
// @Param page query int false "页码，默认 1" default(1)
// @Param limit query int false "每页数量，默认 20，最大 100" default(20)
// @Success 200 {object} apiresponse.Envelope{data=map[string]interface{}} "包含 items、page、limit、total 的分页结果"
// @Failure 400 {object} apiresponse.Envelope "code=10001，分页参数无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/admin/media [get]
func (h *Handler) List(c *gin.Context) {
	page, limit, ok := parsePage(c)
	if !ok {
		return
	}
	items, total, err := h.store.List(c.Request.Context(), page, limit)
	if err != nil {
		log.Printf("读取图片列表失败：%v", err)
		apiresponse.Failure(c, apiresponse.InternalError, "读取图片列表失败，请稍后再试")
		return
	}
	apiresponse.Success(c, http.StatusOK, gin.H{"items": items, "page": page, "limit": limit, "total": total})
}

// Delete 删除未被引用的图片。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取图片 ID 和写入响应。
// 返回：无。
// @Summary 删除未引用图片
// @Description 需要登录和 CSRF 令牌。被文章、页面或站点内容引用的图片不能删除。
// @Tags 管理图片
// @Produce json
// @Param id path string true "图片 UUID"
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Success 200 {object} apiresponse.Envelope "删除成功，data 为 null"
// @Failure 400 {object} apiresponse.Envelope "code=10001，图片 ID 格式错误"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 404 {object} apiresponse.Envelope "code=10004，图片不存在"
// @Failure 409 {object} apiresponse.Envelope "code=10005，图片仍被内容引用"
// @Failure 500 {object} apiresponse.Envelope "code=50000，删除失败"
// @Router /api/v1/admin/media/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if !mediaIDPattern.MatchString(id) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "图片 ID 格式不正确")
		return
	}
	err := h.store.DeleteUnused(c.Request.Context(), id)
	switch {
	case errors.Is(err, ErrNotFound):
		apiresponse.Failure(c, apiresponse.NotFound, ErrNotFound.Error())
	case errors.Is(err, ErrInUse):
		apiresponse.Failure(c, apiresponse.Conflict, ErrInUse.Error())
	case err != nil:
		log.Printf("删除图片失败：%v", err)
		apiresponse.Failure(c, apiresponse.InternalError, "删除图片失败，请稍后再试")
	default:
		apiresponse.Success(c, http.StatusOK, nil)
	}
}

// PublicFile 根据存储键向客户端提供图片文件。
// 参数：h 是图片接口处理器；c 是请求上下文，用于读取存储键和写入文件响应。
// 返回：无。
// @Summary 获取公开图片文件
// @Tags 图片
// @Produce application/octet-stream
// @Param key path string true "图片存储文件名"
// @Success 200 {file} file "图片文件"
// @Failure 404 {object} apiresponse.Envelope "code=10004，图片不存在"
// @Failure 500 {object} apiresponse.Envelope "code=50000，读取图片失败"
// @Router /uploads/{key} [get]
func (h *Handler) PublicFile(c *gin.Context) {
	path, mimeType, err := h.store.PublicFile(c.Request.Context(), c.Param("key"))
	if errors.Is(err, ErrNotFound) {
		apiresponse.Failure(c, apiresponse.NotFound, "图片不存在")
		return
	}
	if err != nil {
		log.Printf("读取图片信息失败：%v", err)
		apiresponse.Failure(c, apiresponse.InternalError, "读取图片失败，请稍后再试")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		apiresponse.Failure(c, apiresponse.NotFound, "图片文件不存在")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		apiresponse.Failure(c, apiresponse.InternalError, "读取图片失败，请稍后再试")
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
		apiresponse.Failure(c, apiresponse.InvalidRequest, "page 必须是有效的正整数")
		return 0, 0, false
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
	}
	if err != nil || limit < 1 || limit > 100 {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "limit 必须在 1 到 100 之间")
		return 0, 0, false
	}
	return page, limit, true
}
