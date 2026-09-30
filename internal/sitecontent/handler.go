package sitecontent

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/SmileSnow819/YukiBloom-backend/internal/apiresponse"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	localePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	slugPattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	pageIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

type Handler struct{ store *Store }

// 创建站点内容 HTTP 处理器。
// 参数：store 是站点内容数据存储。
// 返回：*Handler 是处理公开内容与管理端页面请求的处理器。
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// 读取公开站点内容，并过滤未启用或尚未生效的条目。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文。
// 返回：无；结果通过 HTTP JSON 响应返回。
// @Summary 查询公开站点内容
// @Description 返回公开站点资料、分类、精选系列、导航、公告、已审核友链及音乐信息。
// @Tags 站点内容
// @Produce json
// @Success 200 {object} apiresponse.Envelope{data=Content} "公开站点内容"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/site-content [get]
func (h *Handler) PublicContent(c *gin.Context) {
	content, err := h.store.Get(c.Request.Context())
	if err != nil {
		siteContentError(c, err)
		return
	}
	now := time.Now()
	announcements := make([]Announcement, 0, len(content.Announcements))
	for _, item := range content.Announcements {
		if !item.Enabled || (item.StartDate != nil && now.Before(*item.StartDate)) || (item.EndDate != nil && now.After(*item.EndDate)) {
			continue
		}
		if item.PublishDate != "" && item.PublishDate > now.Format("2006-01-02") {
			continue
		}
		announcements = append(announcements, item)
	}
	content.Announcements = announcements
	friends := make([]FriendLink, 0, len(content.FriendLinks))
	for _, item := range content.FriendLinks {
		if item.Status == "approved" {
			friends = append(friends, item)
		}
	}
	content.FriendLinks = friends
	series := make([]FeaturedSeries, 0, len(content.FeaturedSeries))
	for _, item := range content.FeaturedSeries {
		if item.Enabled {
			series = append(series, item)
		}
	}
	content.FeaturedSeries = series
	social := make([]SocialLink, 0, len(content.SocialLinks))
	for _, item := range content.SocialLinks {
		if item.Enabled {
			social = append(social, item)
		}
	}
	content.SocialLinks = social
	music := make([]MusicGroup, 0, len(content.MusicGroups))
	for _, item := range content.MusicGroups {
		if item.Enabled {
			music = append(music, item)
		}
	}
	content.MusicGroups = music
	background := make([]BackgroundTrack, 0, len(content.BackgroundMusic))
	for _, item := range content.BackgroundMusic {
		if item.Enabled {
			background = append(background, item)
		}
	}
	content.BackgroundMusic = background
	apiresponse.Success(c, http.StatusOK, content)
}

// 读取包含管理字段的完整站点内容。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文。
// 返回：无；结果通过 HTTP JSON 响应返回。
// @Summary 查询后台站点内容
// @Description 需要先通过管理员登录接口登录；响应包含未公开条目和 version。
// @Tags 管理站点内容
// @Produce json
// @Success 200 {object} apiresponse.Envelope{data=Content} "完整站点内容"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/admin/site-content [get]
func (h *Handler) AdminContent(c *gin.Context) {
	content, err := h.store.Get(c.Request.Context())
	if err != nil {
		siteContentError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, content)
}

// 校验并整体替换站点内容，处理版本冲突和重复标识。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文及 JSON 请求体。
// 返回：无；操作结果通过 HTTP 状态码和 JSON 响应返回。
// @Summary 整体保存站点内容
// @Description 需要登录和 CSRF 令牌。必须提交查询时取得的 version，旧版本会返回冲突。
// @Tags 管理站点内容
// @Accept json
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param content body Content true "完整站点内容及当前 version"
// @Success 200 {object} apiresponse.Envelope{data=Content} "保存后的站点内容"
// @Failure 400 {object} apiresponse.Envelope "code=10001，站点内容无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 409 {object} apiresponse.Envelope "code=10005，版本冲突或内容标识重复"
// @Failure 500 {object} apiresponse.Envelope "code=50000，保存失败"
// @Router /api/v1/admin/site-content [put]
func (h *Handler) ReplaceContent(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	var content Content
	if err := c.ShouldBindJSON(&content); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "请求内容格式不正确或超过 4 MiB")
		return
	}
	// Preserve the existing profile when an older or narrower admin form omits it.
	if content.Profile == nil {
		current, err := h.store.Get(c.Request.Context())
		if err != nil {
			siteContentError(c, err)
			return
		}
		content.Profile = current.Profile
	}
	if err := validateContent(content, false); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return
	}
	if err := h.store.Replace(c.Request.Context(), content); err != nil {
		if errors.Is(err, ErrConflict) {
			apiresponse.Failure(c, apiresponse.Conflict, "站点内容已被其他操作修改，请刷新后重试")
			return
		}
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" {
			apiresponse.Failure(c, apiresponse.Conflict, "站点内容存在重复的标识或链接")
			return
		}
		siteContentError(c, err)
		return
	}
	h.AdminContent(c)
}

