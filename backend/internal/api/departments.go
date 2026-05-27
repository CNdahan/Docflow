package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ksm/docflow/internal/service"
)

type DepartmentHandler struct {
	svc *service.DepartmentService
}

func NewDepartmentHandler(svc *service.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{svc: svc}
}

func (h *DepartmentHandler) List(c *gin.Context) {
	items, err := h.svc.List()
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

type createDeptReq struct {
	Name string `json:"name" binding:"required,min=1,max=64"`
}

func (h *DepartmentHandler) Create(c *gin.Context) {
	var req createDeptReq
	if !bindJSON(c, &req) {
		return
	}
	d, err := h.svc.Create(req.Name)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, d)
}

func (h *DepartmentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "BAD_REQUEST", "message": "id 非法"})
		return
	}
	var in service.DepartmentUpdate
	if !bindJSON(c, &in) {
		return
	}
	d, err := h.svc.Update(id, in)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}
