package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	errs "github.com/ksm/docflow/internal/errors"
	"github.com/ksm/docflow/internal/service"
)

type StatsHandler struct {
	svc *service.StatsService
}

func NewStatsHandler(svc *service.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

func (h *StatsHandler) DocumentOverview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	f := service.DocOverviewFilter{DocID: id, Status: c.Query("status")}
	if v := c.Query("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := c.Query("size"); v != "" {
		f.Size, _ = strconv.Atoi(v)
	}
	res, err := h.svc.DocumentOverview(f)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *StatsHandler) ExportDocumentOverview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	f, err := h.svc.ExportDocumentOverview(id)
	if err != nil {
		abortWithError(c, err)
		return
	}
	defer f.Close()
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename=overview.xlsx")
	f.Write(c.Writer)
}

func (h *StatsHandler) GlobalOverview(c *gin.Context) {
	res, err := h.svc.GlobalOverview()
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *StatsHandler) DepartmentOverview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		abortWithError(c, errs.New(http.StatusBadRequest, "BAD_REQUEST", "id 非法"))
		return
	}
	res, err := h.svc.DepartmentOverview(id)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
