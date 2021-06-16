package main

import (
	"gopkg.in/gomail.v2"

	"github.com/cyverse-de/logcabin"
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

	m := gomail.NewMessage()
	m.SetHeader("From", r.from)
	m.SetHeader("mailed-by", "cyverse.org")
	m.SetHeader("To", r.to[0])
	m.SetHeader("Subject", r.subject)
	m.SetBody("text/html", r.body)

	d := gomail.Dialer{Host: r.host, Port: 25}

	// Send the email to Bob, Cora and Dan.
	if err := d.DialAndSend(m); err != nil {
		logcabin.Error.Println(err)
		return false, err
	}

	return true, nil
}
