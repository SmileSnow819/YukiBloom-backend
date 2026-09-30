package apiresponse

import "net/http"

const (
	// CodeInvalidRequest 表示参数或请求内容错误。
	CodeInvalidRequest = 10001
	// CodeUnauthenticated 表示用户未登录或会话无效。
	CodeUnauthenticated = 10002
	// CodeForbidden 表示无权执行操作或 CSRF 校验失败。
	CodeForbidden = 10003
	// CodeNotFound 表示请求的资源不存在。
	CodeNotFound = 10004
	// CodeConflict 表示内容版本冲突或唯一标识已被占用。
	CodeConflict = 10005
	// CodePayloadTooLarge 表示请求内容超过大小限制。
	CodePayloadTooLarge = 10006
	// CodeRateLimited 表示请求频率超过允许范围。
	CodeRateLimited = 10007
	// CodeMethodNotAllowed 表示接口不支持当前请求方法。
	CodeMethodNotAllowed = 10008
	// CodeInternalError 表示服务内部错误。
	CodeInternalError = 50000
)

// ErrorKind 标识需要返回给客户端的稳定错误类别。
type ErrorKind uint8

const (
	// InvalidRequest 表示参数或请求内容错误。
	InvalidRequest ErrorKind = iota + 1
	// Unauthenticated 表示用户未登录或会话无效。
	Unauthenticated
	// Forbidden 表示无权执行操作或 CSRF 校验失败。
	Forbidden
	// NotFound 表示请求的资源不存在。
	NotFound
	// Conflict 表示内容版本冲突或唯一标识已被占用。
	Conflict
	// PayloadTooLarge 表示请求内容超过大小限制。
	PayloadTooLarge
	// RateLimited 表示请求频率超过允许范围。
	RateLimited
	// MethodNotAllowed 表示接口不支持当前请求方法。
	MethodNotAllowed
	// InternalError 表示服务内部错误。
	InternalError
)

// ErrorDefinition 描述某类 API 错误对应的业务码、HTTP 状态和默认文案。
type ErrorDefinition struct {
	HTTPStatus int
	Code       int
	Message    string
}

// errorDefinition 获取错误类别对应的集中定义。
// 参数：kind 是需要转换的稳定错误类别。
// 返回：ErrorDefinition 包含 HTTP 状态码、业务码和默认中文文案；未知类别按服务内部错误处理。
func errorDefinition(kind ErrorKind) ErrorDefinition {
	switch kind {
	case InvalidRequest:
		return ErrorDefinition{HTTPStatus: http.StatusBadRequest, Code: CodeInvalidRequest, Message: "请求参数或内容无效"}
	case Unauthenticated:
		return ErrorDefinition{HTTPStatus: http.StatusUnauthorized, Code: CodeUnauthenticated, Message: "请先登录"}
	case Forbidden:
		return ErrorDefinition{HTTPStatus: http.StatusForbidden, Code: CodeForbidden, Message: "无权执行此操作"}
	case NotFound:
		return ErrorDefinition{HTTPStatus: http.StatusNotFound, Code: CodeNotFound, Message: "请求的内容不存在"}
	case Conflict:
		return ErrorDefinition{HTTPStatus: http.StatusConflict, Code: CodeConflict, Message: "内容已被其他操作修改，请刷新后重试"}
	case PayloadTooLarge:
		return ErrorDefinition{HTTPStatus: http.StatusRequestEntityTooLarge, Code: CodePayloadTooLarge, Message: "请求内容超过允许大小"}
	case RateLimited:
		return ErrorDefinition{HTTPStatus: http.StatusTooManyRequests, Code: CodeRateLimited, Message: "请求过于频繁，请稍后再试"}
	case MethodNotAllowed:
		return ErrorDefinition{HTTPStatus: http.StatusMethodNotAllowed, Code: CodeMethodNotAllowed, Message: "不支持此请求方式"}
	default:
		return ErrorDefinition{HTTPStatus: http.StatusInternalServerError, Code: CodeInternalError, Message: "服务暂时不可用，请稍后再试"}
	}
}
