package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ksm/docflow/internal/auth"
)

const (
	ctxKeyUserID = "uid"
	ctxKeyRole   = "role"
	ctxKeyDept   = "dept"
	ctxKeyJTI    = "jti"
)

// JWT 解析请求头 Authorization: Bearer xxx,把 user_id / role 注入 context
func JWT(tm *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": "UNAUTHORIZED", "message": "缺少 Authorization 头",
			})
			return
		}
		tokenStr := strings.TrimPrefix(raw, "Bearer ")
		claims, err := tm.Parse(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": "UNAUTHORIZED", "message": "token 无效或已过期",
			})
			return
		}
		c.Set(ctxKeyUserID, claims.UserID)
		c.Set(ctxKeyRole, claims.Role)
		if claims.DepartmentID != nil {
			c.Set(ctxKeyDept, *claims.DepartmentID)
		}
		c.Set(ctxKeyJTI, claims.ID)
		c.Next()
	}
}

// CurrentUserID 从 gin context 取当前用户 ID,JWT 中间件后可用
func CurrentUserID(c *gin.Context) int64 {
	v, _ := c.Get(ctxKeyUserID)
	id, _ := v.(int64)
	return id
}

func CurrentRole(c *gin.Context) string {
	v, _ := c.Get(ctxKeyRole)
	s, _ := v.(string)
	return s
}

func CurrentDepartment(c *gin.Context) (int64, bool) {
	v, ok := c.Get(ctxKeyDept)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

func CurrentJTI(c *gin.Context) string {
	v, _ := c.Get(ctxKeyJTI)
	s, _ := v.(string)
	return s
}
