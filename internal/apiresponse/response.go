// Package apiresponse 定义所有 JSON API 共用的响应结构与写入方法。
package apiresponse

import (
	"github.com/gin-gonic/gin"
)

const (
	// CodeSuccess 表示接口调用成功。
	CodeSuccess = 0
)

// Envelope 是 API JSON 响应的统一外层结构。
type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data" swaggertype:"object"`
}

// Success 使用统一结构写入成功响应。
// 参数：c 是当前 HTTP 请求上下文；status 是成功的 HTTP 状态码；data 是业务响应数据。
// 返回：无；响应写入 c.Writer。
func Success(c *gin.Context, status int, data any) {
	c.JSON(status, Envelope{Code: CodeSuccess, Message: "成功", Data: data})
}

// Failure 使用指定错误类别写入统一失败响应。
// 参数：c 是当前 HTTP 请求上下文；kind 是集中定义的错误类别；message 是供客户端展示的中文错误说明，可为空以使用默认文案。
// 返回：无；响应写入 c.Writer。
func Failure(c *gin.Context, kind ErrorKind, message string) {
	definition := errorDefinition(kind)
	if message != "" {
		definition.Message = message
	}
	c.JSON(definition.HTTPStatus, Envelope{Code: definition.Code, Message: definition.Message, Data: nil})
}

// Abort 写入统一错误响应并终止当前 Gin 中间件链。
// 参数：c 是当前 HTTP 请求上下文；kind 是集中定义的错误类别；message 是供客户端展示的中文错误说明，可为空以使用默认文案。
// 返回：无；响应写入 c.Writer，后续处理器不会继续执行。
func Abort(c *gin.Context, kind ErrorKind, message string) {
	Failure(c, kind, message)
	c.Abort()
}
