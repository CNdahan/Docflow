package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/middleware"
	"github.com/ksm/docflow/internal/service"
)

type SubmissionHandler struct {
	svc *service.SubmissionService
}

func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

type submitReq struct {
	Note          string  `json:"note"`
	AttachmentIDs []int64 `json:"attachment_ids" binding:"required,min=1"`
}

func (h *SubmissionHandler) Submit(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "document_id 非法"))
		return
	}
	var req submitReq
	if !bindJSON(c, &req) {
		return
	}
	uid := middleware.CurrentUserID(c)
	sub, err := h.svc.Submit(service.SubmitInput{
		DocumentID:    docID,
		UserID:        uid,
		Note:          req.Note,
		AttachmentIDs: req.AttachmentIDs,
	})
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *SubmissionHandler) ListMine(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	f := service.ListMyFilter{
		UserID: uid,
		Status: c.Query("status"),
	}
	if v := c.Query("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := c.Query("size"); v != "" {
		f.Size, _ = strconv.Atoi(v)
	}
	items, total, err := h.svc.ListMine(f)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *SubmissionHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	uid := middleware.CurrentUserID(c)
	role := middleware.CurrentRole(c)
	deptID, hasDept := middleware.CurrentDepartment(c)
	var deptPtr *int64
	if hasDept {
		d := deptID
		deptPtr = &d
	}
	dto, err := h.svc.GetDetail(id, uid, role, deptPtr)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto)
}

type returnReq struct {
	Reason string `json:"reason" binding:"required,min=5,max=500"`
}

func (h *SubmissionHandler) Return(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	var req returnReq
	if !bindJSON(c, &req) {
		return
	}
	uid := middleware.CurrentUserID(c)
	role := middleware.CurrentRole(c)
	deptID, hasDept := middleware.CurrentDepartment(c)
	var deptPtr *int64
	if hasDept {
		d := deptID
		deptPtr = &d
	}
	sub, err := h.svc.Return(id, req.Reason, uid, role, deptPtr)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, sub)
}