// 按语言和链接标识返回已发布的独立页面。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文，包含 locale 查询参数和 slug 路径参数。
// 返回：无；页面或错误通过 HTTP JSON 响应返回。
// @Summary 查询公开独立页面
// @Tags 页面
// @Produce json
// @Param slug path string true "页面链接标识"
// @Param locale query string true "语言代码，例如 zh-CN"
// @Success 200 {object} apiresponse.Envelope{data=Page} "页面内容"
// @Failure 400 {object} apiresponse.Envelope "code=10001，语言代码无效"
// @Failure 404 {object} apiresponse.Envelope "code=10004，页面不存在或尚未发布"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/pages/{slug} [get]
func (h *Handler) PublicPage(c *gin.Context) {
	locale := c.Query("locale")
	if !localePattern.MatchString(locale) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "请提供有效的 locale 参数")
		return
	}
	page, err := h.store.PublicPage(c.Request.Context(), locale, c.Param("slug"))
	if errors.Is(err, ErrNotFound) {
		apiresponse.Failure(c, apiresponse.NotFound, "页面不存在")
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, page)
}

// 返回管理端可见的全部独立页面。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文。
// 返回：无；页面列表通过 HTTP JSON 响应返回。
// @Summary 查询后台页面列表
// @Description 需要先通过管理员登录接口登录，结果包含草稿和已发布页面。
// @Tags 管理页面
// @Produce json
// @Success 200 {object} apiresponse.Envelope{data=map[string]interface{}} "页面列表"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/admin/pages [get]
func (h *Handler) AdminPages(c *gin.Context) {
	pages, err := h.store.AdminPages(c.Request.Context())
	if err != nil {
		siteContentError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, gin.H{"items": pages})
}

// 按 ID 返回管理端页面详情。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文，包含页面 ID 路径参数。
// 返回：无；页面或错误通过 HTTP JSON 响应返回。
// @Summary 查询后台页面详情
// @Description 需要先通过管理员登录接口登录。
// @Tags 管理页面
// @Produce json
// @Param id path string true "页面 UUID"
// @Success 200 {object} apiresponse.Envelope{data=Page} "页面详情"
// @Failure 400 {object} apiresponse.Envelope "code=10001，页面 ID 格式错误"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 404 {object} apiresponse.Envelope "code=10004，页面不存在"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/admin/pages/{id} [get]
func (h *Handler) AdminPageByID(c *gin.Context) {
	if !pageIDPattern.MatchString(c.Param("id")) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "页面 ID 格式不正确")
		return
	}
	page, err := h.store.AdminPageByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiresponse.Failure(c, apiresponse.NotFound, ErrNotFound.Error())
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, page)
}

// 创建或更新独立页面草稿，并校验请求中的版本和标识。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文及页面 JSON 请求体。
// 返回：无；保存后的页面或错误通过 HTTP 响应返回。
// @Summary 创建页面草稿
// @Description 需要登录和 CSRF 令牌。
// @Tags 管理页面
// @Accept json
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param page body Page true "页面内容"
// @Success 201 {object} apiresponse.Envelope{data=Page} "新建页面"
// @Failure 400 {object} apiresponse.Envelope "code=10001，页面内容无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 409 {object} apiresponse.Envelope "code=10005，语言和链接标识已被占用"
// @Failure 500 {object} apiresponse.Envelope "code=50000，保存失败"
// @Router /api/v1/admin/pages [post]
func (h *Handler) SavePage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var page Page
	if err := c.ShouldBindJSON(&page); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "请求内容格式不正确")
		return
	}
	isUpdate := c.Request.Method == http.MethodPatch
	if isUpdate {
		id := c.Param("id")
		if !pageIDPattern.MatchString(id) || page.ID != "" && page.ID != id {
			apiresponse.Failure(c, apiresponse.InvalidRequest, "页面 ID 与请求地址不一致")
			return
		}
		page.ID = id
	} else if page.ID != "" {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "创建页面时不能填写已有页面 ID")
		return
	}
	if err := ValidatePage(page, isUpdate); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return
	}
	if page.ID != "" && !pageIDPattern.MatchString(page.ID) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "页面 ID 格式不正确")
		return
	}
	isNew := !isUpdate
	page, err := h.store.SavePage(c.Request.Context(), page, false)
	if errors.Is(err, ErrConflict) {
		apiresponse.Failure(c, apiresponse.Conflict, ErrConflict.Error())
		return
	}
	if errors.Is(err, ErrNotFound) {
		apiresponse.Failure(c, apiresponse.NotFound, ErrNotFound.Error())
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}
	apiresponse.Success(c, status, page)
}

