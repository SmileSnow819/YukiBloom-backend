package personal

type Location struct {
	ID        string  `json:"id" yaml:"id"`
	Name      string  `json:"name" yaml:"name"`
	Latitude  float64 `json:"lat" yaml:"lat"`
	Longitude float64 `json:"lng" yaml:"lng"`
	Type      string  `json:"type" yaml:"type"`
	Icon      string  `json:"icon" yaml:"icon"`
	SortOrder int     `json:"sortOrder" yaml:"sortOrder"`
}

type Stay struct {
	ID          string `json:"id" yaml:"id"`
	StartDate   string `json:"startDate" yaml:"startDate"`
	EndDate     string `json:"endDate" yaml:"endDate"`
	IsPresent   bool   `json:"isPresent" yaml:"isPresent"`
	LocationID  string `json:"locationId" yaml:"locationId"`
	Title       string `json:"title" yaml:"title"`
	Type        string `json:"type" yaml:"type"`
	Description string `json:"description" yaml:"description"`
	SortOrder   int    `json:"sortOrder" yaml:"sortOrder"`
}

type Route struct {
	ID          string   `json:"id" yaml:"id"`
	From        string   `json:"from" yaml:"from"`
	To          string   `json:"to" yaml:"to"`
	Date        string   `json:"date" yaml:"date"`
	Transport   string   `json:"transport" yaml:"transport"`
	Label       string   `json:"label" yaml:"label"`
	Description string   `json:"description" yaml:"description"`
	Images      []string `json:"images" yaml:"images"`
	SortOrder   int      `json:"sortOrder" yaml:"sortOrder"`
}

type Footprints struct {
	Locations []Location `json:"locations" yaml:"locations"`
	Stays     []Stay     `json:"stays" yaml:"stays"`
	Routes    []Route    `json:"routes" yaml:"routes"`
}

type Internship struct {
	ID          string `json:"id" yaml:"id"`
	StartDate   string `json:"startDate" yaml:"startDate"`
	EndDate     string `json:"endDate" yaml:"endDate"`
	IsPresent   bool   `json:"isPresent" yaml:"isPresent"`
	Company     string `json:"company" yaml:"company"`
	Icon        string `json:"icon" yaml:"icon"`
	IconColor   string `json:"iconColor" yaml:"iconColor"`
	Position    string `json:"position" yaml:"position"`
	Description string `json:"description" yaml:"description"`
	SortOrder   int    `json:"sortOrder" yaml:"sortOrder"`
}

// TimelineInput 是整体保存实习经历接口的请求体。
type TimelineInput struct {
	Items []Internship `json:"items"`
}
