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

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

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

func (h *Handler) Create(c *gin.Context) {
	var input PostInput
	if !bindPost(c, &input, false) {
		return
	}
	if err := validateInput(input); err != nil {
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

func (h *Handler) Update(c *gin.Context) {
	if !validUUID.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文章 ID 格式不正确"})
		return
	}
	var input PostInput
	if !bindPost(c, &input, true) {
		return
	}
	if err := validateInput(input); err != nil {
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

func (h *Handler) Publish(c *gin.Context) { h.setPublished(c, true) }

func (h *Handler) Unpublish(c *gin.Context) { h.setPublished(c, false) }

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

func validateInput(input PostInput) error {
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

func internalError(c *gin.Context, err error) {
	// 详细错误只写服务日志，接口不返回数据库或 SQL 信息。
	log.Printf("文章请求处理失败：%v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "服务暂时不可用，请稍后再试"})
}