// UpdatePage 处理独立页面的局部更新请求。
// 参数：h 是站点内容处理器；c 是包含页面 ID 和 JSON 请求体的 HTTP 上下文。
// 返回：无；更新后的页面或中文错误写入 HTTP 响应。
// @Summary 更新页面草稿
// @Description 需要登录和 CSRF 令牌。请求中的 version 必须与查询页面时一致，否则返回冲突。
// @Tags 管理页面
// @Accept json
// @Produce json
// @Param id path string true "页面 UUID"
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param page body Page true "页面新内容及当前 version"
// @Success 200 {object} apiresponse.Envelope{data=Page} "更新后的页面"
// @Failure 400 {object} apiresponse.Envelope "code=10001，页面内容、ID 或版本无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 404 {object} apiresponse.Envelope "code=10004，页面不存在"
// @Failure 409 {object} apiresponse.Envelope "code=10005，页面版本或链接标识冲突"
// @Failure 500 {object} apiresponse.Envelope "code=50000，保存失败"
// @Router /api/v1/admin/pages/{id} [patch]
func (h *Handler) UpdatePage(c *gin.Context) { h.SavePage(c) }

// 将指定页面发布。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文，包含页面 ID。
// 返回：无；更新后的页面或错误通过 HTTP 响应返回。
// @Summary 发布页面
// @Description 需要登录和 CSRF 令牌。
// @Tags 管理页面
// @Produce json
// @Param id path string true "页面 UUID"
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Success 200 {object} apiresponse.Envelope{data=Page} "已发布页面"
// @Failure 400 {object} apiresponse.Envelope "code=10001，页面 ID 格式错误"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 404 {object} apiresponse.Envelope "code=10004，页面不存在"
// @Failure 500 {object} apiresponse.Envelope "code=50000，发布失败"
// @Router /api/v1/admin/pages/{id}/publish [post]
func (h *Handler) PublishPage(c *gin.Context) { h.setPagePublished(c, true) }

// 将指定页面取消发布并恢复为草稿。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文，包含页面 ID。
// 返回：无；更新后的页面或错误通过 HTTP 响应返回。
// @Summary 撤回页面
// @Description 需要登录和 CSRF 令牌。撤回后页面不再出现在公开接口。
// @Tags 管理页面
// @Produce json
// @Param id path string true "页面 UUID"
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Success 200 {object} apiresponse.Envelope{data=Page} "已撤回页面"
// @Failure 400 {object} apiresponse.Envelope "code=10001，页面 ID 格式错误"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 404 {object} apiresponse.Envelope "code=10004，页面不存在"
// @Failure 500 {object} apiresponse.Envelope "code=50000，撤回失败"
// @Router /api/v1/admin/pages/{id}/unpublish [post]
func (h *Handler) UnpublishPage(c *gin.Context) { h.setPagePublished(c, false) }

// 设置页面的发布状态。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文；published 表示是否发布。
// 返回：无；更新后的页面或错误通过 HTTP 响应返回。
func (h *Handler) setPagePublished(c *gin.Context, published bool) {
	if !pageIDPattern.MatchString(c.Param("id")) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "页面 ID 格式不正确")
		return
	}
	page, err := h.store.SetPagePublished(c.Request.Context(), c.Param("id"), published)
	if errors.Is(err, ErrNotFound) {
		apiresponse.Failure(c, apiresponse.NotFound, ErrNotFound.Error())
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, page)
}

// 删除指定独立页面。
// 参数：h 是站点内容处理器；c 是当前 HTTP 请求上下文，包含页面 ID。
// 返回：无；结果通过 HTTP 状态码返回。
// @Summary 删除页面
// @Description 需要登录和 CSRF 令牌。
// @Tags 管理页面
// @Produce json
// @Param id path string true "页面 UUID"
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Success 200 {object} apiresponse.Envelope "删除成功，data 为 null"
// @Failure 400 {object} apiresponse.Envelope "code=10001，页面 ID 格式错误"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 404 {object} apiresponse.Envelope "code=10004，页面不存在"
// @Failure 500 {object} apiresponse.Envelope "code=50000，删除失败"
// @Router /api/v1/admin/pages/{id} [delete]
func (h *Handler) DeletePage(c *gin.Context) {
	if !pageIDPattern.MatchString(c.Param("id")) {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "页面 ID 格式不正确")
		return
	}
	err := h.store.DeletePage(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		apiresponse.Failure(c, apiresponse.NotFound, ErrNotFound.Error())
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, nil)
}

