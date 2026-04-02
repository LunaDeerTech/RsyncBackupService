package handler

import (
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	dashboardService *service.DashboardService
}

func NewSystemHandler(dashboardService *service.DashboardService) *SystemHandler {
	return &SystemHandler{dashboardService: dashboardService}
}

func (h *SystemHandler) Status(c *gin.Context) {
	if h.dashboardService == nil {
		writeError(c, http.StatusInternalServerError, "dashboard service unavailable")
		return
	}

	status, err := h.dashboardService.GetSystemStatus(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "load system status failed")
		return
	}

	c.JSON(http.StatusOK, status)
}

func (h *SystemHandler) Dashboard(c *gin.Context) {
	if h.dashboardService == nil {
		writeError(c, http.StatusInternalServerError, "dashboard service unavailable")
		return
	}

	summary, err := h.dashboardService.GetDashboard(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "load dashboard failed")
		return
	}

	c.JSON(http.StatusOK, summary)
}
