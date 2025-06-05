package server

import (
	"encoding/json"
	"go-stock-prediction/pkg/logger"
	"net/http"
)

type ResponseFailure struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func ResponseError(w http.ResponseWriter, status int, message string) {
	response := ResponseFailure{
		StatusCode: status,
		Message:    message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	bodyResponse, err := json.Marshal(response)
	if err != nil {
		logger.Logger.Error("Failed to marshal error response", "error", err)
		return
	}

	w.Write(bodyResponse)
}

func ResponseSuccess(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		bodyResponse, err := json.Marshal(data)
		if err != nil {
			logger.Logger.Error("Failed to marshal success response", "error", err)
			ResponseError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		w.Write(bodyResponse)
	}
}
