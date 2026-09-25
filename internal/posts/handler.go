package posts

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	validSlug   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	validLocale = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	validUUID   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

type Handler struct{ store *Store }

// NewHandler 创建使用文章存储的 HTTP 处理器。
// 参数：store 是文章数据存储。
// 返回：配置好的 Handler。
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// PublicList 校验筛选和分页参数，并返回已发布文章列表。
// 参数：h 是文章处理器；c 是当前 HTTP 请求上下文。
// 返回：无；文章分页结果或中文错误写入 HTTP 响应。
func (h *Handler) PublicList(c *gin.Context) {
	if locale := c.Query("locale"); locale != "" && !validLocale.MatchString(locale) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "locale 格式不正确"})
		return
	}
	if len(c.Query("category")) > 100 || len(c.Query("tag")) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分类或标签筛选条件过长"})
		return
	}
	page, limit, ok := readPage(c)
	if !ok {
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if len(query) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索内容不能超过 100 个字符"})
		return
	}
	result, err := h.store.PublicList(c.Request.Context(), c.Query("locale"), c.Query("category"), c.Query("tag"), query, page, limit)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// PublicBySlug 按语言和 slug 返回一篇已发布文章。
// 参数：h 是文章处理器；c 提供语言查询参数和文章 slug。
// 返回：无；文章内容或中文错误写入 HTTP 响应。
func (h *Handler) PublicBySlug(c *gin.Context) {
	locale := c.Query("locale")
	if !validLocale.MatchString(locale) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 locale 参数"})
		return
	}
	post, err := h.store.PublicBySlug(c.Request.Context(), locale, c.Param("slug"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, post)
}

// AdminList 返回后台使用的文章分页列表，包含草稿。
// 参数：h 是文章处理器；c 提供分页参数并接收 HTTP 响应。
// 返回：无；文章分页结果或中文错误写入 HTTP 响应。
func (h *Handler) AdminList(c *gin.Context) {
	page, limit, ok := readPage(c)
	if !ok {
		return
	}
	result, err := h.store.AdminList(c.Request.Context(), page, limit)
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// AdminByID 按文章编号读取后台编辑所需的完整记录。
// 参数：h 是文章处理器；c 提供文章编号并接收 HTTP 响应。
// 返回：无；文章记录或中文错误写入 HTTP 响应。
func (h *Handler) AdminByID(c *gin.Context) {
	if !validUUID.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章 ID 格式不正确"})
		return
	}
	post, err := h.store.AdminByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, post)
}

// Create 校验并保存一篇新的文章草稿。
// 参数：h 是文章处理器；c 包含文章 JSON 请求体并接收 HTTP 响应。
// 返回：无；新文章、冲突或中文错误写入 HTTP 响应。
func (h *Handler) Create(c *gin.Context) {
	var input PostInput
	if !bindPost(c, &input, false) {
		return
	}
	if err := ValidateInput(input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	post, err := h.store.Create(c.Request.Context(), input)
	if errors.Is(err, ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": ErrConflict.Error()})
		return
	}
	if errors.Is(err, ErrMediaNotFound) {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrMediaNotFound.Error()})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, post)
}

// Update 使用版本号保护并保存文章修改。
// 参数：h 是文章处理器；c 提供文章编号和包含 version 的 JSON 请求体。
// 返回：无；更新后的文章、冲突或中文错误写入 HTTP 响应。
func (h *Handler) Update(c *gin.Context) {
	if !validUUID.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章 ID 格式不正确"})
		return
	}
	var input PostInput
	if !bindPost(c, &input, true) {
		return
	}
	if err := ValidateInput(input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	post, err := h.store.Update(c.Request.Context(), c.Param("id"), input)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if errors.Is(err, ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": ErrConflict.Error()})
		return
	}
	if errors.Is(err, ErrMediaNotFound) {
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrMediaNotFound.Error()})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, post)
}

