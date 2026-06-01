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

	// 部门 + 用户: super + dept (service 层控制细粒度权限)
	adminGroup := authed.Group("")
	adminGroup.Use(middleware.RequireRole(model.RoleSuper, model.RoleDept))
	{
		adminGroup.GET("/departments", h.Department.List)
		adminGroup.GET("/users", h.User.List)
		adminGroup.POST("/users", h.User.Create)
		adminGroup.PATCH("/users/:id", h.User.Update)
		adminGroup.POST("/users/:id/reset-password", h.User.ResetPassword)
		adminGroup.GET("/users/export", h.User.Export)
		adminGroup.GET("/users/export-template", h.User.ExportTemplate)
		adminGroup.POST("/users/import", h.User.Import)
	}

	// 部门管理写操作: super only
	superGroup := authed.Group("")
	superGroup.Use(middleware.RequireRole(model.RoleSuper))
	{
		superGroup.POST("/departments", h.Department.Create)
		superGroup.PATCH("/departments/:id", h.Department.Update)
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
		publish.PATCH("/documents/:id", h.Document.Update)
		publish.POST("/documents/:id/recall", h.Document.Recall)
	}

	// 修订日志 (super + dept)
	authed.GET("/documents/:id/revisions", h.Document.ListRevisions)

	// 上报
	authed.POST("/submissions/:id", h.Submission.Submit)         // :id 是 document_id
	authed.GET("/submissions/mine", h.Submission.ListMine)
	authed.GET("/submissions/:id/detail", h.Submission.Detail)
	authed.POST("/submissions/:id/return", h.Submission.Return)

	// 统计
	authed.GET("/stats/documents/:id", h.Stats.DocumentOverview)
	authed.GET("/stats/documents/:id/export", h.Stats.ExportDocumentOverview)
	authed.GET("/stats/departments/:id", h.Stats.DepartmentOverview)
	statsSuper := authed.Group("/stats")
	statsSuper.Use(middleware.RequireRole(model.RoleSuper))
	{
		statsSuper.GET("/global", h.Stats.GlobalOverview)
	}

	return r
}
