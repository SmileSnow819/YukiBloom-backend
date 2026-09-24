package sitecontent

import "time"

type Profile struct {
	Title          string   `json:"title"`
	Alternate      string   `json:"alternate"`
	Subtitle       string   `json:"subtitle"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Avatar         string   `json:"avatar"`
	ShowLogo       bool     `json:"showLogo" yaml:"showLogo"`
	Author         string   `json:"author"`
	URL            string   `json:"url"`
	DefaultOGImage string   `json:"defaultOgImage" yaml:"defaultOgImage"`
	StartYear      int      `json:"startYear" yaml:"startYear"`
	Timezone       string   `json:"timezone"`
	Keywords       []string `json:"keywords"`
}

type SocialLink struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
	Icon     string `json:"icon"`
	Color    string `json:"color"`
	Enabled  bool   `json:"enabled"`
}

type CategoryMapping struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type FeaturedCategory struct {
	Link        string `json:"link"`
	Label       string `json:"label"`
	Image       string `json:"image"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type FeaturedSeries struct {
	Slug            string            `json:"slug"`
	CategoryName    string            `json:"categoryName" yaml:"categoryName"`
	Label           string            `json:"label"`
	FullName        string            `json:"fullName" yaml:"fullName"`
	Description     string            `json:"description"`
	Cover           string            `json:"cover"`
	Enabled         bool              `json:"enabled"`
	Icon            string            `json:"icon"`
	HighlightOnHome bool              `json:"highlightOnHome" yaml:"highlightOnHome"`
	Links           map[string]string `json:"links"`
}

type NavigationItem struct {
	ID       string           `json:"id,omitempty"`
	Name     string           `json:"name"`
	NameKey  string           `json:"nameKey" yaml:"nameKey"`
	Path     string           `json:"path"`
	Icon     string           `json:"icon"`
	Children []NavigationItem `json:"children,omitempty"`
}

type AnnouncementLink struct {
	URL      string `json:"url"`
	Text     string `json:"text"`
	External bool   `json:"external"`
}

type Announcement struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Content     string           `json:"content"`
	Type        string           `json:"type"`
	Priority    int              `json:"priority"`
	Color       string           `json:"color"`
	PublishDate string           `json:"publishDate" yaml:"publishDate"`
	StartDate   *time.Time       `json:"startDate,omitempty" yaml:"startDate"`
	EndDate     *time.Time       `json:"endDate,omitempty" yaml:"endDate"`
	Enabled     bool             `json:"enabled"`
	Link        AnnouncementLink `json:"link"`
}

type FriendSettings struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	ApplyTitle  string `json:"applyTitle" yaml:"applyTitle"`
	ApplyDesc   string `json:"applyDesc" yaml:"applyDesc"`
	ExampleYAML string `json:"exampleYaml" yaml:"exampleYaml"`
}

type FriendLink struct {
	ID          string `json:"id,omitempty"`
	Site        string `json:"site"`
	URL         string `json:"url"`
	Owner       string `json:"owner"`
	Description string `json:"description" yaml:"desc"`
	Image       string `json:"image"`
	Color       string `json:"color"`
	Status      string `json:"status"`
}

type Translation struct {
	Locale      string `json:"locale"`
	EntityType  string `json:"entityType"`
	EntityKey   string `json:"entityKey"`
	Label       string `json:"label"`
	FullName    string `json:"fullName"`
	Description string `json:"description"`
}

type MusicLink struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type MusicGroup struct {
	ID      string      `json:"id,omitempty"`
	Title   string      `json:"title"`
	Enabled bool        `json:"enabled"`
	Links   []MusicLink `json:"links"`
}

type BackgroundTrack struct {
	ID      string `json:"id,omitempty"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type Page struct {
	ID           string    `json:"id,omitempty"`
	Locale       string    `json:"locale"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	BodyMarkdown string    `json:"bodyMarkdown"`
	Status       string    `json:"status"`
	Version      int64     `json:"version"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Content struct {
	Version            int64              `json:"version"`
	Profile            *Profile           `json:"profile"`
	SocialLinks        []SocialLink       `json:"socialLinks"`
	CategoryMappings   []CategoryMapping  `json:"categoryMappings"`
	FeaturedCategories []FeaturedCategory `json:"featuredCategories"`
	FeaturedSeries     []FeaturedSeries   `json:"featuredSeries"`
	Navigation         []NavigationItem   `json:"navigation"`
	Announcements      []Announcement     `json:"announcements"`
	FriendSettings     FriendSettings     `json:"friendSettings"`
	FriendLinks        []FriendLink       `json:"friendLinks"`
	Translations       []Translation      `json:"translations"`
	MusicGroups        []MusicGroup       `json:"musicGroups"`
	BackgroundMusic    []BackgroundTrack  `json:"backgroundMusic"`
}
