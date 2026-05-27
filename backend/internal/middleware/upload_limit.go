package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// UploadLimit 限制单次请求体最大字节数,超出 Gin 直接返回 413
func UploadLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