// 校验独立页面的语言、标识、标题、正文长度和更新版本。
// 参数：page 是待校验页面；update 表示是否为更新操作。
// 返回：error 为 nil 表示有效，非 nil 时说明字段不符合要求。
func ValidatePage(page Page, update bool) error {
	if !localePattern.MatchString(page.Locale) || !slugPattern.MatchString(page.Slug) {
		return errors.New("页面需要有效的语言代码和 slug")
	}
	if strings.TrimSpace(page.Title) == "" || len([]rune(page.Title)) > 300 {
		return errors.New("页面标题不能为空，且不能超过 300 个字符")
	}
	if len(page.Description) > 4000 || len(page.BodyMarkdown) > 2<<20 {
		return errors.New("页面摘要或正文超过允许长度")
	}
	if update && page.Version < 1 {
		return errors.New("修改页面时必须提供有效的 version")
	}
	return nil
}

// 校验站点资料及各类公开内容字段。
// 参数：content 是待校验的完整站点内容。
// 返回：error 为 nil 表示有效，非 nil 时说明具体校验失败原因。
func ValidateContent(content Content) error {
	return validateContent(content, true)
}

func validateContent(content Content, requireProfile bool) error {
	if content.Profile == nil && requireProfile {
		return errors.New("必须填写站点资料")
	}
	if profile := content.Profile; profile != nil {
		if strings.TrimSpace(profile.Title) == "" || strings.TrimSpace(profile.Name) == "" || profile.StartYear < 1900 || profile.StartYear > 2200 || !validURL(profile.URL, false) {
			return errors.New("站点名称、作者、建站年份或站点 URL 不正确")
		}
		if profile.Timezone == "" {
			return errors.New("站点时区不能为空")
		}
		if _, err := time.LoadLocation(profile.Timezone); err != nil {
			return errors.New("站点时区不是有效的时区名称")
		}
		if !validAsset(profile.Avatar) || !validAsset(profile.DefaultOGImage) {
			return errors.New("站点头像或默认分享图片地址不正确")
		}
	}
	if len(content.SocialLinks) > 100 || len(content.Categories) > 200 || len(content.FeaturedSeries) > 100 || len(content.Navigation) > 100 || len(content.Announcements) > 100 || len(content.FriendLinks) > 1000 || len(content.Translations) > 2000 || len(content.MusicGroups) > 100 || len(content.BackgroundMusic) > 100 {
		return errors.New("站点内容条目数量超过允许上限")
	}
	for _, item := range content.SocialLinks {
		if item.Platform == "" || len(item.Platform) > 80 || !validURL(item.URL, true) {
			return errors.New("社交链接需要平台名称和有效 URL")
		}
	}
	categoryNames := make(map[string]struct{}, len(content.Categories))
	categorySlugs := make(map[string]struct{}, len(content.Categories))
	categorySortOrders := make(map[int]struct{}, len(content.Categories))
	for _, item := range content.Categories {
		name := strings.TrimSpace(item.Name)
		if name == "" || !slugPattern.MatchString(item.Slug) {
			return errors.New("分类需要名称和有效链接标识")
		}
		if _, exists := categoryNames[name]; exists {
			return errors.New("分类名称不能重复")
		}
		if _, exists := categorySlugs[item.Slug]; exists {
			return errors.New("分类链接标识不能重复")
		}
		if item.SortOrder < 0 {
			return errors.New("分类顺序不能小于 0")
		}
		if _, exists := categorySortOrders[item.SortOrder]; exists {
			return errors.New("分类顺序不能重复")
		}
		categoryNames[name] = struct{}{}
		categorySlugs[item.Slug] = struct{}{}
		categorySortOrders[item.SortOrder] = struct{}{}
		if !validAsset(item.Image) || item.ShowOnHome && item.Image == "" {
			return errors.New("首页展示的分类需要有效封面图片")
		}
	}
	for _, item := range content.FeaturedSeries {
		if !slugPattern.MatchString(item.Slug) || item.CategoryName == "" || !validAsset(item.Cover) {
			return errors.New("精选系列需要有效 slug 和分类名称")
		}
	}
	for _, item := range content.Navigation {
		if err := validateNavigation(item, 0); err != nil {
			return err
		}
	}
	for _, item := range content.Announcements {
		if item.ID == "" || item.Title == "" || item.Type != "info" && item.Type != "success" && item.Type != "warning" && item.Type != "error" {
			return errors.New("公告需要 ID、标题和有效类型")
		}
		if item.Link.URL != "" && !validLink(item.Link.URL) {
			return errors.New("公告链接 URL 不正确")
		}
		if item.PublishDate != "" {
			if _, err := time.Parse("2006-01-02", item.PublishDate); err != nil {
				return errors.New("公告发布日期格式应为 YYYY-MM-DD")
			}
		}
		if item.StartDate != nil && item.EndDate != nil && item.EndDate.Before(*item.StartDate) {
			return errors.New("公告结束时间不能早于开始时间")
		}
	}
	for _, item := range content.FriendLinks {
		if item.Site == "" || !validURL(item.URL, false) || !validAsset(item.Image) || item.Status != "pending" && item.Status != "approved" && item.Status != "rejected" {
			return errors.New("友链需要名称、有效网址和审核状态")
		}
	}
	for _, item := range content.Translations {
		if !localePattern.MatchString(item.Locale) || item.EntityType != "categories" && item.EntityType != "series" || item.EntityKey == "" {
			return errors.New("内容翻译的语言、类型或标识不正确")
		}
	}
	for _, group := range content.MusicGroups {
		if group.Title == "" || len(group.Links) > 100 {
			return errors.New("歌单分组需要标题，且链接数量不能超过 100 个")
		}
		for _, link := range group.Links {
			if !validURL(link.URL, false) {
				return errors.New("歌单链接 URL 不正确")
			}
		}
	}
	for _, item := range content.BackgroundMusic {
		if item.Title == "" || !validURL(item.URL, false) {
			return errors.New("背景音乐需要标题和有效 URL")
		}
	}
	return nil
}

