package personal

import (
	"errors"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	datePattern       = regexp.MustCompile(`^\d{4}\.(?:0[1-9]|1[0-2])(?:\.(?:0[1-9]|[12]\d|3[01]))?$`)
	itemIDPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	locationIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)
)

type Handler struct{ store *Store }

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

func (h *Handler) GetFootprints(c *gin.Context) {
	data, err := h.store.Footprints(c.Request.Context())
	if err != nil {
		personalError(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

func (h *Handler) ReplaceFootprints(c *gin.Context) {
	var data Footprints
	if !bindContent(c, &data) {
		return
	}
	if err := ValidateFootprints(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.ReplaceFootprints(c.Request.Context(), data); err != nil {
		personalError(c, err)
		return
	}
	h.GetFootprints(c)
}

func (h *Handler) GetTimeline(c *gin.Context) {
	items, err := h.store.Timeline(c.Request.Context())
	if err != nil {
		personalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) ReplaceTimeline(c *gin.Context) {
	var body struct {
		Items []Internship `json:"items"`
	}
	if !bindContent(c, &body) {
		return
	}
	if err := ValidateTimeline(body.Items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.ReplaceTimeline(c.Request.Context(), body.Items); err != nil {
		personalError(c, err)
		return
	}
	h.GetTimeline(c)
}

func bindContent(c *gin.Context, output any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20)
	if err := c.ShouldBindJSON(output); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求内容格式不正确或超过 4 MiB"})
		return false
	}
	return true
}

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

func validImageReference(value string) bool {
	if strings.ContainsAny(value, `\\`) || strings.Contains(value, "..") {
		return false
	}
	if strings.HasPrefix(value, "/uploads/") || strings.HasPrefix(value, "/img/") {
		return true
	}
	return strings.HasPrefix(value, "https://")
}

func personalError(c *gin.Context, err error) {
	log.Printf("个人内容请求失败：%v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "读取或保存个人内容失败，请稍后再试"})
}
