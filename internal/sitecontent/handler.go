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

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	localePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	slugPattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	pageIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

type Handler struct{ store *Store }

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

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
	features := make([]FeaturedCategory, 0, len(content.FeaturedCategories))
	for _, item := range content.FeaturedCategories {
		if item.Enabled {
			features = append(features, item)
		}
	}
	content.FeaturedCategories = features
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
	c.JSON(http.StatusOK, content)
}

func (h *Handler) AdminContent(c *gin.Context) {
	content, err := h.store.Get(c.Request.Context())
	if err != nil {
		siteContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, content)
}

func (h *Handler) ReplaceContent(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	var content Content
	if err := c.ShouldBindJSON(&content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求内容格式不正确或超过 4 MiB"})
		return
	}
	if err := ValidateContent(content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.Replace(c.Request.Context(), content); err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" {
			c.JSON(http.StatusConflict, gin.H{"error": "站点内容存在重复的标识或链接"})
			return
		}
		siteContentError(c, err)
		return
	}
	h.AdminContent(c)
}

func (h *Handler) PublicPage(c *gin.Context) {
	locale := c.Query("locale")
	if !localePattern.MatchString(locale) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供有效的 locale 参数"})
		return
	}
	page, err := h.store.PublicPage(c.Request.Context(), locale, c.Param("slug"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "页面不存在"})
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *Handler) AdminPages(c *gin.Context) {
	pages, err := h.store.AdminPages(c.Request.Context())
	if err != nil {
		siteContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": pages})
}

func (h *Handler) AdminPageByID(c *gin.Context) {
	if !pageIDPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "页面 ID 格式不正确"})
		return
	}
	page, err := h.store.AdminPageByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *Handler) SavePage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var page Page
	if err := c.ShouldBindJSON(&page); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求内容格式不正确"})
		return
	}
	isUpdate := c.Request.Method == http.MethodPatch
	if isUpdate {
		id := c.Param("id")
		if !pageIDPattern.MatchString(id) || page.ID != "" && page.ID != id {
			c.JSON(http.StatusBadRequest, gin.H{"error": "页面 ID 与请求地址不一致"})
			return
		}
		page.ID = id
	} else if page.ID != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建页面时不能填写已有页面 ID"})
		return
	}
	if err := ValidatePage(page, isUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if page.ID != "" && !pageIDPattern.MatchString(page.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "页面 ID 格式不正确"})
		return
	}
	isNew := !isUpdate
	page, err := h.store.SavePage(c.Request.Context(), page, false)
	if errors.Is(err, ErrConflict) {
		c.JSON(http.StatusConflict, gin.H{"error": ErrConflict.Error()})
		return
	}
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
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
	c.JSON(status, page)
}

func (h *Handler) PublishPage(c *gin.Context) { h.setPagePublished(c, true) }

func (h *Handler) UnpublishPage(c *gin.Context) { h.setPagePublished(c, false) }

func (h *Handler) setPagePublished(c *gin.Context, published bool) {
	if !pageIDPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "页面 ID 格式不正确"})
		return
	}
	page, err := h.store.SetPagePublished(c.Request.Context(), c.Param("id"), published)
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *Handler) DeletePage(c *gin.Context) {
	if !pageIDPattern.MatchString(c.Param("id")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "页面 ID 格式不正确"})
		return
	}
	err := h.store.DeletePage(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	if err != nil {
		siteContentError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

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

func ValidateContent(content Content) error {
	if content.Profile == nil {
		return errors.New("必须填写站点资料")
	}
	profile := content.Profile
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
	if len(content.SocialLinks) > 100 || len(content.CategoryMappings) > 200 || len(content.FeaturedCategories) > 100 || len(content.FeaturedSeries) > 100 || len(content.Navigation) > 100 || len(content.Announcements) > 100 || len(content.FriendLinks) > 1000 || len(content.Translations) > 2000 || len(content.MusicGroups) > 100 || len(content.BackgroundMusic) > 100 {
		return errors.New("站点内容条目数量超过允许上限")
	}
	for _, item := range content.SocialLinks {
		if item.Platform == "" || len(item.Platform) > 80 || !validURL(item.URL, true) {
			return errors.New("社交链接需要平台名称和有效 URL")
		}
	}
	for _, item := range content.CategoryMappings {
		if strings.TrimSpace(item.Name) == "" || !slugPattern.MatchString(item.Slug) {
			return errors.New("分类映射需要名称和有效 slug")
		}
	}
	for _, item := range content.FeaturedCategories {
		if item.Link == "" || item.Label == "" || !slugPattern.MatchString(item.Link) || !validAsset(item.Image) {
			return errors.New("精选分类需要有效链接和名称")
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
		if !localePattern.MatchString(item.Locale) || item.EntityType != "categories" && item.EntityType != "series" && item.EntityType != "featuredCategories" || item.EntityKey == "" {
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

func validLink(value string) bool {
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.Contains(value, "..") && !strings.ContainsAny(value, `\\`) {
		return true
	}
	return validURL(value, true)
}

func validAsset(value string) bool {
	if value == "" {
		return true
	}
	if strings.ContainsAny(value, `\\`) || strings.Contains(value, "..") {
		return false
	}
	if strings.HasPrefix(value, "/img/") || strings.HasPrefix(value, "/uploads/") {
		return true
	}
	return validURL(value, false)
}

func siteContentError(c *gin.Context, err error) {
	log.Printf("站点内容请求失败：%v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "站点内容暂时不可用，请稍后再试"})
}
