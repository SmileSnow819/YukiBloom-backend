package sitecontent

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

type legacySite struct {
	Site               Profile               `yaml:"site"`
	Social             map[string]SocialLink `yaml:"social"`
	CategoryMap        map[string]string     `yaml:"categoryMap"`
	FeaturedCategories []map[string]any      `yaml:"featuredCategories"`
	FeaturedSeries     []map[string]any      `yaml:"featuredSeries"`
	Navigation         []NavigationItem      `yaml:"navigation"`
	Announcements      []map[string]any      `yaml:"announcements"`
	Friends            *legacyFriends        `yaml:"friends"`
	BGM                legacyBGM             `yaml:"bgm"`
}

type legacyFriends struct {
	Intro FriendSettings `yaml:"intro"`
	Data  []FriendLink   `yaml:"data"`
}

type legacyBGM struct {
	Enabled bool `yaml:"enabled"`
	Audio   []struct {
		Title string   `yaml:"title"`
		List  []string `yaml:"list"`
	} `yaml:"audio"`
}

type legacyTranslation struct {
	Categories map[string]string `yaml:"categories"`
	Series     map[string]struct {
		Label       string `yaml:"label"`
		FullName    string `yaml:"fullName"`
		Description string `yaml:"description"`
	} `yaml:"series"`
	FeaturedCategories map[string]struct {
		Label       string `yaml:"label"`
		Description string `yaml:"description"`
	} `yaml:"featuredCategories"`
}

func LoadLegacy(sitePath, translationsPath, aboutPath, musicPath string) (Content, []Page, error) {
	var legacy legacySite
	if err := readLegacyYAML(sitePath, &legacy); err != nil {
		return Content{}, nil, fmt.Errorf("读取站点配置失败：%w", err)
	}
	content := emptyContent()
	content.Profile = &legacy.Site
	socialKeys := sortedKeys(legacy.Social)
	for _, key := range socialKeys {
		item := legacy.Social[key]
		item.Platform = key
		item.Enabled = true
		content.SocialLinks = append(content.SocialLinks, item)
	}
	for _, name := range sortedKeys(legacy.CategoryMap) {
		content.CategoryMappings = append(content.CategoryMappings, CategoryMapping{Name: name, Slug: legacy.CategoryMap[name]})
	}
	for _, raw := range legacy.FeaturedCategories {
		var item FeaturedCategory
		if err := remarshalYAML(raw, &item); err != nil {
			return Content{}, nil, fmt.Errorf("精选分类格式错误：%w", err)
		}
		if _, present := raw["enabled"]; !present {
			item.Enabled = true
		}
		content.FeaturedCategories = append(content.FeaturedCategories, item)
	}
	for _, raw := range legacy.FeaturedSeries {
		var item FeaturedSeries
		if err := remarshalYAML(raw, &item); err != nil {
			return Content{}, nil, fmt.Errorf("精选系列格式错误：%w", err)
		}
		if _, present := raw["enabled"]; !present {
			item.Enabled = true
		}
		if _, present := raw["highlightOnHome"]; !present {
			item.HighlightOnHome = true
		}
		content.FeaturedSeries = append(content.FeaturedSeries, item)
	}
	content.Navigation = legacy.Navigation
	for _, raw := range legacy.Announcements {
		var item Announcement
		if err := remarshalYAML(raw, &item); err != nil {
			return Content{}, nil, fmt.Errorf("公告格式错误：%w", err)
		}
		if _, present := raw["enabled"]; !present {
			item.Enabled = true
		}
		content.Announcements = append(content.Announcements, item)
	}
	if legacy.Friends != nil {
		content.FriendSettings = legacy.Friends.Intro
		content.FriendLinks = legacy.Friends.Data
		for index := range content.FriendLinks {
			if content.FriendLinks[index].Status == "" {
				content.FriendLinks[index].Status = "approved"
			}
		}
	}
	for _, audio := range legacy.BGM.Audio {
		for _, link := range audio.List {
			content.BackgroundMusic = append(content.BackgroundMusic, BackgroundTrack{Title: audio.Title, URL: link, Enabled: legacy.BGM.Enabled})
		}
	}
	var translations map[string]legacyTranslation
	if err := readLegacyYAML(translationsPath, &translations); err != nil {
		return Content{}, nil, fmt.Errorf("读取内容翻译失败：%w", err)
	}
	for _, locale := range sortedKeys(translations) {
		values := translations[locale]
		for _, key := range sortedKeys(values.Categories) {
			content.Translations = append(content.Translations, Translation{Locale: locale, EntityType: "categories", EntityKey: key, Label: values.Categories[key]})
		}
		for _, key := range sortedKeys(values.Series) {
			value := values.Series[key]
			content.Translations = append(content.Translations, Translation{Locale: locale, EntityType: "series", EntityKey: key, Label: value.Label, FullName: value.FullName, Description: value.Description})
		}
		for _, key := range sortedKeys(values.FeaturedCategories) {
			value := values.FeaturedCategories[key]
			content.Translations = append(content.Translations, Translation{Locale: locale, EntityType: "featuredCategories", EntityKey: key, Label: value.Label, Description: value.Description})
		}
	}
	musicGroups, musicPage, err := parseLegacyMusic(musicPath)
	if err != nil {
		return Content{}, nil, err
	}
	content.MusicGroups = musicGroups
	aboutPage, err := parseLegacyAbout(aboutPath)
	if err != nil {
		return Content{}, nil, err
	}
	if err := ValidateContent(content); err != nil {
		return Content{}, nil, fmt.Errorf("站点内容字段无效：%w", err)
	}
	pages := []Page{aboutPage, musicPage}
	for _, page := range pages {
		if err := ValidatePage(page, false); err != nil {
			return Content{}, nil, fmt.Errorf("页面 %s 无效：%w", page.Slug, err)
		}
	}
	return content, pages, nil
}

