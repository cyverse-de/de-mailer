package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// copied from https://gitlab.com/sonoran.sarah/cacao/-/blob/master/api-service/utils/error.go#L13-35
// HTTPError is an error that includes an HTTP response code.
type HTTPError struct {
	code    int
	message string
}

// Code returns the HTTP response code corresponding to the error.
func (h *HTTPError) Code() int {
	return h.code
}

// Error returns the message corresponding to the error.
func (h *HTTPError) Error() string {
	return h.message
}

// NewHTTPError returns a new HTTP error with the given status code and (optionally formatted) message.
func NewHTTPError(code int, format string, args ...any) *HTTPError {
	return &HTTPError{
		code:    code,
		message: fmt.Sprintf(format, args...),
	}
}

// ErrorStatus is the struct/json object that is marshalled for every error response per CACAO's openapi spec
type ErrorStatus struct {
	Timestamp string `json:"timestamp"`
	ErrorType string `json:"error"`
	Message   string `json:"message"`
}

// JSONError passthrough
func JSONError(w http.ResponseWriter, r *http.Request, errorMsg string, code int) {
	errorObj := new(ErrorStatus)
	errorObj.Timestamp = time.Now().UTC().String()
	errorObj.Message = errorMsg

	w.Header().Add("Content-Type", "application/json")
	b, err := json.Marshal(errorObj)
	if err == nil {
		w.WriteHeader(code)
		w.Write(b) // nolint:errcheck
	} else { // something very bad happened and we shouldn't hit this point
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("could not fulfill request and return a result; please notify site admin")) // nolint:errcheck
	}
}
