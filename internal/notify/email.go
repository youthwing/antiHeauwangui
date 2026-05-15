// Package notify sends sign-result emails over SMTP (Gmail STARTTLS by default).
package notify

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// EmailClient bundles the SMTP credentials and a single per-instance dialer.
// Construct from store.SMTPConfig; safe to keep around long-term — Send opens
// a fresh connection each call.
type EmailClient struct {
	Host     string // e.g. "smtp.gmail.com"
	Port     int    // 587 for STARTTLS
	Username string // authentication user (typically the From address for Gmail)
	Password string // Gmail app password (16 chars, no spaces)
	From     string // optional display: "勿外传 <noreply@x.com>"; falls back to Username
}

// Message describes one outbound email.
type Message struct {
	To      string   // primary recipient
	Bcc     []string // optional blind copies
	Subject string
	Text    string // plain-text fallback
	HTML    string // HTML alternative (preferred by clients)
}

// ErrNotConfigured is returned when no SMTP host is set.
var ErrNotConfigured = errors.New("smtp not configured")

// Send delivers one message. Returns nil on success.
//   - Uses STARTTLS automatically (Go's smtp.SendMail negotiates it).
//   - Subject is base64-encoded per RFC 2047 for UTF-8 safety.
//   - Bcc recipients receive the message but aren't shown in headers.
func (e *EmailClient) Send(msg Message) error {
	if e.Host == "" || e.Port == 0 || e.Username == "" {
		return ErrNotConfigured
	}
	addr := net.JoinHostPort(e.Host, fmt.Sprintf("%d", e.Port))
	auth := smtp.PlainAuth("", e.Username, e.Password, e.Host)

	from := e.From
	if from == "" {
		from = e.Username
	}

	body := buildMime(from, msg.To, msg.Subject, msg.Text, msg.HTML)

	// Envelope recipients = To + Bcc.
	recipients := append([]string{msg.To}, msg.Bcc...)
	return smtp.SendMail(addr, auth, e.Username, recipients, []byte(body))
}

// Verify performs a connection + STARTTLS + AUTH dry run without sending mail.
// Returns nil if credentials are accepted.
func (e *EmailClient) Verify() error {
	if e.Host == "" || e.Port == 0 || e.Username == "" {
		return ErrNotConfigured
	}
	addr := net.JoinHostPort(e.Host, fmt.Sprintf("%d", e.Port))
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Close()
	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("ehlo: %w", err)
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		cfg := &tls.Config{ServerName: e.Host}
		if err := c.StartTLS(cfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	auth := smtp.PlainAuth("", e.Username, e.Password, e.Host)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return c.Quit()
}

func buildMime(from, to, subject, text, html string) string {
	if text == "" {
		text = stripHTML(html)
	}
	boundary := "wangui-mime-" + nowToken()
	encodedSubj := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="

	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + encodedSubj + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")

	// text part
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(text)
	b.WriteString("\r\n\r\n")

	// html part
	if html != "" {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(html)
		b.WriteString("\r\n\r\n")
	}

	b.WriteString("--" + boundary + "--\r\n")
	return b.String()
}

func nowToken() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// stripHTML is a crude fallback for when caller only supplied HTML — generates
// a plaintext alternative for clients that can't render HTML. Not perfect,
// but acceptable for our short sign-result emails.
func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
