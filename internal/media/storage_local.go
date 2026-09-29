package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type localStorage struct{ root string }

// Put 将图片以原子文件写入方式保存到本地目录。
// 参数：s 是本地存储；key 是对象键；data 是图片字节；contentType 本地存储不使用。
// 返回：error；目录创建或文件写入失败时返回错误。
func (s localStorage) Put(_ context.Context, key string, data []byte, _ string) error {
	_, err := writeImageFile(s.root, key, data)
	return err
}

// Delete 删除本地图片文件；文件已不存在时视为成功。
// 参数：s 是本地存储；key 是对象键。
// 返回：error；删除文件失败时返回错误。
func (s localStorage) Delete(_ context.Context, key string) error {
	err := os.Remove(s.LocalPath(key))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除本地图片失败：%w", err)
	}
	return nil
}

// LocalPath 组合本地图片文件的完整路径。
// 参数：s 是本地存储；key 是对象键。
// 返回：图片文件完整路径。
func (s localStorage) LocalPath(key string) string { return filepath.Join(s.root, key) }

// PublicURL 表示本地存储没有外部对象地址。
// 参数：key 是对象键，本地存储不使用。
// 返回：空字符串，表示由 API 本地读取文件。
func (localStorage) PublicURL(string) string { return "" }
