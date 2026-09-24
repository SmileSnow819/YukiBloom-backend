package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "golang.org/x/image/webp"
)

const MaxUploadBytes = 10 << 20
const maxImagePixels = 20_000_000

var (
	ErrInvalidImage = errors.New("只支持有效的 JPEG、PNG 或 WebP 图片")
	ErrTooLarge     = errors.New("图片不能超过 10 MiB")
	ErrDimensions   = errors.New("图片尺寸过大，宽高像素乘积不能超过 2000 万")
	ErrInUse        = errors.New("图片仍被文章引用，不能删除")
	ErrNotFound     = errors.New("图片不存在")
)

type Item struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	MIMEType  string    `json:"mimeType"`
	SizeBytes int64     `json:"sizeBytes"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	pool *pgxpool.Pool
	root string
}

func NewStore(pool *pgxpool.Pool, root string) *Store { return &Store{pool: pool, root: root} }

func (s *Store) Save(ctx context.Context, source io.Reader) (Item, error) {
	data, err := io.ReadAll(io.LimitReader(source, MaxUploadBytes+1))
	if err != nil {
		return Item{}, fmt.Errorf("读取图片失败：%w", err)
	}
	if len(data) > MaxUploadBytes {
		return Item{}, ErrTooLarge
	}
	info, err := Validate(data)
	if err != nil {
		return Item{}, err
	}
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return Item{}, fmt.Errorf("创建图片目录失败：%w", err)
	}
	key, err := storageKey(info.Extension)
	if err != nil {
		return Item{}, fmt.Errorf("生成图片文件名失败：%w", err)
	}
	temp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return Item{}, fmt.Errorf("创建图片临时文件失败：%w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o640); err != nil {
		temp.Close()
		return Item{}, fmt.Errorf("设置图片文件权限失败：%w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return Item{}, fmt.Errorf("写入图片失败：%w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return Item{}, fmt.Errorf("保存图片失败：%w", err)
	}
	if err := temp.Close(); err != nil {
		return Item{}, fmt.Errorf("关闭图片文件失败：%w", err)
	}
	path := filepath.Join(s.root, key)
	if err := os.Rename(tempPath, path); err != nil {
		return Item{}, fmt.Errorf("保存图片文件失败：%w", err)
	}
	var item Item
	err = s.pool.QueryRow(ctx, `INSERT INTO media (storage_key, mime_type, size_bytes, width, height)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, storage_key, mime_type, size_bytes, width, height, created_at`,
		key, info.MIMEType, len(data), info.Width, info.Height).Scan(&item.ID, &key, &item.MIMEType, &item.SizeBytes, &item.Width, &item.Height, &item.CreatedAt)
	if err != nil {
		_ = os.Remove(path)
		return Item{}, fmt.Errorf("保存图片信息失败：%w", err)
	}
	item.URL = "/uploads/" + key
	return item, nil
}

type ImageInfo struct {
	MIMEType  string
	Extension string
	Width     int
	Height    int
}

func Validate(data []byte) (ImageInfo, error) {
	if len(data) > MaxUploadBytes {
		return ImageInfo{}, ErrTooLarge
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageInfo{}, ErrInvalidImage
	}
	if config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return ImageInfo{}, ErrDimensions
	}
	_, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || decodedFormat != format {
		return ImageInfo{}, ErrInvalidImage
	}
	mimeType, extension, ok := imageType(format)
	if !ok {
		return ImageInfo{}, ErrInvalidImage
	}
	return ImageInfo{MIMEType: mimeType, Extension: extension, Width: config.Width, Height: config.Height}, nil
}

func (s *Store) List(ctx context.Context, page, limit int) ([]Item, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM media").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, storage_key, mime_type, size_bytes, width, height, created_at
		FROM media ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Item, 0)
	for rows.Next() {
		var item Item
		var key string
		if err := rows.Scan(&item.ID, &key, &item.MIMEType, &item.SizeBytes, &item.Width, &item.Height, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		item.URL = "/uploads/" + key
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *Store) PublicFile(ctx context.Context, key string) (string, string, error) {
	if !validStorageKey(key) {
		return "", "", ErrNotFound
	}
	var mimeType string
	err := s.pool.QueryRow(ctx, "SELECT mime_type FROM media WHERE storage_key=$1", key).Scan(&mimeType)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return filepath.Join(s.root, key), mimeType, err
}

func (s *Store) DeleteUnused(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(20260925)")
	if err != nil {
		return err
	}
	var key string
	err = tx.QueryRow(ctx, "SELECT storage_key FROM media WHERE id=$1 FOR UPDATE", id).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var used bool
	err = tx.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM posts WHERE cover_media_id=$1 OR position($2 in body_markdown)>0)
		OR EXISTS(SELECT 1 FROM footprint_routes r, unnest(r.images) AS image_url(value) WHERE position($2 in image_url.value)>0)
		OR EXISTS(SELECT 1 FROM content_pages WHERE position($2 in body_markdown)>0)
		OR EXISTS(SELECT 1 FROM site_profile WHERE position($2 in avatar_url)>0 OR position($2 in default_og_image)>0)
		OR EXISTS(SELECT 1 FROM featured_categories WHERE position($2 in image)>0)
		OR EXISTS(SELECT 1 FROM featured_series WHERE position($2 in cover)>0)
		OR EXISTS(SELECT 1 FROM friend_links WHERE position($2 in image)>0)`, id, key).Scan(&used)
	if err != nil {
		return err
	}
	if used {
		return ErrInUse
	}
	if _, err := tx.Exec(ctx, "DELETE FROM media WHERE id=$1", id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrInUse
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.root, key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("图片记录已删除，但清理文件失败：%w", err)
	}
	return nil
}

func imageType(format string) (string, string, bool) {
	switch format {
	case "jpeg":
		return "image/jpeg", ".jpg", true
	case "png":
		return "image/png", ".png", true
	case "webp":
		return "image/webp", ".webp", true
	default:
		return "", "", false
	}
}

func storageKey(extension string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return hex.EncodeToString(random) + extension, nil
}

func validStorageKey(key string) bool {
	if strings.ContainsAny(key, `/\`) || len(key) < 36 {
		return false
	}
	base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(key, ".jpg"), ".png"), ".webp")
	decoded, err := hex.DecodeString(base)
	return err == nil && len(decoded) == 16
}
