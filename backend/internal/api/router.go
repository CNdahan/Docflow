package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/config"
	"github.com/ksm/docflow/internal/middleware"
	"github.com/ksm/docflow/internal/model"
)

type Handlers struct {
	Auth        *AuthHandler
	Department  *DepartmentHandler
	User        *UserHandler
	Attachment  *AttachmentHandler
	Document    *DocumentHandler
	Submission  *SubmissionHandler
	Stats       *StatsHandler
}

func BuildRouter(cfg *config.Config, tm *auth.TokenManager, h Handlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(gin.Logger())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           600 * 1e9,
	}))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	api := r.Group("/api/v1")
	api.POST("/auth/login", h.Auth.Login)
	api.POST("/auth/refresh", h.Auth.Refresh)

	authed := api.Group("")
	authed.Use(middleware.JWT(tm))
	authed.Use(middleware.UploadLimit(cfg.Storage.MaxFileSize + 1024*1024))

	authed.POST("/auth/logout", h.Auth.Logout)

	// 部门 + 用户: super
	superGroup := authed.Group("")
	superGroup.Use(middleware.RequireRole(model.RoleSuper))
	{
		superGroup.GET("/departments", h.Department.List)
		superGroup.POST("/departments", h.Department.Create)
		superGroup.PATCH("/departments/:id", h.Department.Update)

		superGroup.GET("/users", h.User.List)
		superGroup.POST("/users", h.User.Create)
		superGroup.PATCH("/users/:id", h.User.Update)
		superGroup.POST("/users/:id/reset-password", h.User.ResetPassword)
	}

	// 附件: 所有登录用户都可上传 (后端按 owner_type 校验语义)
	authed.POST("/attachments", h.Attachment.Upload)
	authed.GET("/attachments/:id/download", h.Attachment.Download)
	authed.GET("/attachments/:id/preview", h.Attachment.Preview)
	authed.DELETE("/attachments/:id", h.Attachment.Delete)

	// 公文
	authed.GET("/documents", h.Document.List)
	authed.GET("/documents/:id", h.Document.Detail)
	publish := authed.Group("")
	publish.Use(middleware.RequireRole(model.RoleSuper, model.RoleDept))
	{
		publish.POST("/documents", h.Document.Publish)
		publish.POST("/documents/:id/recall", h.Document.Recall)
	}

	// 上报
	authed.POST("/submissions/:id", h.Submission.Submit)         // :id 是 document_id
	authed.GET("/submissions/mine", h.Submission.ListMine)
	authed.GET("/submissions/:id/detail", h.Submission.Detail)
	authed.POST("/submissions/:id/return", h.Submission.Return)

	// 统计 (M1 仅暴露单公文纵览, 权限通过详情接口的鉴权间接保障)
	authed.GET("/stats/documents/:id", h.Stats.DocumentOverview)

	return r
}
