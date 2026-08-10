package main

import (
	"io"
	"net/http"
)

// API represents a single instance of the REST API.
type API struct {
	processor *EmailProcessor
}

// NewAPI creates a new REST API instance.
func NewAPI(processor *EmailProcessor) *API {
	return &API{processor: processor}
}

// EmailRequestHandler handles email notification requests.
func (a *API) EmailRequestHandler(w http.ResponseWriter, r *http.Request) {
	hlog := log.WithContext(r.Context()).WithField("transport", "http")
	if r.URL.Path != "/" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("A service that handles email-requests and send out emails to users.")); err != nil {
			hlog.Errorf("failed to send response: %s", err)
		}
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			hlog.Errorf("failed to read request body: %s", err)
			JSONError(w, r, "failed to read request body", http.StatusInternalServerError)
			return
		}
		if err := a.processor.Process(r.Context(), body); err != nil {
			hlog.Error(err)
			code := getErrorResponseCode(err)
			msg := err.Error()
			// Server-side failures (SMTP dial errors, template bugs) can carry internal
			// host/path details; log them but keep them out of the response body.
			if code >= http.StatusInternalServerError {
				msg = "failed to process the email request; see the de-mailer logs for details"
			}
			JSONError(w, r, msg, code)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("Request processed successfully.")); err != nil {
			hlog.Errorf("failed to send response: %s", err)
		}
	default:
		hlog.Errorf("unsupported request method %s", r.Method)
		JSONError(w, r, "Unsupported request method.", http.StatusMethodNotAllowed)
	}
}
