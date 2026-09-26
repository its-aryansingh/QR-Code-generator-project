package email

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"sync"
)

type Sender interface {
	SendVerification(ctx context.Context, to, token, appBaseURL string) error
	SendPasswordReset(ctx context.Context, to, token, appBaseURL string) error
	SendMagicLink(ctx context.Context, to, token, appBaseURL string) error
	SendInvite(ctx context.Context, to, inviterName, workspaceName, token, appBaseURL string) error
}

// MemorySender records sent emails in memory for testing.
type MemorySender struct {
	mu     sync.Mutex
	Emails []SentEmail
}

type SentEmail struct {
	To      string
	Subject string
	Body    string
}

func NewMemorySender() *MemorySender {
	return &MemorySender{Emails: make([]SentEmail, 0)}
}

func (m *MemorySender) record(to, subject, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Emails = append(m.Emails, SentEmail{To: to, Subject: subject, Body: body})
}

func (m *MemorySender) SendVerification(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := "Verify your QRit account"
	body := fmt.Sprintf("Click the link to verify your email:\n\n%s\n\nLink expires in 24 hours.", link)
	m.record(to, subject, body)
	return nil
}

func (m *MemorySender) SendPasswordReset(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := "Reset your QRit password"
	body := fmt.Sprintf("Click the link to reset your password:\n\n%s\n\nLink expires in 1 hour.", link)
	m.record(to, subject, body)
	return nil
}

func (m *MemorySender) SendMagicLink(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/magic?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := "Your QRit magic sign-in link"
	body := fmt.Sprintf("Click the link to sign in to QRit:\n\n%s\n\nLink expires in 15 minutes.", link)
	m.record(to, subject, body)
	return nil
}

func (m *MemorySender) SendInvite(ctx context.Context, to, inviterName, workspaceName, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/invites/%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := fmt.Sprintf("You're invited to join %s on QRit", workspaceName)
	body := fmt.Sprintf("%s invited you to join the workspace %s on QRit.\n\nAccept your invite here:\n%s\n\nLink expires in 7 days.", inviterName, workspaceName, link)
	m.record(to, subject, body)
	return nil
}

// SMTPSender delivers emails over SMTP (e.g. Mailpit locally or transactional provider).
type SMTPSender struct {
	addr string
	from string
	auth smtp.Auth
}

func NewSMTPSender(addr, from string) *SMTPSender {
	return &SMTPSender{
		addr: addr,
		from: from,
	}
}

func (s *SMTPSender) send(to, subject, body string) error {
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.from, to, subject, body))
	return smtp.SendMail(s.addr, s.auth, s.from, []string{to}, msg)
}

func (s *SMTPSender) SendVerification(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := "Verify your QRit account"
	body := fmt.Sprintf("Click the link to verify your email:\n\n%s\n\nLink expires in 24 hours.", link)
	return s.send(to, subject, body)
}

func (s *SMTPSender) SendPasswordReset(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := "Reset your QRit password"
	body := fmt.Sprintf("Click the link to reset your password:\n\n%s\n\nLink expires in 1 hour.", link)
	return s.send(to, subject, body)
}

func (s *SMTPSender) SendMagicLink(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/magic?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := "Your QRit magic sign-in link"
	body := fmt.Sprintf("Click the link to sign in to QRit:\n\n%s\n\nLink expires in 15 minutes.", link)
	return s.send(to, subject, body)
}

func (s *SMTPSender) SendInvite(ctx context.Context, to, inviterName, workspaceName, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/invites/%s", strings.TrimRight(appBaseURL, "/"), token)
	subject := fmt.Sprintf("You're invited to join %s on QRit", workspaceName)
	body := fmt.Sprintf("%s invited you to join the workspace %s on QRit.\n\nAccept your invite here:\n%s\n\nLink expires in 7 days.", inviterName, workspaceName, link)
	return s.send(to, subject, body)
}

// ConsoleSender logs emails to slog (useful during development).
type ConsoleSender struct{}

func (c *ConsoleSender) SendVerification(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/verify-email?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	slog.Info("email.verification", "to", to, "link", link)
	return nil
}

func (c *ConsoleSender) SendPasswordReset(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	slog.Info("email.password_reset", "to", to, "link", link)
	return nil
}

func (c *ConsoleSender) SendMagicLink(ctx context.Context, to, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/magic?token=%s", strings.TrimRight(appBaseURL, "/"), token)
	slog.Info("email.magic_link", "to", to, "link", link)
	return nil
}

func (c *ConsoleSender) SendInvite(ctx context.Context, to, inviterName, workspaceName, token, appBaseURL string) error {
	link := fmt.Sprintf("%s/invites/%s", strings.TrimRight(appBaseURL, "/"), token)
	slog.Info("email.invite", "to", to, "workspace", workspaceName, "link", link)
	return nil
}

// Last returns the most recent email sent to the address (tests).
func (m *MemorySender) Last(to string) (SentEmail, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.Emails) - 1; i >= 0; i-- {
		if strings.EqualFold(m.Emails[i].To, to) {
			return m.Emails[i], true
		}
	}
	return SentEmail{}, false
}
