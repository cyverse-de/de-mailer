package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"gopkg.in/gomail.v2"

	"jaytaylor.com/html2text"
)

const HTML_MIME_TYPE = "text/html"
const TEXT_MIME_TYPE = "text/plain"

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

func (r *EmailClient) Send(ctx context.Context, to []string, mimeType, subject, body string) error {
	ctx, span := otel.Tracer(otelName).Start(ctx, "EmailClient.Send")
	defer span.End()

	log := log.WithContext(ctx)

	m := gomail.NewMessage()
	m.SetHeader("From", r.fromAddress)
	m.SetHeader("mailed-by", "cyverse.org")
	m.SetHeader("To", to[0])
	m.SetHeader("Subject", subject)
	if mimeType == HTML_MIME_TYPE {
		plaintext, err := html2text.FromString(body)
		if err != nil {
			m.SetBody(mimeType, body)
			log.Info(err)
		} else {
			m.SetBody("text/plain", plaintext)
			m.AddAlternative(mimeType, body)
		}
	} else {
		m.SetBody(mimeType, body)
	}

	d := gomail.Dialer{Host: r.smtpHost, Port: r.smtpPort, LocalName: "de-mailer"}

	if err := d.DialAndSend(m); err != nil {
		log.Error(err)
		return err
	}

	return nil
}
