package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/middleware"
	"github.com/ksm/docflow/internal/service"
)

type DocumentHandler struct {
	svc *service.DocumentService
}

func NewDocumentHandler(svc *service.DocumentService) *DocumentHandler {
	return &DocumentHandler{svc: svc}
}

func (h *DocumentHandler) List(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	role := middleware.CurrentRole(c)
	deptID, _ := middleware.CurrentDepartment(c)
	var deptPtr *int64
	if deptID != 0 {
		deptPtr = &deptID
	}
	f := service.ListDocsFilter{
		RoleView: c.Query("role_view"),
		UserID:   uid,
		UserRole: role,
		UserDept: deptPtr,
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

func (h *DocumentHandler) Publish(c *gin.Context) {
	var in service.PublishInput
	if !bindJSON(c, &in) {
		return
	}
	role := middleware.CurrentRole(c)
	uid := middleware.CurrentUserID(c)
	deptID, hasDept := middleware.CurrentDepartment(c)
	var deptPtr *int64
	if hasDept {
		d := deptID
		deptPtr = &d
	}
	doc, err := h.svc.Publish(uid, role, deptPtr, in)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *DocumentHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	uid := middleware.CurrentUserID(c)
	role := middleware.CurrentRole(c)
	d, err := h.svc.GetDetail(id, uid, role)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *DocumentHandler) Recall(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	role := middleware.CurrentRole(c)
	uid := middleware.CurrentUserID(c)
	if err := h.svc.Recall(id, uid, role); err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
