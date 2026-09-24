package importer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
	yaml "github.com/goccy/go-yaml"
)

type MarkdownPost struct {
	Path      string
	Input     posts.PostInput
	Draft     bool
	CoverPath string
}

func ParseMarkdown(path, locale string, data []byte) (MarkdownPost, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return MarkdownPost{}, errors.New("文件缺少 YAML frontmatter 起始标记 ---")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return MarkdownPost{}, errors.New("文件缺少 YAML frontmatter 结束标记 ---")
	}
	header := []byte(text[4 : 4+end])
	body := text[4+end+5:]
	fields := map[string]any{}
	if err := yaml.Unmarshal(header, &fields); err != nil {
		return MarkdownPost{}, fmt.Errorf("frontmatter 格式错误：%w", err)
	}
	if locale == "" {
		locale = "zh-CN"
	}
	title := stringValue(fields["title"])
	if strings.TrimSpace(title) == "" {
		return MarkdownPost{}, errors.New("frontmatter 缺少 title")
	}
	slug := stringValue(fields["link"])
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	date, err := parseDate(fields["date"])
	if err != nil {
		return MarkdownPost{}, fmt.Errorf("frontmatter date 无效：%w", err)
	}
	description := stringValue(fields["description"])
	categories := stringSlice(fields["categories"])
	tags := stringSlice(fields["tags"])
	draft, _ := fields["draft"].(bool)
	cover := stringValue(fields["cover"])
	extra := make(map[string]any, len(fields))
	for key, value := range fields {
		extra[key] = value
	}
	extraJSON, err := json.Marshal(extra)
	if err != nil {
		return MarkdownPost{}, fmt.Errorf("frontmatter 扩展字段无法保存：%w", err)
	}
	return MarkdownPost{
		Path: path,
		Input: posts.PostInput{
			Locale: locale, Slug: slug, Title: title, Description: description,
			BodyMarkdown: strings.TrimLeft(body, "\n"), DisplayDate: &date,
			Categories: categories, Tags: tags, Extra: extraJSON,
		},
		Draft: draft, CoverPath: cover,
	}, nil
}

func parseDate(value any) (time.Time, error) {
	date, ok := value.(string)
	if !ok {
		return time.Time{}, errors.New("日期必须是文本")
	}
	date = strings.TrimSpace(date)
	formats := []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, format := range formats {
		var parsed time.Time
		var err error
		if format == time.RFC3339Nano {
			parsed, err = time.Parse(format, date)
		} else {
			parsed, err = time.ParseInLocation(format, date, shanghai)
		}
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("支持 YYYY-MM-DD、YYYY-MM-DD HH:mm:ss 或带时区的 ISO 日期")
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func stringSlice(value any) []string {
	var result []string
	var appendValue func(any)
	appendValue = func(item any) {
		switch typed := item.(type) {
		case string:
			result = append(result, typed)
		case []any:
			for _, nested := range typed {
				appendValue(nested)
			}
		}
	}
	appendValue(value)
	return result
}
