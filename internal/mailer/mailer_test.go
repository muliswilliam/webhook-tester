package mailer

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromEnv(t *testing.T) {
	logger := log.New(io.Discard, "", 0)

	t.Setenv("SMTP_HOST", "")
	assert.IsType(t, &LogMailer{}, FromEnv(logger))

	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "pass")
	t.Setenv("SMTP_FROM", "Webhook Tester <no-reply@example.com>")
	assert.Equal(t, &SMTPMailer{
		Addr:     "smtp.example.com:587",
		Username: "user",
		Password: "pass",
		From:     "Webhook Tester <no-reply@example.com>",
	}, FromEnv(logger))

	t.Setenv("SMTP_PORT", "2525")
	assert.Equal(t, "smtp.example.com:2525", FromEnv(logger).(*SMTPMailer).Addr)
}

func TestLogMailer(t *testing.T) {
	var buf bytes.Buffer
	m := &LogMailer{Logger: log.New(&buf, "", 0)}

	require.NoError(t, m.Send("jane@example.com", "Hello", "Body text"))
	assert.Contains(t, buf.String(), "jane@example.com")
	assert.Contains(t, buf.String(), "Body text")
}

func TestBuildMessage(t *testing.T) {
	msg, err := buildMessage("from@example.com", "to@example.com", "Reset your password", "line 1\nline 2\n")
	require.NoError(t, err)

	s := string(msg)
	assert.Contains(t, s, "From: from@example.com\r\n")
	assert.Contains(t, s, "To: to@example.com\r\n")
	assert.Contains(t, s, "Subject: Reset your password\r\n")
	assert.Contains(t, s, "Content-Type: text/plain; charset=utf-8\r\n")
	assert.True(t, strings.HasSuffix(s, "\r\n\r\nline 1\r\nline 2\r\n"))
}

func TestBuildMessage_RejectsHeaderInjection(t *testing.T) {
	_, err := buildMessage("from@example.com", "to@example.com\r\nBcc: x@evil.example", "s", "b")
	assert.Error(t, err)
}

// TestSMTPMailer_Send talks to a minimal in-process SMTP server.
func TestSMTPMailer_Send(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	received := make(chan string, 1)
	envelopeFrom := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		tp := textproto.NewConn(conn)
		_ = tp.PrintfLine("220 test ESMTP")
		for {
			line, err := tp.ReadLine()
			if err != nil {
				return
			}
			switch cmd := strings.ToUpper(strings.Fields(line)[0]); cmd {
			case "EHLO", "HELO":
				_ = tp.PrintfLine("250 test")
			case "MAIL":
				envelopeFrom <- line
				_ = tp.PrintfLine("250 OK")
			case "RCPT":
				_ = tp.PrintfLine("250 OK")
			case "DATA":
				_ = tp.PrintfLine("354 go ahead")
				data, _ := io.ReadAll(bufio.NewReader(tp.DotReader()))
				received <- string(data)
				_ = tp.PrintfLine("250 OK")
			case "QUIT":
				_ = tp.PrintfLine("221 bye")
				return
			default:
				_ = tp.PrintfLine("250 OK")
			}
		}
	}()

	m := &SMTPMailer{Addr: ln.Addr().String(), From: "Webhook Tester <from@example.com>"}
	require.NoError(t, m.Send("to@example.com", "Hi", "Hello there"))
	assert.Equal(t, "MAIL FROM:<from@example.com>", <-envelopeFrom, "the envelope takes the bare address")
	msg := <-received
	assert.Contains(t, msg, `From: "Webhook Tester" <from@example.com>`)
	assert.Contains(t, msg, "Hello there")
}

func TestSMTPMailer_InvalidFrom(t *testing.T) {
	m := &SMTPMailer{Addr: "127.0.0.1:1", From: "not an address"}
	assert.ErrorContains(t, m.Send("to@example.com", "Hi", "Hello"), "SMTP_FROM")
}
