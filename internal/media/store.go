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
	"strings"
	"time"

	"github.com/gen2brain/jpegn"
	webpencoder "github.com/gen2brain/webp"
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
	pool    *pgxpool.Pool
	storage ObjectStorage
}

// NewStore 创建使用指定对象存储的图片存储。
// 参数：pool 是数据库连接池；storage 是 COS 图片对象存储。
// 返回：配置好的 Store。
func NewStore(pool *pgxpool.Pool, storage ObjectStorage) *Store {
	return &Store{pool: pool, storage: storage}
}

// Save 校验图片、保存文件并登记图片信息。
// 参数：s 是图片存储；ctx 控制数据库操作；source 是图片数据流。
// 返回：已保存的 Item；读取、校验或保存失败时返回错误。
func (s *Store) Save(ctx context.Context, source io.Reader) (Item, error) {
	item, _, err := s.save(ctx, s.pool, source)
	return item, err
}

// SaveInTx 在调用方的事务中登记图片，并返回存储键供事务回滚时清理对象。
// 参数：s 是图片存储；ctx 控制操作；tx 是调用方事务；source 是图片数据流。
// 返回：已保存的图片、对象存储键，以及保存错误。
func (s *Store) SaveInTx(ctx context.Context, tx pgx.Tx, source io.Reader) (Item, string, error) {
	return s.save(ctx, tx, source)
}

// save 校验并保存图片对象，再通过指定查询器登记元数据。
// 参数：s 是图片存储；ctx 控制操作；query 是连接池或事务；source 是图片数据流。
// 返回：图片记录、对象存储键，以及保存错误。
func (s *Store) save(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, source io.Reader) (Item, string, error) {
	data, info, err := readImage(source)
	if err != nil {
		return Item{}, "", err
	}
	key, err := storageKey(info.Extension)
	if err != nil {
		return Item{}, "", fmt.Errorf("生成图片文件名失败：%w", err)
	}
	if err := s.storage.Put(ctx, key, data, info.MIMEType); err != nil {
		return Item{}, "", err
	}
	item, err := s.saveImageMetadata(ctx, query, key, info, int64(len(data)))
	if err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if cleanupErr := s.storage.Delete(cleanupCtx, key); cleanupErr != nil {
			return Item{}, key, fmt.Errorf("%w；清理未登记图片失败：%v", err, cleanupErr)
		}
		return Item{}, "", err
	}
	item.URL = s.storage.PublicURL(key)
	return item, key, nil
}

// RemoveObject 删除尚未提交媒体元数据的图片对象，用于导入回滚。
// 参数：s 是图片存储；ctx 控制对象存储请求；key 是待删除的存储键。
// 返回：error；删除失败时返回对象存储错误。
func (s *Store) RemoveObject(ctx context.Context, key string) error {
	if !validStorageKey(key) {
		return ErrNotFound
	}
	return s.storage.Delete(ctx, key)
}

// readImage 限制读取大小、校验图片内容，并将 JPEG 或 PNG 转为 WebP。
// 参数：source 是上传图片的数据流。
// 返回：待保存的 WebP 字节、格式和尺寸信息；读取、校验或编码失败时返回对应错误。
func readImage(source io.Reader) ([]byte, ImageInfo, error) {
	data, err := io.ReadAll(io.LimitReader(source, MaxUploadBytes+1))
	if err != nil {
		return nil, ImageInfo{}, fmt.Errorf("读取图片失败：%w", err)
	}
	if len(data) > MaxUploadBytes {
		return nil, ImageInfo{}, ErrTooLarge
	}
	info, err := Validate(data)
	if err != nil {
		return nil, ImageInfo{}, err
	}
	if info.MIMEType == "image/webp" {
		return data, info, nil
	}
	var decoded image.Image
	if info.MIMEType == "image/jpeg" {
		decoded, err = jpegn.Decode(bytes.NewReader(data), &jpegn.Options{AutoRotate: true})
	} else {
		decoded, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, ImageInfo{}, ErrInvalidImage
	}
	var output bytes.Buffer
	if err := webpencoder.Encode(&output, decoded, webpencoder.Options{Quality: 80, Method: 4}); err != nil {
		return nil, ImageInfo{}, fmt.Errorf("图片转换为 WebP 失败：%w", err)
	}
	if output.Len() > MaxUploadBytes {
		return nil, ImageInfo{}, ErrTooLarge
	}
	info.MIMEType = "image/webp"
	info.Extension = ".webp"
	info.Width = decoded.Bounds().Dx()
	info.Height = decoded.Bounds().Dy()
	return output.Bytes(), info, nil
}

