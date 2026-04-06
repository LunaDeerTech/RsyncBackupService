package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggerRecordsRequestFields(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	testLogger := slog.New(slog.NewJSONHandler(&output, nil))
	slog.SetDefault(testLogger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recorder := httptest.NewRecorder()

	handler := Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	handler.ServeHTTP(recorder, req)

	logged := output.String()
	checks := []string{
		`"msg":"http request"`,
		`"method":"GET"`,
		`"path":"/api/v1/health"`,
		`"status":201`,
		`"duration":`,
	}

	for _, check := range checks {
		if !strings.Contains(logged, check) {
			t.Fatalf("log output %q does not contain %q", logged, check)
		}
	}
}
