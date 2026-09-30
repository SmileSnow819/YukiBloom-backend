package media

import (
	"context"

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
	// PublicURL 返回云端模式下图片对象的公开访问地址。
	// 参数：key 是对象键。
	// 返回：公开对象地址。
	PublicURL(key string) string
}

// NewStorage 根据服务配置创建 COS 图片存储。
// 参数：cfg 是已校验的服务配置。
// 返回：COS 图片对象存储；COS 客户端初始化失败时返回错误。
func NewStorage(cfg config.Config) (ObjectStorage, error) {
	return newCOSStorage(cfg)
}
