package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmailRequestHandler(t *testing.T) {
	validBody := `{"template":"blank","subject":"s","to":"user@example.org","values":{"contents":"x"}}`

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		senderErr    error
		wantStatus   int
		wantBodyPart string
		rejectPart   string
		wantJSON     bool
	}{
		{
			name:         "GET banner",
			method:       http.MethodGet,
			path:         "/",
			wantStatus:   200,
			wantBodyPart: "A service that handles email-requests",
		},
		{
			name:         "POST valid request",
			method:       http.MethodPost,
			path:         "/",
			body:         validBody,
			wantStatus:   200,
			wantBodyPart: "Request processed successfully.",
		},
		{
			name:       "POST invalid JSON",
			method:     http.MethodPost,
			path:       "/",
			body:       `{not json`,
			wantStatus: 400,
			wantJSON:   true,
		},
		{
			// Regression tests: send failures used to be reported as 200, and 500 bodies
			// must stay generic — SMTP dial errors carry internal host details.
			name:         "POST with failing sender",
			method:       http.MethodPost,
			path:         "/",
			body:         validBody,
			senderErr:    errors.New("dial tcp 10.0.0.1:25: connection refused"),
			wantStatus:   500,
			wantJSON:     true,
			wantBodyPart: "see the de-mailer logs",
			rejectPart:   "10.0.0.1",
		},
		{
			name:       "unsupported method",
			method:     http.MethodPut,
			path:       "/",
			body:       validBody,
			wantStatus: 405,
			wantJSON:   true,
		},
		{
			name:       "unknown path",
			method:     http.MethodGet,
			path:       "/nope",
			wantStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &fakeSender{err: tt.senderErr}
			api := NewAPI(NewEmailProcessor(sender, testDESettings(), "noreply@example.org"))

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			recorder := httptest.NewRecorder()
			api.EmailRequestHandler(recorder, req)

			resp := recorder.Result()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("expected status %d, got %d (%s)", tt.wantStatus, resp.StatusCode, recorder.Body.String())
			}
			if tt.wantBodyPart != "" && !strings.Contains(recorder.Body.String(), tt.wantBodyPart) {
				t.Errorf("expected body to contain %q, got %q", tt.wantBodyPart, recorder.Body.String())
			}
			if tt.rejectPart != "" && strings.Contains(recorder.Body.String(), tt.rejectPart) {
				t.Errorf("expected body to not leak %q, got %q", tt.rejectPart, recorder.Body.String())
			}
			if tt.wantJSON {
				if got := resp.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("expected an application/json error body, got Content-Type %q", got)
				}
			}
		})
	}
}
