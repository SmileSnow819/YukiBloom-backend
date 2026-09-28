package personal

import (
	"errors"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/SmileSnow819/YukiBloom-backend/internal/apiresponse"
	"github.com/gin-gonic/gin"
)

var (
	datePattern       = regexp.MustCompile(`^\d{4}\.(?:0[1-9]|1[0-2])(?:\.(?:0[1-9]|[12]\d|3[01]))?$`)
	itemIDPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	locationIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)
)

type Handler struct{ store *Store }

// NewHandler 创建使用足迹与时间线存储的 HTTP 处理器。
// 参数：store 是个人内容的数据存储。
// 返回：配置好的 Handler。
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// GetFootprints 查询公开足迹并返回地点、停留和路线数据。
// 参数：h 是个人内容处理器；c 是当前 HTTP 请求上下文。
// 返回：无；查询结果或中文错误写入 HTTP 响应。
// @Summary 查询公开足迹
// @Tags 个人内容
// @Produce json
// @Success 200 {object} apiresponse.Envelope{data=Footprints} "地点、停留记录和路线"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/footprints [get]
func (h *Handler) GetFootprints(c *gin.Context) {
	data, err := h.store.Footprints(c.Request.Context())
	if err != nil {
		personalError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, data)
}

// ReplaceFootprints 校验管理员提交的足迹并整体保存。
// 参数：h 是个人内容处理器；c 是包含 JSON 请求体的 HTTP 上下文。
// 返回：无；保存后的内容或中文错误写入 HTTP 响应。
// @Summary 整体保存足迹
// @Description 需要登录和 CSRF 令牌；请求会替换全部地点、停留记录和路线。
// @Tags 管理个人内容
// @Accept json
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param footprints body Footprints true "完整足迹数据"
// @Success 200 {object} apiresponse.Envelope{data=Footprints} "保存后的足迹数据"
// @Failure 400 {object} apiresponse.Envelope "code=10001，足迹数据无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 409 {object} apiresponse.Envelope "code=10005，足迹版本冲突"
// @Failure 500 {object} apiresponse.Envelope "code=50000，保存失败"
// @Router /api/v1/admin/footprints [put]
func (h *Handler) ReplaceFootprints(c *gin.Context) {
	var data Footprints
	if !bindContent(c, &data) {
		return
	}
	if err := ValidateFootprints(data); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return
	}
	if data.Version < 1 {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "保存足迹时必须提供有效的 version")
		return
	}
	if err := h.store.ReplaceFootprints(c.Request.Context(), data); err != nil {
		if errors.Is(err, ErrConflict) {
			apiresponse.Failure(c, apiresponse.Conflict, ErrConflict.Error())
			return
		}
		personalError(c, err)
		return
	}
	h.GetFootprints(c)
}

// GetTimeline 查询并返回公开的实习经历时间线。
// 参数：h 是个人内容处理器；c 是当前 HTTP 请求上下文。
// 返回：无；时间线或中文错误写入 HTTP 响应。
// @Summary 查询公开实习经历
// @Tags 个人内容
// @Produce json
// @Success 200 {object} apiresponse.Envelope{data=TimelineInput} "包含 version 和 items 的时间线"
// @Failure 500 {object} apiresponse.Envelope "code=50000，查询失败"
// @Router /api/v1/timeline [get]
func (h *Handler) GetTimeline(c *gin.Context) {
	items, version, err := h.store.Timeline(c.Request.Context())
	if err != nil {
		personalError(c, err)
		return
	}
	apiresponse.Success(c, http.StatusOK, TimelineInput{Version: version, Items: items})
}

// ReplaceTimeline 校验管理员提交的经历列表并整体保存。
// 参数：h 是个人内容处理器；c 是包含 JSON 请求体的 HTTP 上下文。
// 返回：无；保存后的时间线或中文错误写入 HTTP 响应。
// @Summary 整体保存实习经历
// @Description 需要登录和 CSRF 令牌；请求会替换全部经历条目。
// @Tags 管理个人内容
// @Accept json
// @Produce json
// @Param X-CSRF-Token header string true "登录接口返回的 csrfToken"
// @Param timeline body TimelineInput true "包含 items 实习经历数组"
// @Success 200 {object} apiresponse.Envelope{data=map[string]interface{}} "保存后的时间线"
// @Failure 400 {object} apiresponse.Envelope "code=10001，时间线数据无效"
// @Failure 401 {object} apiresponse.Envelope "code=10002，尚未登录"
// @Failure 403 {object} apiresponse.Envelope "code=10003，CSRF 校验失败"
// @Failure 409 {object} apiresponse.Envelope "code=10005，实习经历版本冲突"
// @Failure 500 {object} apiresponse.Envelope "code=50000，保存失败"
// @Router /api/v1/admin/timeline [put]
func (h *Handler) ReplaceTimeline(c *gin.Context) {
	var body TimelineInput
	if !bindContent(c, &body) {
		return
	}
	if err := ValidateTimeline(body.Items); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, err.Error())
		return
	}
	if body.Version < 1 {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "保存实习经历时必须提供有效的 version")
		return
	}
	if err := h.store.ReplaceTimeline(c.Request.Context(), body.Items, body.Version); err != nil {
		if errors.Is(err, ErrConflict) {
			apiresponse.Failure(c, apiresponse.Conflict, ErrConflict.Error())
			return
		}
		personalError(c, err)
		return
	}
	h.GetTimeline(c)
}