func parseLegacyAbout(path string) (Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Page{}, fmt.Errorf("读取关于我页面失败：%w", err)
	}
	text := string(data)
	start := strings.Index(text, "<PageLayout frontmatter={frontmatter}>")
	end := strings.LastIndex(text, "</PageLayout>")
	if start < 0 || end <= start {
		return Page{}, errors.New("关于我页面的布局标记无法识别")
	}
	body := strings.TrimSpace(text[start+len("<PageLayout frontmatter={frontmatter}>") : end])
	body = strings.ReplaceAll(body, "<TimelineWrapper />", "<!-- 实习经历时间线由页面组件展示 -->")
	return Page{Locale: "zh-CN", Slug: "about", Title: "关于我", Description: "关于我？", BodyMarkdown: body}, nil
}

func parseLegacyMusic(path string) ([]MusicGroup, Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Page{}, fmt.Errorf("读取歌单页面失败：%w", err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return nil, Page{}, errors.New("歌单页面缺少 frontmatter")
	}
	frontmatterEnd := strings.Index(text[4:], "\n---\n")
	if frontmatterEnd < 0 {
		return nil, Page{}, errors.New("歌单页面 frontmatter 未结束")
	}
	var meta struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(text[4:4+frontmatterEnd]), &meta); err != nil {
		return nil, Page{}, fmt.Errorf("歌单页面 frontmatter 格式错误：%w", err)
	}
	body := text[4+frontmatterEnd+5:]
	start := strings.Index(body, "{% media audio %}")
	end := strings.Index(body, "{% endmedia %}")
	if start < 0 || end <= start {
		return nil, Page{}, errors.New("歌单页面缺少音频列表标记")
	}
	var groups []struct {
		Title string   `yaml:"title"`
		List  []string `yaml:"list"`
	}
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(body[start+len("{% media audio %}"):end])), &groups); err != nil {
		return nil, Page{}, fmt.Errorf("歌单列表格式错误：%w", err)
	}
	output := make([]MusicGroup, 0, len(groups))
	for _, group := range groups {
		item := MusicGroup{Title: group.Title, Enabled: true, Links: []MusicLink{}}
		for _, link := range group.List {
			item.Links = append(item.Links, MusicLink{URL: link})
		}
		output = append(output, item)
	}
	return output, Page{Locale: "zh-CN", Slug: "music", Title: meta.Title, Description: meta.Description, BodyMarkdown: ""}, nil
}

func readLegacyYAML(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, value)
}

func remarshalYAML(raw map[string]any, value any) error {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, value)
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