// Publish 将指定文章发布到公开接口。
// 参数：h 是文章处理器；c 提供文章编号并接收 HTTP 响应。
// 返回：无；发布结果或中文错误写入 HTTP 响应。
func (h *Handler) Publish(c *gin.Context) { h.setPublished(c, true) }

// Unpublish 撤回指定文章，使其不再出现在公开接口。
// 参数：h 是文章处理器；c 提供文章编号并接收 HTTP 响应。
// 返回：无；撤回结果或中文错误写入 HTTP 响应。
func (h *Handler) Unpublish(c *gin.Context) { h.setPublished(c, false) }

// setPublished 按 published 参数发布或撤回文章。
// 参数：h 是文章处理器；c 提供文章编号并接收 HTTP 响应；published 为 true 时发布，为 false 时撤回。
// 返回：无；操作结果或中文错误写入 HTTP 响应。
func (h *Handler) setPublished(c *gin.Context, published bool) {
	if !validUUID.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章 ID 格式不正确"})
		return
	}
	post, err := h.store.SetPublished(c.Request.Context(), c.Param("id"), published)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}
	if err != nil {
		internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, post)
}

// bindPost 限制请求大小并将 JSON 文章内容解码到输入结构。
// 参数：c 是当前 HTTP 请求上下文；input 接收解码后的文章；update 表示是否要求有效版本号。
// 返回：bool 表示请求体是否有效并成功解码。
func bindPost(c *gin.Context, input *PostInput, update bool) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	if err := c.ShouldBindJSON(input); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) || errors.Is(err, io.ErrUnexpectedEOF) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "文章内容不能超过 2 MiB"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求内容格式不正确"})
		}
		return false
	}
	if update && input.Version < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "修改文章时必须提供有效的 version"})
		return false
	}
	return true
}

// ValidateInput 检查文章语言、slug、标题、分类、标签和封面编号。
// 参数：input 是待创建或更新的文章内容。
// 返回：error；字段有效时返回 nil，否则返回具体的中文校验错误。
func ValidateInput(input PostInput) error {
	if !validLocale.MatchString(input.Locale) {
		return errors.New("locale 格式不正确")
	}
	if !validSlug.MatchString(input.Slug) || len(input.Slug) > 160 {
		return errors.New("slug 只能使用小写英文字母、数字和连字符，且不能超过 160 个字符")
	}
	if strings.TrimSpace(input.Title) == "" || len([]rune(input.Title)) > 300 {
		return errors.New("标题不能为空，且不能超过 300 个字符")
	}
	if len([]rune(input.Description)) > 1000 {
		return errors.New("摘要不能超过 1000 个字符")
	}
	if len(input.Categories) > 50 || len(input.Tags) > 50 {
		return errors.New("分类或标签数量不能超过 50 个")
	}
	for _, value := range append(append([]string{}, input.Categories...), input.Tags...) {
		if strings.TrimSpace(value) == "" || len([]rune(value)) > 100 {
			return errors.New("分类和标签不能为空，且不能超过 100 个字符")
		}
	}
	if input.CoverMediaID != nil && !validUUID.MatchString(*input.CoverMediaID) {
		return errors.New("封面图片 ID 格式不正确")
	}
	if len(input.Extra) > 0 {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(input.Extra, &fields); err != nil || fields == nil {
			return errors.New("extra 必须是 JSON 对象")
		}
	}
	return nil
}

// readPage 读取并校验请求中的 page 和 limit 分页参数。
// 参数：c 是包含分页查询参数的 HTTP 上下文。
// 返回：int 是页码；int 是每页数量；bool 表示两项参数是否有效。
func readPage(c *gin.Context) (int, int, bool) {
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

// internalError 记录文章处理的内部错误并返回统一错误响应。
// 参数：c 是当前 HTTP 请求上下文；err 是仅供服务日志使用的内部错误。
// 返回：无；通用错误响应写入 HTTP 响应。
func internalError(c *gin.Context, err error) {
	// 详细错误只写服务日志，接口不返回数据库或 SQL 信息。
	log.Printf("文章请求处理失败：%v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "服务暂时不可用，请稍后再试"})
}
