package middleware

import (
	"encoding/json"
	"net/http"
)

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func writeError(w http.ResponseWriter, status int, code int, message string) {
	writeResponse(w, status, response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func writeResponse(w http.ResponseWriter, status int, payload response) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
