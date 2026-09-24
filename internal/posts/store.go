package posts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("文章不存在")
	ErrConflict      = errors.New("文章已被其他操作修改，或该语言下的链接地址已占用")
	ErrMediaNotFound = errors.New("封面图片不存在")
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const postColumns = `id, locale, slug, title, description, body_markdown, status, display_date, published_at, created_at, updated_at, version, categories, tags, extra, cover_media_id`
const publicListColumns = `id, locale, slug, title, description, ''::text AS body_markdown, status, display_date, published_at, created_at, updated_at, version, categories, tags, extra, cover_media_id`

func (s *Store) PublicList(ctx context.Context, locale, category, tag, query string, page, limit int) (Page, error) {
	conditions := []string{"status = 'published'"}
	args := []any{}
	add := func(value string) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if locale != "" {
		conditions = append(conditions, "locale = "+add(locale))
	}
	if category != "" {
		conditions = append(conditions, add(category)+" = ANY(categories)")
	}
	if tag != "" {
		conditions = append(conditions, add(tag)+" = ANY(tags)")
	}
	if query != "" {
		p := add("%" + query + "%")
		conditions = append(conditions, "(title ILIKE "+p+" OR description ILIKE "+p+" OR body_markdown ILIKE "+p+")")
	}
	where := strings.Join(conditions, " AND ")
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM posts WHERE "+where, args...).Scan(&total); err != nil {
		return Page{}, err
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := s.pool.Query(ctx, "SELECT "+publicListColumns+" FROM posts WHERE "+where+fmt.Sprintf(" ORDER BY COALESCE(display_date, published_at) DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items, err := collectPosts(rows)
	return Page{Items: items, Page: page, Limit: limit, Total: total}, err
}

func (s *Store) PublicBySlug(ctx context.Context, locale, slug string) (Post, error) {
	post, err := scanPost(s.pool.QueryRow(ctx, "SELECT "+postColumns+" FROM posts WHERE status = 'published' AND locale = $1 AND slug = $2", locale, slug))
	return post, mapReadError(err)
}

func (s *Store) AdminList(ctx context.Context, page, limit int) (Page, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM posts").Scan(&total); err != nil {
		return Page{}, err
	}
	rows, err := s.pool.Query(ctx, "SELECT "+postColumns+" FROM posts ORDER BY updated_at DESC, id DESC LIMIT $1 OFFSET $2", limit, (page-1)*limit)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items, err := collectPosts(rows)
	return Page{Items: items, Page: page, Limit: limit, Total: total}, err
}

func (s *Store) AdminByID(ctx context.Context, id string) (Post, error) {
	post, err := scanPost(s.pool.QueryRow(ctx, "SELECT "+postColumns+" FROM posts WHERE id = $1", id))
	return post, mapReadError(err)
}

func (s *Store) Create(ctx context.Context, input PostInput) (Post, error) {
	extra := input.Extra
	if len(extra) == 0 {
		extra = json.RawMessage(`{}`)
	}
	if input.Categories == nil {
		input.Categories = []string{}
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	post, err := scanPost(s.pool.QueryRow(ctx, `INSERT INTO posts
		(locale, slug, title, description, body_markdown, display_date, categories, tags, extra, cover_media_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+postColumns,
		input.Locale, input.Slug, input.Title, input.Description, input.BodyMarkdown, input.DisplayDate,
		input.Categories, input.Tags, []byte(extra), input.CoverMediaID))
	return post, mapWriteError(err)
}

func (s *Store) Update(ctx context.Context, id string, input PostInput) (Post, error) {
	extra := input.Extra
	if len(extra) == 0 {
		extra = json.RawMessage(`{}`)
	}
	if input.Categories == nil {
		input.Categories = []string{}
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	post, err := scanPost(s.pool.QueryRow(ctx, `UPDATE posts SET
		locale=$2, slug=$3, title=$4, description=$5, body_markdown=$6, display_date=$7,
		categories=$8, tags=$9, extra=$10, cover_media_id=$11, updated_at=now(), version=version+1
		WHERE id=$1 AND version=$12 RETURNING `+postColumns,
		id, input.Locale, input.Slug, input.Title, input.Description, input.BodyMarkdown, input.DisplayDate,
		input.Categories, input.Tags, []byte(extra), input.CoverMediaID, input.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if checkErr := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1)", id).Scan(&exists); checkErr != nil {
			return Post{}, checkErr
		}
		if !exists {
			return Post{}, ErrNotFound
		}
		return Post{}, ErrConflict
	}
	return post, mapWriteError(err)
}

func (s *Store) SetPublished(ctx context.Context, id string, published bool) (Post, error) {
	status := "draft"
	if published {
		status = "published"
	}
	post, err := scanPost(s.pool.QueryRow(ctx, `UPDATE posts SET status=$2,
		published_at=CASE WHEN $2='published' THEN COALESCE(published_at, now()) ELSE NULL END,
		updated_at=now(), version=version+1 WHERE id=$1 RETURNING `+postColumns, id, status))
	return post, mapWriteError(err)
}

func scanPost(row pgx.Row) (Post, error) {
	var post Post
	var extra []byte
	err := row.Scan(&post.ID, &post.Locale, &post.Slug, &post.Title, &post.Description,
		&post.BodyMarkdown, &post.Status, &post.DisplayDate, &post.PublishedAt, &post.CreatedAt,
		&post.UpdatedAt, &post.Version, &post.Categories, &post.Tags, &extra, &post.CoverMediaID)
	post.Extra = json.RawMessage(extra)
	return post, err
}

func collectPosts(rows pgx.Rows) ([]Post, error) {
	items := make([]Post, 0)
	for rows.Next() {
		post, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, post)
	}
	return items, rows.Err()
}

func mapReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func mapWriteError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrMediaNotFound
		}
	}
	return err
}
