package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ksm/docflow/internal/service"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) List(c *gin.Context) {
	f := service.ListUsersFilter{
		Role: c.Query("role"),
	}
	if v := c.Query("department_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "department_id 非法"})
			return
		}
		f.DepartmentID = &id
	}
	if v := c.Query("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := c.Query("size"); v != "" {
		f.Size, _ = strconv.Atoi(v)
	}
	res, err := h.svc.List(f)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *UserHandler) Create(c *gin.Context) {
	var in service.CreateUserInput
	if !bindJSON(c, &in) {
		return
	}
	u, err := h.svc.Create(in)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "id 非法"})
		return
	}
	var in service.UpdateUserInput
	if !bindJSON(c, &in) {
		return
	}
	u, err := h.svc.Update(id, in)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, u)
}

type resetPwdReq struct {
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "id 非法"})
		return
	}
	var req resetPwdReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.ResetPassword(id, req.NewPassword); err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
