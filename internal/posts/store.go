package posts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

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

// NewStore 创建文章数据存储。
// 参数：pool 是 PostgreSQL 连接池。
// 返回：*Store 是可执行文章查询和写入的存储对象。
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const postColumns = `id, locale, slug, title, description, body_markdown, status, display_date, published_at, created_at, updated_at, version, categories, tags, extra, cover_media_id`
const publicListColumns = `id, locale, slug, title, description, ''::text AS body_markdown, status, display_date, published_at, created_at, updated_at, version, categories, tags, extra, cover_media_id`

// PublicList 按语言、分类、标签和关键词筛选已发布文章并分页。
// 参数：s 是文章存储；ctx 控制查询；locale、category、tag、query 是筛选条件；page 和 limit 是页码与每页条数。
// 返回：Page 包含文章和分页信息；error 表示数据库查询失败。
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

// PublicBySlug 按语言和链接标识读取一篇已发布文章。
// 参数：s 是文章存储；ctx 控制查询；locale 是语言代码；slug 是文章链接标识。
// 返回：Post 是匹配文章；error 表示文章不存在或查询失败。
func (s *Store) PublicBySlug(ctx context.Context, locale, slug string) (Post, error) {
	post, err := scanPost(s.pool.QueryRow(ctx, "SELECT "+postColumns+" FROM posts WHERE status = 'published' AND locale = $1 AND slug = $2", locale, slug))
	return post, mapReadError(err)
}

// AdminList 查询管理端文章列表并分页。
// 参数：s 是文章存储；ctx 控制查询；page 和 limit 是页码与每页条数。
// 返回：Page 包含文章和分页信息；error 表示数据库查询失败。
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

// AdminByID 按数据库 ID 查询文章，包含草稿。
// 参数：s 是文章存储；ctx 控制查询；id 是文章 ID。
// 返回：Post 是匹配文章；error 表示文章不存在或查询失败。
func (s *Store) AdminByID(ctx context.Context, id string) (Post, error) {
	post, err := scanPost(s.pool.QueryRow(ctx, "SELECT "+postColumns+" FROM posts WHERE id = $1", id))
	return post, mapReadError(err)
}

// Exists 检查指定语言下的文章链接标识是否已占用。
// 参数：s 是文章存储；ctx 控制查询；locale 是语言代码；slug 是文章链接标识。
// 返回：bool 表示是否已存在；error 表示数据库查询失败。
func (s *Store) Exists(ctx context.Context, locale, slug string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE locale=$1 AND slug=$2)", locale, slug).Scan(&exists)
	return exists, err
}

// Create 新建文章并在一个事务中提交数据库变更。
// 参数：s 是文章存储；ctx 控制事务；input 包含文章正文、元数据和封面信息。
// 返回：Post 是创建后的文章；error 表示写入失败或唯一标识冲突。
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return Post{}, err
	}
	post, err := scanPost(tx.QueryRow(ctx, `INSERT INTO posts
		(locale, slug, title, description, body_markdown, display_date, categories, tags, extra, cover_media_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+postColumns,
		input.Locale, input.Slug, input.Title, input.Description, input.BodyMarkdown, input.DisplayDate,
		input.Categories, input.Tags, []byte(extra), input.CoverMediaID))
	if err != nil {
		return Post{}, mapWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, err
	}
	return post, nil
}

// Import 仅导入尚不存在的文章，不覆盖后台后续编辑的内容。
// 参数：s 是文章存储；ctx 控制事务；input 是待导入文章；published 决定是否直接发布。
// 返回：Post 是新导入文章；bool 表示本次是否创建；error 表示导入失败。
func (s *Store) Import(ctx context.Context, input PostInput, published bool) (Post, bool, error) {
	status := "draft"
	var publishedAt *time.Time
	if published {
		status = "published"
		publishedAt = input.DisplayDate
		if publishedAt == nil {
			now := time.Now()
			publishedAt = &now
		}
	}
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return Post{}, false, err
	}
	post, err := scanPost(tx.QueryRow(ctx, `INSERT INTO posts
		(locale, slug, title, description, body_markdown, status, display_date, published_at, categories, tags, extra, cover_media_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (locale, slug) DO NOTHING RETURNING `+postColumns,
		input.Locale, input.Slug, input.Title, input.Description, input.BodyMarkdown,
		status, input.DisplayDate, publishedAt, input.Categories, input.Tags, []byte(extra), input.CoverMediaID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, false, nil
	}
	if err != nil {
		return Post{}, false, mapWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, false, err
	}
	return post, true, nil
}

// Update 按文章 ID 和版本号更新内容，避免覆盖并发修改。
// 参数：s 是文章存储；ctx 控制事务；id 是文章 ID；input 是新内容及当前版本号。
// 返回：Post 是更新后的文章；error 表示文章不存在、版本冲突或写入失败。
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)"); err != nil {
		return Post{}, err
	}
	post, err := scanPost(tx.QueryRow(ctx, `UPDATE posts SET
		locale=$2, slug=$3, title=$4, description=$5, body_markdown=$6, display_date=$7,
		categories=$8, tags=$9, extra=$10, cover_media_id=$11, updated_at=now(), version=version+1
		WHERE id=$1 AND version=$12 RETURNING `+postColumns,
		id, input.Locale, input.Slug, input.Title, input.Description, input.BodyMarkdown, input.DisplayDate,
		input.Categories, input.Tags, []byte(extra), input.CoverMediaID, input.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if checkErr := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1)", id).Scan(&exists); checkErr != nil {
			return Post{}, checkErr
		}
		if !exists {
			return Post{}, ErrNotFound
		}
		return Post{}, ErrConflict
	}
	if err != nil {
		return Post{}, mapWriteError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, err
	}
	return post, nil
}

// SetPublished 修改文章的发布状态。
// 参数：s 是文章存储；ctx 控制查询；id 是文章 ID；published 表示是否发布。
// 返回：Post 是更新后的文章；error 表示文章不存在或数据库写入失败。
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

// scanPost 将数据库当前行的字段读取为文章对象。
// 参数：row 是字段顺序与 postColumns 一致的数据库行。
// 返回：Post 是读取出的文章；error 表示字段读取失败。
func scanPost(row pgx.Row) (Post, error) {
	var post Post
	var extra []byte
	err := row.Scan(&post.ID, &post.Locale, &post.Slug, &post.Title, &post.Description,
		&post.BodyMarkdown, &post.Status, &post.DisplayDate, &post.PublishedAt, &post.CreatedAt,
		&post.UpdatedAt, &post.Version, &post.Categories, &post.Tags, &extra, &post.CoverMediaID)
	post.Extra = json.RawMessage(extra)
	return post, err
}

// collectPosts 逐行读取文章查询结果。
// 参数：rows 是文章查询返回的结果集。
// 返回：[]Post 是读取到的文章列表；error 表示扫描或遍历失败。
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

// mapReadError 将数据库的无记录错误转换为文章领域错误。
// 参数：err 是数据库读取错误。
// 返回：error 是转换后的文章不存在错误或原始错误。
func mapReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// mapWriteError 将数据库写入错误转换为文章领域错误。
// 参数：err 是数据库写入错误。
// 返回：error 是文章不存在、冲突、封面缺失错误或原始错误。
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
