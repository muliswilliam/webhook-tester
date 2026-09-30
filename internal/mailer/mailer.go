// Package mailer sends the app's transactional emails.
package mailer

import (
	"bytes"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// A Mailer sends plain-text emails.
type Mailer interface {
	Send(to, subject, body string) error
}

// FromEnv returns an SMTP mailer configured by SMTP_HOST, SMTP_PORT (default
// 587), SMTP_USERNAME, SMTP_PASSWORD and SMTP_FROM. Without SMTP_HOST it
// returns a mailer that only logs emails, for local development.
func FromEnv(logger *log.Logger) Mailer {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		logger.Println("SMTP_HOST not set - emails will be logged instead of sent")
		return &LogMailer{Logger: logger}
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	if _, err := mail.ParseAddress(os.Getenv("SMTP_FROM")); err != nil {
		logger.Printf("SMTP_FROM %q is not a valid address - emails will fail to send: %v", os.Getenv("SMTP_FROM"), err)
	}
	return &SMTPMailer{
		Addr:     net.JoinHostPort(host, port),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
}

// SMTPMailer sends email through an SMTP server, upgrading to TLS with
// STARTTLS whenever the server offers it.
type SMTPMailer struct {
	Addr     string // host:port
	Username string // optional; enables PLAIN auth, which net/smtp only allows over TLS or to localhost
	Password string
	From     string
}

func (m *SMTPMailer) Send(to, subject, body string) error {
	// From may carry a display name ("Name <addr>"), which only belongs in
	// the header; the SMTP envelope takes the bare address.
	from, err := mail.ParseAddress(m.From)
	if err != nil {
		return fmt.Errorf("invalid SMTP_FROM %q: %w", m.From, err)
	}
	var auth smtp.Auth
	if m.Username != "" {
		host, _, _ := net.SplitHostPort(m.Addr)
		auth = smtp.PlainAuth("", m.Username, m.Password, host)
	}
	msg, err := buildMessage(from.String(), to, subject, body)
	if err != nil {
		return err
	}
	if err := smtp.SendMail(m.Addr, auth, from.Address, []string{to}, msg); err != nil {
		return fmt.Errorf("sending email to %s: %w", to, err)
	}
	return nil
}

// LogMailer logs emails instead of sending them.
type LogMailer struct {
	Logger *log.Logger
}

func (m *LogMailer) Send(to, subject, body string) error {
	m.Logger.Printf("email to %s: %s\n%s", to, subject, body)
	return nil
}

func buildMessage(from, to, subject, body string) ([]byte, error) {
	for _, v := range []string{from, to, subject} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("email header contains a line break: %q", v)
		}
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return b.Bytes(), nil
}
