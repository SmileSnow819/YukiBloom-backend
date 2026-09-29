package media

import (
	"context"
	"fmt"

	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
)

// ObjectStorage 定义图片对象的保存、删除和公开访问方式。
// 参数：无。
// 返回：无。
type ObjectStorage interface {
	// Put 保存经过校验的图片对象。
	// 参数：ctx 控制对象存储请求；key 是对象键；data 是图片字节；contentType 是图片 MIME 类型。
	// 返回：error；上传失败时返回对象存储错误。
	Put(ctx context.Context, key string, data []byte, contentType string) error
	// Delete 删除指定存储键对应的图片对象。
	// 参数：ctx 控制对象存储请求；key 是对象键。
	// 返回：error；删除失败时返回对象存储错误。
	Delete(ctx context.Context, key string) error
	// LocalPath 返回本地模式下图片对象的完整文件路径。
	// 参数：key 是对象键。
	// 返回：本地文件路径；云端存储返回空字符串。
	LocalPath(key string) string
	// PublicURL 返回云端模式下图片对象的公开访问地址。
	// 参数：key 是对象键。
	// 返回：公开对象地址；本地存储返回空字符串。
	PublicURL(key string) string
}

// NewStorage 根据服务配置创建本地或 COS 图片存储。
// 参数：cfg 是已校验的服务配置。
// 返回：图片对象存储；配置不支持或 COS 客户端初始化失败时返回错误。
func NewStorage(cfg config.Config) (ObjectStorage, error) {
	switch cfg.MediaStorage {
	case "", "local":
		return localStorage{root: cfg.UploadDir}, nil
	case "cos":
		return newCOSStorage(cfg)
	default:
		return nil, fmt.Errorf("不支持的图片存储类型：%s", cfg.MediaStorage)
	}
}
