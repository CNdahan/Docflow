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
	res, err := h.svc.DocumentOverview(id)
	if err != nil {
		abortWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}