// bindContent 限制请求体大小并将 JSON 解码到目标结构。
// 参数：c 是当前 HTTP 请求上下文；output 是接收解码结果的指针。
// 返回：bool 表示请求体是否成功读取和解码。
func bindContent(c *gin.Context, output any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	if err := c.ShouldBindJSON(output); err != nil {
		apiresponse.Failure(c, apiresponse.InvalidRequest, "请求内容格式不正确或超过 4 MiB")
		return false
	}
	return true
}

// ValidateFootprints 检查足迹数量、字段、日期、坐标和关联地点。
// 参数：data 是待保存的完整足迹数据。
// 返回：error；数据有效时返回 nil，否则返回具体的中文校验错误。
func ValidateFootprints(data Footprints) error {
	if len(data.Locations) > 500 || len(data.Stays) > 1000 || len(data.Routes) > 2000 {
		return errors.New("地点、停留记录或路线数量超过允许上限")
	}
	locations := make(map[string]struct{}, len(data.Locations))
	itemIDs := make(map[string]struct{}, len(data.Stays)+len(data.Routes))
	for _, item := range data.Locations {
		if !locationIDPattern.MatchString(item.ID) || strings.TrimSpace(item.Name) == "" || len([]rune(item.Name)) > 100 {
			return errors.New("每个地点都需要有效的 ID 和名称")
		}
		if math.IsNaN(item.Latitude) || math.IsInf(item.Latitude, 0) || item.Latitude < -90 || item.Latitude > 90 || math.IsNaN(item.Longitude) || math.IsInf(item.Longitude, 0) || item.Longitude < -180 || item.Longitude > 180 {
			return errors.New("地点经纬度超出有效范围")
		}
		if _, exists := locations[item.ID]; exists {
			return errors.New("地点 ID 不能重复")
		}
		locations[item.ID] = struct{}{}
	}
	for _, item := range data.Stays {
		if !datePattern.MatchString(item.StartDate) || (item.EndDate != "" && !datePattern.MatchString(item.EndDate)) || strings.TrimSpace(item.Title) == "" {
			return errors.New("停留记录需要有效日期和标题")
		}
		if item.IsPresent && item.EndDate != "" {
			return errors.New("当前停留记录不能同时填写结束日期")
		}
		if item.ID != "" && !itemIDPattern.MatchString(item.ID) {
			return errors.New("停留记录 ID 格式不正确")
		}
		if item.ID != "" {
			if _, exists := itemIDs[item.ID]; exists {
				return errors.New("停留记录和路线 ID 不能重复")
			}
			itemIDs[item.ID] = struct{}{}
		}
		if _, exists := locations[item.LocationID]; !exists {
			return errors.New("停留记录引用了不存在的地点")
		}
	}
	for _, item := range data.Routes {
		if !datePattern.MatchString(item.Date) {
			return errors.New("路线日期格式应为 YYYY.MM 或 YYYY.MM.DD")
		}
		if item.ID != "" && !itemIDPattern.MatchString(item.ID) {
			return errors.New("路线 ID 格式不正确")
		}
		if item.ID != "" {
			if _, exists := itemIDs[item.ID]; exists {
				return errors.New("停留记录和路线 ID 不能重复")
			}
			itemIDs[item.ID] = struct{}{}
		}
		if _, exists := locations[item.From]; !exists {
			return errors.New("路线起点引用了不存在的地点")
		}
		if _, exists := locations[item.To]; !exists {
			return errors.New("路线终点引用了不存在的地点")
		}
		for _, image := range item.Images {
			if !validImageReference(image) {
				return errors.New("路线图片只支持站内图片路径或 HTTPS 链接")
			}
		}
	}
	return nil
}

// ValidateTimeline 检查实习经历的日期、必填字段和当前状态。
// 参数：items 是待保存的实习经历列表。
// 返回：error；数据有效时返回 nil，否则返回具体的中文校验错误。
func ValidateTimeline(items []Internship) error {
	if len(items) > 200 {
		return errors.New("实习经历数量不能超过 200 条")
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !datePattern.MatchString(item.StartDate) || (item.EndDate != "" && !datePattern.MatchString(item.EndDate)) || strings.TrimSpace(item.Company) == "" || strings.TrimSpace(item.Position) == "" {
			return errors.New("每条实习经历都需要有效日期、公司和职位")
		}
		if item.IsPresent && item.EndDate != "" {
			return errors.New("当前实习经历不能同时填写结束日期")
		}
		if item.ID != "" {
			if !itemIDPattern.MatchString(item.ID) {
				return errors.New("实习经历 ID 格式不正确")
			}
			if _, exists := seen[item.ID]; exists {
				return errors.New("实习经历 ID 不能重复")
			}
			seen[item.ID] = struct{}{}
		}
	}
	return nil
}

// validImageReference 判断路线图片是否为允许的站内地址或 HTTPS 链接。
// 参数：value 是待检查的图片引用。
// 返回：bool 表示图片引用是否有效。
func validImageReference(value string) bool {
	if strings.ContainsAny(value, `\\`) || strings.Contains(value, "..") {
		return false
	}
	if strings.HasPrefix(value, "/uploads/") || strings.HasPrefix(value, "/img/") {
		return true
	}
	return strings.HasPrefix(value, "https://")
}

// personalError 记录个人内容处理错误并返回通用中文错误响应。
// 参数：c 是当前 HTTP 请求上下文；err 是仅写入服务日志的内部错误。
// 返回：无；错误响应写入 HTTP 响应。
func personalError(c *gin.Context, err error) {
	log.Printf("个人内容请求失败：%v", err)
	apiresponse.Failure(c, apiresponse.InternalError, "读取或保存个人内容失败，请稍后再试")
}
