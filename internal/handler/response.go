package handler

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	writeResponse(w, status, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

func Error(w http.ResponseWriter, status int, code int, message string) {
	writeResponse(w, status, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func ErrorWithData(w http.ResponseWriter, status int, code int, message string, data interface{}) {
	writeResponse(w, status, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func writeResponse(w http.ResponseWriter, status int, response Response) {
	payload, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
