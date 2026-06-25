// Package mailer sends SMTP email using only the Go standard library.
// It supports plain, STARTTLS and implicit SSL/TLS transports.
package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"netwatch/internal/model"
)

const dialTimeout = 20 * time.Second

// Recipients splits a comma/semicolon/space separated address string.
func Recipients(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	}) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Send delivers a plain-text message using the supplied configuration.
func Send(cfg model.EmailConfig, subject, body string) error {
	if cfg.Server == "" || cfg.Port == 0 {
		return fmt.Errorf("SMTP server and port are required")
	}
	to := Recipients(cfg.To)
	if len(to) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}
	if from == "" {
		return fmt.Errorf("a From address is required")
	}

	msg := buildMessage(from, to, subject, body)
	addr := net.JoinHostPort(cfg.Server, strconv.Itoa(cfg.Port))
	tlsCfg := &tls.Config{
		ServerName:         cfg.Server,
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // user-controlled, documented
	}

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Server)
	}

	if strings.EqualFold(cfg.TLSMode, "ssl") {
		return sendSSL(addr, cfg.Server, tlsCfg, auth, from, to, msg)
	}
	return sendPlainOrStartTLS(cfg, addr, cfg.Server, tlsCfg, auth, from, to, msg)
}

func sendSSL(addr, host string, tlsCfg *tls.Config, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: dialTimeout}, "tcp", addr, tlsCfg)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	return finish(c, auth, from, to, msg)
}

func sendPlainOrStartTLS(cfg model.EmailConfig, addr, host string, tlsCfg *tls.Config, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()

	if strings.EqualFold(cfg.TLSMode, "starttls") {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("server does not advertise STARTTLS")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	return finish(c, auth, from, to, msg)
}

func finish(c *smtp.Client, auth smtp.Auth, from string, to []string, msg []byte) error {
	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, r := range to {
		if err := c.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func buildMessage(from string, to []string, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	b.WriteString("\r\n")
	return []byte(b.String())
}
