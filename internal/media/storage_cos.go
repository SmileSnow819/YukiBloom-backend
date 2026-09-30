package media

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
	"github.com/tencentyun/cos-go-sdk-v5"
)

type cosStorage struct {
	client    *cos.Client
	publicURL string
}

// newCOSStorage 根据 COS 配置创建签名客户端和公开图片 URL 基址。
// 参数：cfg 是已校验的服务配置。
// 返回：COS 对象存储；访问域名无效时返回错误。
func newCOSStorage(cfg config.Config) (cosStorage, error) {
	endpoint := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.COSBucket, cfg.COSRegion)
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return cosStorage{}, fmt.Errorf("COS 访问域名配置无效：%w", err)
	}
	publicURL := strings.TrimRight(cfg.COSPublicBaseURL, "/")
	if publicURL == "" {
		publicURL = endpoint
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: endpointURL}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.COSSecretID,
			SecretKey: cfg.COSSecretKey,
		},
	})
	return cosStorage{client: client, publicURL: publicURL}, nil
}

// Put 使用 COS SDK 上传图片并写入 MIME 与长期缓存响应头。
// 参数：s 是 COS 存储；ctx 控制上传请求；key 是对象键；data 是图片字节；contentType 是图片 MIME 类型。
// 返回：error；COS 拒绝或上传失败时返回错误。
func (s cosStorage) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.Object.Put(ctx, key, bytes.NewReader(data), &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType:  contentType,
			CacheControl: "public, max-age=31536000, immutable",
		},
	})
	if err != nil {
		return fmt.Errorf("上传图片到 COS 失败：%w", err)
	}
	return nil
}

// Delete 删除指定 COS 图片对象。
// 参数：s 是 COS 存储；ctx 控制删除请求；key 是对象键。
// 返回：error；COS 删除失败时返回错误。
func (s cosStorage) Delete(ctx context.Context, key string) error {
	_, err := s.client.Object.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("删除 COS 图片失败：%w", err)
	}
	return nil
}

// PublicURL 根据对象键构造 COS 图片公开地址。
// 参数：s 是 COS 存储；key 是对象键。
// 返回：经过 URL 转义的 COS 对象访问地址。
func (s cosStorage) PublicURL(key string) string {
	return strings.TrimRight(s.publicURL, "/") + "/" + url.PathEscape(key)
}
