package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

func currentAuthUser(c *gin.Context) (service.AuthIdentity, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return service.AuthIdentity{}, false
	}

	return user, true
}

func decodeExcludePatterns(encodedPatterns string) []string {
	if strings.TrimSpace(encodedPatterns) == "" {
		return []string{}
	}

	var excludePatterns []string
	if err := json.Unmarshal([]byte(encodedPatterns), &excludePatterns); err != nil {
		return []string{}
	}

	return excludePatterns
}

func formatOptionalHTTPTime(value *time.Time) *string {
	if value == nil {
		return nil
	}

	formattedValue := value.UTC().Format(http.TimeFormat)
	return &formattedValue
}