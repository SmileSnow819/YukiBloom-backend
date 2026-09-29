package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Config struct {
	Port             string
	DatabaseURL      string
	CookieSecure     bool
	UploadDir        string
	MediaStorage     string
	COSBucket        string
	COSRegion        string
	COSSecretID      string
	COSSecretKey     string
	COSPublicBaseURL string
}

// Load 从环境变量读取并校验服务配置。
// 参数：getenv 是按变量名读取环境变量的函数。
// 返回：校验后的 Config；配置无效时返回错误。
func Load(getenv func(string) string) (Config, error) {
	databaseURL := strings.TrimSpace(getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL 不能为空")
	}
	uploadDir := strings.TrimSpace(getenv("UPLOAD_DIR"))
	if uploadDir == "" {
		uploadDir = "./var/uploads"
	}
	mediaStorage := strings.TrimSpace(getenv("MEDIA_STORAGE"))
	if mediaStorage == "" {
		mediaStorage = "local"
	}
	if mediaStorage != "local" && mediaStorage != "cos" {
		return Config{}, errors.New("MEDIA_STORAGE 必须是 local 或 cos")
	}
	cosBucket := strings.TrimSpace(getenv("COS_BUCKET"))
	cosRegion := strings.TrimSpace(getenv("COS_REGION"))
	cosSecretID := strings.TrimSpace(getenv("COS_SECRET_ID"))
	cosSecretKey := strings.TrimSpace(getenv("COS_SECRET_KEY"))
	cosPublicURL := strings.TrimRight(strings.TrimSpace(getenv("COS_PUBLIC_BASE_URL")), "/")
	if mediaStorage == "cos" {
		for _, item := range []struct{ name, value string }{
			{"COS_BUCKET", cosBucket},
			{"COS_REGION", cosRegion},
			{"COS_SECRET_ID", cosSecretID},
			{"COS_SECRET_KEY", cosSecretKey},
		} {
			if item.value == "" {
				return Config{}, fmt.Errorf("使用 COS 图片存储时 %s 不能为空", item.name)
			}
		}
		if cosPublicURL != "" {
			parsedURL, err := url.Parse(cosPublicURL)
			if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
				return Config{}, errors.New("COS_PUBLIC_BASE_URL 必须是有效的 HTTPS 地址，且不能包含查询参数或片段")
			}
		}
	}

	port := strings.TrimSpace(getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return Config{}, fmt.Errorf("PORT 必须是 1 到 65535 之间的数字")
	}

	cookieSecure := true
	if raw := strings.TrimSpace(getenv("COOKIE_SECURE")); raw != "" {
		cookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE 必须是 true 或 false")
		}
	}

	return Config{
		Port: port, DatabaseURL: databaseURL, CookieSecure: cookieSecure, UploadDir: uploadDir,
		MediaStorage: mediaStorage, COSBucket: cosBucket, COSRegion: cosRegion,
		COSSecretID: cosSecretID, COSSecretKey: cosSecretKey, COSPublicBaseURL: cosPublicURL,
	}, nil
}
