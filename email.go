package main

import (
	"gopkg.in/gomail.v2"

	"github.com/cyverse-de/logcabin"
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

//Request struct
type Email struct {
	host    string
	from    string
	to      []string
	subject string
	body    string
}

func (r *EmailClient) Send(to []string, mimeType, subject, body string) error {

	m := gomail.NewMessage()
	m.SetHeader("From", r.fromAddress)
	m.SetHeader("mailed-by", "cyverse.org")
	m.SetHeader("To", to[0])
	m.SetHeader("Subject", subject)
	m.SetBody(mimeType, body)

	d := gomail.Dialer{Host: r.smtpHost, Port: r.smtpPort}

	if err := d.DialAndSend(m); err != nil {
		logcabin.Error.Println(err)
		return err
	}

	return nil
}
