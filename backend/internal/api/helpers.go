package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	errs "github.com/ksm/docflow/internal/errors"
)

// abortWithError 把任意 error 翻译成统一 JSON 响应
func abortWithError(c *gin.Context, err error) {
	if ae, ok := errs.AsAppError(err); ok {
		c.AbortWithStatusJSON(ae.HTTPStatus, gin.H{
			"code":    ae.Code,
			"message": ae.Message,
		})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"code": "NOT_FOUND", "message": "资源不存在",
		})
		return
	}
	log.Error().Err(err).Str("path", c.Request.URL.Path).Msg("unhandled error")
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
		"code": "INTERNAL", "message": "服务内部错误",
	})
}

// bindJSON 解析请求 body,失败时统一 400
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"code": "BAD_REQUEST", "message": err.Error(),
		})
		return false
	}
	return true
}
