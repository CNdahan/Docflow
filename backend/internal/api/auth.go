package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ksm/docflow/internal/auth"
	"github.com/ksm/docflow/internal/middleware"
	"github.com/ksm/docflow/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
	tm  *auth.TokenManager
}

func NewAuthHandler(svc *service.AuthService, tm *auth.TokenManager) *AuthHandler {
	return &AuthHandler{svc: svc, tm: tm}
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.Login(req.Username, req.Password)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.svc.Refresh(req.RefreshToken)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	jti := middleware.CurrentJTI(c)
	if jti != "" {
		// 在黑名单中保留到 access token 自然过期
		h.tm.Revoke(jti, time.Now().Add(2*time.Hour))
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
