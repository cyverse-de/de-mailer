package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// EmailSender abstracts SMTP delivery so the processor can be tested without a mail server.
type EmailSender interface {
	Send(ctx context.Context, req *FormattedEmailRequest) error
}

// EmailProcessor turns a raw email-request payload into a sent email, independent of transport.
type EmailProcessor struct {
	sender      EmailSender
	deSettings  *DESettings
	fromAddress string
}

// NewEmailProcessor creates a new email request processor.
func NewEmailProcessor(sender EmailSender, deSettings *DESettings, fromAddress string) *EmailProcessor {
	return &EmailProcessor{
		sender:      sender,
		deSettings:  deSettings,
		fromAddress: fromAddress,
	}
}

// parseEmailRequest parses a raw email request payload and its template values.
func parseEmailRequest(body []byte) (EmailRequest, map[string]any, error) {
	var emailReq EmailRequest
	payloadMap := make(map[string]any)

	if err := json.Unmarshal(body, &emailReq); err != nil {
		return emailReq, payloadMap, NewHTTPError(http.StatusBadRequest, "failed to parse request body: %s", err)
	}
	if err := json.Unmarshal(emailReq.Values, &payloadMap); err != nil {
		return emailReq, payloadMap, NewHTTPError(http.StatusBadRequest, "failed to parse template values: %s", err)
	}
	return emailReq, payloadMap, nil
}

// Process parses, formats, and sends a single email request. Errors are *HTTPError
// where the failure can be attributed to the request itself.
func (p *EmailProcessor) Process(ctx context.Context, body []byte) error {
	emailReq, payloadMap, err := parseEmailRequest(body)
	if err != nil {
		return err
	}
	if emailReq.FromAddr == "" {
		emailReq.FromAddr = p.fromAddress
	}

	log.WithContext(ctx).Infof("processing email request: template %q to %s", emailReq.Template, emailReq.To)

	formattedMsg, isHtml, err := FormatMessage(ctx, emailReq, payloadMap, *p.deSettings)
	if err != nil {
		return err
	}

	mimeType := TEXT_MIME_TYPE
	if isHtml {
		mimeType = HTML_MIME_TYPE
	}
	formattedReq := &FormattedEmailRequest{
		To:          []string{emailReq.To},
		Cc:          emailReq.Cc,
		Bcc:         emailReq.Bcc,
		From:        emailReq.FromAddr,
		Subject:     emailReq.Subject,
		Attachments: emailReq.Attachments,
		MIMEType:    mimeType,
		Body:        formattedMsg.String(),
	}
	if err := p.sender.Send(ctx, formattedReq); err != nil {
		return fmt.Errorf("failed to send email to %s: %w", emailReq.To, err)
	}
	return nil
}
