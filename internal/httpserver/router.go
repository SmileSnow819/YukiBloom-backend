package httpserver

import (
	"log"
	"net/http"

	"github.com/SmileSnow819/YukiBloom-backend/internal/auth"
	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
	"github.com/gin-gonic/gin"
)

func NewRouter(authHandler *auth.Handler, postHandler *posts.Handler) *gin.Engine {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("请求处理异常：%v", recovered)
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "服务发生异常，请稍后再试"})
				}
			}
		}()
		c.Next()
	})
	router.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "正常"})
	})
	if authHandler != nil {
		router.POST("/api/v1/admin/login", authHandler.Login)
		admin := router.Group("/api/v1/admin", authHandler.RequireSession())
		admin.GET("/session", authHandler.Session)
		admin.POST("/logout", authHandler.Logout)
		if postHandler != nil {
			admin.GET("/posts", postHandler.AdminList)
			admin.POST("/posts", postHandler.Create)
			admin.GET("/posts/:id", postHandler.AdminByID)
			admin.PATCH("/posts/:id", postHandler.Update)
			admin.POST("/posts/:id/publish", postHandler.Publish)
			admin.POST("/posts/:id/unpublish", postHandler.Unpublish)
		}
	}
	if postHandler != nil {
		router.GET("/api/v1/posts", postHandler.PublicList)
		router.GET("/api/v1/posts/:slug", postHandler.PublicBySlug)
	}
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "不支持此请求方式"})
	})
	return router
}
