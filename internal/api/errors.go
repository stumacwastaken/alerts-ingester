package api

import (
	"errors"
	"net/http"
)

func ResponseFromError(w http.ResponseWriter, r *http.Request, err error) {
	var apiError APIError
	w.Header().Set("Content-Type", "application/json")
	if errors.As(err, &apiError) {
		http.Error(w, apiError.Error(), apiError.StatusCode())

	} else {
		http.Error(w, "{'error': 'internal service error'}", 500)
	}
}

type APIError interface {
	Error() string
	StatusCode() int
}
