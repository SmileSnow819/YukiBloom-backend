package httpserver

import (
	"net/http"

	"github.com/SmileSnow819/YukiBloom-backend/internal/auth"
	"github.com/gin-gonic/gin"
)

func NewRouter(authHandler *auth.Handler) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	if authHandler != nil {
		router.POST("/api/v1/admin/login", authHandler.Login)
		admin := router.Group("/api/v1/admin", authHandler.RequireSession())
		admin.GET("/session", authHandler.Session)
		admin.POST("/logout", authHandler.Logout)
	}
	return router
}
