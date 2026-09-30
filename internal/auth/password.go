package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 使用 bcrypt 生成密码哈希。
// 参数：password 是待哈希的明文密码。
// 返回：密码哈希；密码为空或哈希失败时返回错误。
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("密码不能为空")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 比较明文密码与已存储的 bcrypt 哈希。
// 参数：hash 是已存储的密码哈希；password 是待验证的明文密码。
// 返回：密码是否匹配。
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
