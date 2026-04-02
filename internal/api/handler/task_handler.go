package handler

import (
	"errors"
	"net/http"
	"strings"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type TaskHandler struct {
	executorService *service.ExecutorService
}

func NewTaskHandler(executorService *service.ExecutorService) *TaskHandler {
	return &TaskHandler{executorService: executorService}
}

func (h *TaskHandler) ListRunning(c *gin.Context) {
	if h.executorService == nil {
		writeError(c, http.StatusInternalServerError, "task service unavailable")
		return
	}

	c.JSON(http.StatusOK, h.executorService.ListRunningTasks())
}

func (h *TaskHandler) Cancel(c *gin.Context) {
	if h.executorService == nil {
		writeError(c, http.StatusInternalServerError, "task service unavailable")
		return
	}

	taskID := strings.TrimSpace(c.Param("id"))
	if taskID == "" {
		writeError(c, http.StatusBadRequest, "invalid task id")
		return
	}

	if err := h.executorService.CancelTask(taskID); err != nil {
		switch {
		case errors.Is(err, executorpkg.ErrTaskNotFound):
			writeError(c, http.StatusNotFound, "running task not found")
		default:
			writeError(c, http.StatusInternalServerError, "cancel task failed")
		}
		return
	}

	c.Status(http.StatusAccepted)
}
