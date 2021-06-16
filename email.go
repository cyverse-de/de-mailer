package main

import (
	"net/smtp"
)

//email settings
type EmailSettings struct {
	smtpHost    string
	fromAddress string
}

//Request struct
type Email struct {
	host    string
	from    string
	to      []string
	subject string
	body    string
}

func NewEmail(smtpHost string, from string, to []string, subject, body string) *Email {
	return &Email{
		host:    smtpHost,
		from:    from,
		to:      to,
		subject: subject,
		body:    body,
	}
}

func (r *Email) SendEmail() (bool, error) {
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subject := "Subject: " + r.subject + "!\n"
	msg := []byte(subject + mime + "\n" + r.body)
	addr := r.host

	if err := smtp.SendMail(addr, nil, r.from, r.to, msg); err != nil {
		return false, err
	}
	return true, nil
}