// 递归校验导航项目、链接及嵌套深度。
// 参数：item 是当前导航项目；depth 是从根节点开始的嵌套层数。
// 返回：error 为 nil 表示有效，非 nil 时说明导航字段不符合要求。
func validateNavigation(item NavigationItem, depth int) error {
	if depth > 4 || strings.TrimSpace(item.Name) == "" || len(item.Children) > 100 {
		return errors.New("导航菜单名称无效或嵌套层级过深")
	}
	if item.Path != "" && !(strings.HasPrefix(item.Path, "/") && !strings.HasPrefix(item.Path, "//") && !strings.Contains(item.Path, `\\`)) && !validURL(item.Path, false) {
		return errors.New("导航链接只能使用站内路径或 HTTP/HTTPS 地址")
	}
	if item.ID != "" && !pageIDPattern.MatchString(item.ID) {
		return errors.New("导航项目 ID 格式不正确")
	}
	for _, child := range item.Children {
		if err := validateNavigation(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// 判断字符串是否为允许的 HTTP、HTTPS 或可选 mailto 地址。
// 参数：value 是待检查的地址；allowMailto 表示是否接受 mailto 协议。
// 返回：bool 表示地址是否符合允许的格式。
func validURL(value string, allowMailto bool) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	if allowMailto && parsed.Scheme == "mailto" {
		return parsed.Opaque != "" || parsed.Path != ""
	}
	return parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}

// 判断链接是否为安全的站内路径或外部网址。
// 参数：value 是待检查的链接。
// 返回：bool 表示链接是否有效。
func validLink(value string) bool {
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.Contains(value, "..") && !strings.ContainsAny(value, `\\`) {
		return true
	}
	return validURL(value, true)
}

// 判断图片资源是否为空、站内上传地址或有效外部网址。
// 参数：value 是待检查的资源地址。
// 返回：bool 表示资源地址是否有效。
func validAsset(value string) bool {
	if value == "" {
		return true
	}
	if strings.ContainsAny(value, `\\`) || strings.Contains(value, "..") {
		return false
	}
	if strings.HasPrefix(value, "/img/") {
		return true
	}
	return validURL(value, false)
}

// 记录站点内容请求错误并返回统一的服务器错误响应。
// 参数：c 是当前 HTTP 请求上下文；err 是需要记录的内部错误。
// 返回：无；通过 HTTP 500 JSON 响应返回错误信息。
func siteContentError(c *gin.Context, err error) {
	log.Printf("站点内容请求失败：%v", err)
	apiresponse.Failure(c, apiresponse.InternalError, "站点内容暂时不可用，请稍后再试")
}
