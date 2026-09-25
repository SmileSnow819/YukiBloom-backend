package httpserver

import (
	"log"
	"net/http"

	"github.com/SmileSnow819/YukiBloom-backend/internal/auth"
	"github.com/SmileSnow819/YukiBloom-backend/internal/importer"
	"github.com/SmileSnow819/YukiBloom-backend/internal/media"
	"github.com/SmileSnow819/YukiBloom-backend/internal/personal"
	"github.com/SmileSnow819/YukiBloom-backend/internal/posts"
	"github.com/SmileSnow819/YukiBloom-backend/internal/sitecontent"
	"github.com/gin-gonic/gin"
)

// NewRouter 注册健康检查、公开内容、管理员操作和图片访问路由。
// 参数：authHandler 管理登录和会话；postHandler 管理文章；mediaHandler 管理图片；importerHandler 处理 Markdown 导入；personalHandler 管理足迹和实习经历；siteHandler 管理站点内容与页面。
// 返回：配置完成的 Gin 路由引擎。
func NewRouter(authHandler *auth.Handler, postHandler *posts.Handler, mediaHandler *media.Handler, importerHandler *importer.Handler, personalHandler *personal.Handler, siteHandler *sitecontent.Handler) *gin.Engine {
	router := gin.New()
	// 不信任客户端提交的代理请求头，避免伪造 IP 绕过登录限流。
	if err := router.SetTrustedProxies(nil); err != nil {
		panic("配置受信任代理失败：" + err.Error())
	}
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
		if importerHandler != nil {
			admin.POST("/posts/markdown/preview", importerHandler.Preview)
			admin.POST("/posts/markdown", importerHandler.CreateDraft)
		}
		if mediaHandler != nil {
			admin.GET("/media", mediaHandler.List)
			admin.POST("/media", mediaHandler.Upload)
			admin.DELETE("/media/:id", mediaHandler.Delete)
		}
		if personalHandler != nil {
			admin.GET("/footprints", personalHandler.GetFootprints)
			admin.PUT("/footprints", personalHandler.ReplaceFootprints)
			admin.GET("/timeline", personalHandler.GetTimeline)
			admin.PUT("/timeline", personalHandler.ReplaceTimeline)
		}
		if siteHandler != nil {
			admin.GET("/site-content", siteHandler.AdminContent)
			admin.PUT("/site-content", siteHandler.ReplaceContent)
			admin.GET("/pages", siteHandler.AdminPages)
			admin.POST("/pages", siteHandler.SavePage)
			admin.GET("/pages/:id", siteHandler.AdminPageByID)
			admin.PATCH("/pages/:id", siteHandler.SavePage)
			admin.DELETE("/pages/:id", siteHandler.DeletePage)
			admin.POST("/pages/:id/publish", siteHandler.PublishPage)
			admin.POST("/pages/:id/unpublish", siteHandler.UnpublishPage)
		}
	}
	if postHandler != nil {
		router.GET("/api/v1/posts", postHandler.PublicList)
		router.GET("/api/v1/posts/:slug", postHandler.PublicBySlug)
	}
	if mediaHandler != nil {
		router.GET("/uploads/:key", mediaHandler.PublicFile)
	}
	if personalHandler != nil {
		router.GET("/api/v1/footprints", personalHandler.GetFootprints)
		router.GET("/api/v1/timeline", personalHandler.GetTimeline)
	}
	if siteHandler != nil {
		router.GET("/api/v1/site-content", siteHandler.PublicContent)
		router.GET("/api/v1/pages/:slug", siteHandler.PublicPage)
	}
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "不支持此请求方式"})
	})
	return router
}
