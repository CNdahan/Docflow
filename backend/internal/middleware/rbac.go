package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole 角色级粗校验,精细资源校验留给 service 层
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role := CurrentRole(c)
		if _, ok := allowed[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": "FORBIDDEN", "message": "无权限",
			})
			return
		}
		c.Next()
	}
}
