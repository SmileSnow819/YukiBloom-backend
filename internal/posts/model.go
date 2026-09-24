package posts

import (
	"encoding/json"
	"time"
)

type Post struct {
	ID           string          `json:"id"`
	Locale       string          `json:"locale"`
	Slug         string          `json:"slug"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	BodyMarkdown string          `json:"bodyMarkdown,omitempty"`
	Status       string          `json:"status"`
	DisplayDate  *time.Time      `json:"displayDate,omitempty"`
	PublishedAt  *time.Time      `json:"publishedAt,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	Version      int64           `json:"version"`
	Categories   []string        `json:"categories"`
	Tags         []string        `json:"tags"`
	Extra        json.RawMessage `json:"extra"`
	CoverMediaID *string         `json:"coverMediaId,omitempty"`
}

type PostInput struct {
	Locale       string          `json:"locale"`
	Slug         string          `json:"slug"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	BodyMarkdown string          `json:"bodyMarkdown"`
	DisplayDate  *time.Time      `json:"displayDate"`
	Categories   []string        `json:"categories"`
	Tags         []string        `json:"tags"`
	Extra        json.RawMessage `json:"extra"`
	CoverMediaID *string         `json:"coverMediaId"`
	Version      int64           `json:"version"`
}

type Page struct {
	Items []Post `json:"items"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
	Total int64  `json:"total"`
}