// saveImageMetadata 将图片文件信息写入 media 表。
// 参数：ctx 控制数据库操作；query 是连接池或事务；key 是图片存储键；info 是已校验的图片信息；size 是文件字节数。
// 返回：数据库生成的图片记录；写入失败时返回错误。
func (s *Store) saveImageMetadata(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, key string, info ImageInfo, size int64) (Item, error) {
	var item Item
	err := query.QueryRow(ctx, `INSERT INTO media (storage_key, mime_type, size_bytes, width, height)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, storage_key, mime_type, size_bytes, width, height, created_at`,
		key, info.MIMEType, size, info.Width, info.Height).Scan(&item.ID, &key, &item.MIMEType, &item.SizeBytes, &item.Width, &item.Height, &item.CreatedAt)
	if err != nil {
		return Item{}, fmt.Errorf("保存图片信息失败：%w", err)
	}
	return item, nil
}

type ImageInfo struct {
	MIMEType  string
	Extension string
	Width     int
	Height    int
}

// Validate 检查图片大小、格式及像素尺寸。
// 参数：data 是待校验的图片字节。
// 返回：图片格式和尺寸信息；校验失败时返回错误。
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

// List 分页查询图片记录及总数。
// 参数：s 是图片存储；ctx 控制数据库操作；page 是页码；limit 是每页数量。
// 返回：当前页的 Item 列表、记录总数，以及查询错误。
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
		item.URL = s.storage.PublicURL(key)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// PublicURLsByIDs 批量查询媒体 ID 对应的公开 COS 地址。
// 参数：s 是图片存储；ctx 控制数据库操作；ids 是媒体记录 ID 列表。
// 返回：媒体 ID 到公开地址的映射；查询失败时返回错误。
func (s *Store) PublicURLsByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	urls := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return urls, nil
	}
	rows, err := s.pool.Query(ctx, "SELECT id, storage_key FROM media WHERE id::text = ANY($1::text[])", ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, err
		}
		urls[id] = s.storage.PublicURL(key)
	}
	return urls, rows.Err()
}

// DeleteUnused 删除未被内容引用的图片记录及文件。
// 参数：s 是图片存储；ctx 控制数据库操作；id 是待删除的图片 ID。
// 返回：图片不存在、仍被引用或删除失败时的错误，成功时为 nil。
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
		OR EXISTS(SELECT 1 FROM categories WHERE position($2 in image)>0)
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
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cleanupCancel()
	if err := s.storage.Delete(cleanupCtx, key); err != nil {
		return fmt.Errorf("图片记录已删除，但清理图片对象失败：%w", err)
	}
	return nil
}

// imageType 将解码格式映射为图片 MIME 类型和扩展名。
// 参数：format 是图片解码格式。
// 返回：MIME 类型、文件扩展名，以及格式是否受支持。
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

// storageKey 生成带扩展名的随机图片存储键。
// 参数：extension 是图片文件扩展名。
// 返回：随机存储键；随机数读取失败时返回错误。
func storageKey(extension string) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return hex.EncodeToString(random) + extension, nil
}

// validStorageKey 检查图片存储键的路径安全性和格式。
// 参数：key 是待检查的存储键。
// 返回：存储键是否有效。
func validStorageKey(key string) bool {
	if strings.ContainsAny(key, `/\`) || len(key) < 36 {
		return false
	}
	base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(key, ".jpg"), ".png"), ".webp")
	decoded, err := hex.DecodeString(base)
	return err == nil && len(decoded) == 16
}
