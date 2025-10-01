package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"gopkg.in/gomail.v2"

	"github.com/inbucket/html2text"
)

const HTML_MIME_TYPE = "text/html"
const TEXT_MIME_TYPE = "text/plain"

// FormattedEmailRequest represents a request to send an email that has already been formatted.
type FormattedEmailRequest struct {
	To       []string
	Cc       []string
	Bcc      []string
	From     string
	MIMEType string
	Subject  string
	Body     string
}

// Validate returns an error if the email request is invalid.
func (r *FormattedEmailRequest) Validate() error {
	if len(r.To) == 0 {
		return fmt.Errorf("at least one destination email address must be provided")
	}
	if r.Subject == "" {
		return fmt.Errorf("a message subject must be provided")
	}
	if r.Body == "" {
		return fmt.Errorf("a message body must be provided")
	}
	return nil
}

// EmailClient is a client used to send email messages to an SMTP server.
type EmailClient struct {
	smtpHost    string
	smtpPort    int
	fromAddress string
}

// NewEmailClient creates a new email client.
func NewEmailClient(smtpHost string, from string) *EmailClient {
	return &EmailClient{
		smtpHost:    smtpHost,
		smtpPort:    25,
		fromAddress: from,
	}
}

// GetFromAddress returns the source email address. If the source address is provided in the email request then that
// emil address is used. Otherwise, the default address configured in the email client is used.
func (r *EmailClient) GetFromAddress(req *FormattedEmailRequest) string {
	fromAddress := req.From
	if fromAddress == "" {
		fromAddress = r.fromAddress
	}
	return fromAddress
}

// Send sends an email.
func (r *EmailClient) Send(ctx context.Context, req *FormattedEmailRequest) error {
	ctx, span := otel.Tracer(otelName).Start(ctx, "EmailClient.Send")
	defer span.End()
	log := log.WithContext(ctx)

	// Validate the email request.
	err := req.Validate()
	if err != nil {
		log.Errorf("Invalid email request: %s", err)
		return err
	}

	// Create the message and set the headers.
	m := gomail.NewMessage()
	m.SetHeader("From", r.GetFromAddress(req))
	m.SetHeader("mailed-by", "cyverse.org")
	m.SetHeader("To", req.To...)
	if len(req.Cc) != 0 {
		m.SetHeader("Cc", req.Cc...)
	}
	if len(req.Bcc) != 0 {
		m.SetHeader("Bcc", req.Bcc...)
	}
	m.SetHeader("Subject", req.Subject)

	// Set the message body.
	if req.MIMEType == HTML_MIME_TYPE {
		plaintext, err := html2text.FromString(req.Body)
		if err != nil {
			m.SetBody(req.MIMEType, req.Body)
			log.Info(err)
		} else {
			m.SetBody("text/plain", plaintext)
			m.AddAlternative(req.MIMEType, req.Body)
		}
	} else {
		m.SetBody(req.MIMEType, req.Body)
	}

	d := gomail.Dialer{Host: r.smtpHost, Port: r.smtpPort, LocalName: "de-mailer"}

	if err := d.DialAndSend(m); err != nil {
		log.Error(err)
		return err
	}

	return nil
}
