package importer

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/SmileSnow819/YukiBloom-backend/internal/apiresponse"
	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
	"github.com/gin-gonic/gin"
)

const maxMarkdownUpload = 2 << 20

type Handler struct{ store *posts.Store }

// NewHandler 创建 Markdown 导入处理器。
// 参数：store 是文章存储。
// 返回：使用该存储的 Handler。
func NewHandler(store *posts.Store) *Handler { return &Handler{store: store} }

// Preview 解析上传的 Markdown 并返回文章预览。
// 参数：h 是Markdown 导入处理器；c 是请求上下文，用于读取上传文件和写入响应。
// 返回：无。
// @Summary 预览 Markdown 文章
// @Description 需要登录和 CSRF 令牌。上传 Markdown 文件并解析 frontmatter，不保存文章。
// @Tags Markdown 导入
// @Accept mpfd
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param file formData file true "Markdown 文件，最大 2 MiB"
// @Param locale formData string false "语言代码，默认 zh-CN"
// @Success 200 {object} apiresponse.Envelope{data=map[string]interface{}} "文章预览及封面路径"
// @Failure 400 {object} apiresponse.Envelope "code=10001，Markdown 格式无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 413 {object} apiresponse.Envelope "code=10006，文件超过 2 MiB"
// @Router /api/v1/admin/posts/markdown/preview [post]
func (h *Handler) Preview(c *gin.Context) {
	post, ok := h.parseUpload(c)
	if !ok {
		return
	}
	apiresponse.Success(c, http.StatusOK, gin.H{"post": post.Input, "status": "draft", "coverPath": post.CoverPath})
}

// CreateDraft 将上传的 Markdown 解析并保存为草稿。
// 参数：h 是Markdown 导入处理器；c 是请求上下文，用于读取表单和写入响应。
// 返回：无。
// @Summary 上传 Markdown 并保存为草稿
// @Description 需要登录和 CSRF 令牌。若 Markdown 含封面，先上传图片并传入 coverMediaId。
// @Tags Markdown 导入
// @Accept mpfd
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param file formData file true "Markdown 文件，最大 2 MiB"
// @Param locale formData string false "语言代码，默认 zh-CN"
// @Param coverMediaId formData string false "已上传封面图片的 ID"
// @Success 201 {object} apiresponse.Envelope{data=posts.Post} "新建文章草稿"
// @Failure 400 {object} apiresponse.Envelope "code=10001，Markdown 或封面信息无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 409 {object} apiresponse.Envelope "code=10005，文章链接标识已被占用"
// @Failure 413 {object} apiresponse.Envelope "code=10006，文件超过 2 MiB"
// @Router /api/v1/admin/posts/markdown [post]
func (h *Handler) CreateDraft(c *gin.Context) {
	post, ok := h.parseUpload(c)
	if !ok {
		return
	}
	coverMediaID := strings.TrimSpace(c.PostForm("coverMediaId"))
	if post.CoverPath != "" && coverMediaID == "" {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "文章包含封面，请先上传封面图片，并通过 coverMediaId 字段关联")
		return
	}
	if coverMediaID != "" {
		post.Input.CoverMediaID = &coverMediaID
	}
	if err := posts.ValidateInput(post.Input); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return
	}
	created, err := h.store.Create(c.Request.Context(), post.Input)
	if errors.Is(err, posts.ErrConflict) {
		apiresponse.Failure(c, apiresponse.Conflict, posts.ErrConflict.Error())
		return
	}
	if errors.Is(err, posts.ErrMediaNotFound) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "选择的封面图片不存在")
		return
	}
	if err != nil {
		log.Printf("保存 Markdown 草稿失败：%v", err)
		apiresponse.Failure(c, apiresponse.InternalError, "保存草稿失败，请稍后再试")
		return
	}
	apiresponse.Success(c, http.StatusCreated, created)
}

// parseUpload 读取、解析并校验上传的 Markdown 文件。
// 参数：h 是Markdown 导入处理器；c 是请求上下文，用于读取上传文件和报告错误。
// 返回：解析后的 MarkdownPost，以及上传是否有效。
func (h *Handler) parseUpload(c *gin.Context) (MarkdownPost, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMarkdownUpload+(64<<10))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			apiresponse.Failure(c, apiresponse.PayloadTooLarge, "Markdown 文件不能超过 2 MiB")
		} else {
			apiresponse.Failure(c, apiresponse.InvalidRequest, "请在 file 字段中选择 Markdown 文件")
		}
		return MarkdownPost{}, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxMarkdownUpload+1))
	if err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "读取 Markdown 文件失败")
		return MarkdownPost{}, false
	}
	if len(data) > maxMarkdownUpload {
		apiresponse.Failure(c, apiresponse.PayloadTooLarge, "Markdown 文件不能超过 2 MiB")
		return MarkdownPost{}, false
	}
	locale := c.PostForm("locale")
	if locale == "" {
		locale = "zh-CN"
	}
	post, err := ParseMarkdown(header.Filename, locale, data)
	if err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return MarkdownPost{}, false
	}
	if err := posts.ValidateInput(post.Input); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return MarkdownPost{}, false
	}
	return post, true
}
