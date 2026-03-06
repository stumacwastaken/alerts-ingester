package api

import (
	"errors"
	"fmt"
	"net/http"
)

type APIError interface {
	Error() string
	StatusCode() int
}

func ResponseFromError(w http.ResponseWriter, r *http.Request, err error) {
	var apiError APIError
	w.Header().Set("Content-Type", "application/json")
	if errors.As(err, &apiError) {
		w.WriteHeader(apiError.StatusCode())
		w.Write([]byte(apiError.Error()))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(apiError.Error()))
	}
}

func InternalServerError(msg string) error {
	return &apiError{
		msg:  msg,
		code: http.StatusInternalServerError,
	}
}

// for internal use to send 500 errors where it makes sense to
type apiError struct {
	msg  string
	code int
}

func (a *apiError) Error() string {
	return fmt.Sprintf(`{"error": "%s", "code": %d}`, a.msg, a.code)
}

func (a *apiError) StatusCode() int {
	return a.code
}
