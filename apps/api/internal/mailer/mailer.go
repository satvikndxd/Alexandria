// Package mailer delivers the transactional email outbox.
//
// Emails are queued in the same transaction that creates their trigger (a
// magic link, a welcome), then delivered here by a worker. A slow or blocked
// SMTP relay therefore delays mail; it never stalls an auth request and never
// loses a token.
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// Sender is the transport seam: production uses SMTP, local development uses
// the log sender so the whole auth flow is testable with zero infrastructure.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// LogSender prints mail to structured logs. It is the default in development
// and in CI, where the magic-link URL is readable from the test output.
type LogSender struct{}

func (LogSender) Send(_ context.Context, to, subject, body string) error {
	slog.Info("email (log transport)", "to", to, "subject", subject, "body", body)
	return nil
}

// SMTPSender is a plain-AUTH SMTP client. STARTTLS is handled by net/smtp when
// the server advertises it.
type SMTPSender struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func (s SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	addr := s.Host + ":" + orDefault(s.Port, "587")
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	msg := strings.Join([]string{
		"From: " + orDefault(s.From, "Alexandria <noreply@alexandria.example>"),
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")
	return smtp.SendMail(addr, auth, orDefault(s.From, "noreply@alexandria.example"), []string{to}, []byte(msg))
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// Worker drains the outbox on an interval.
type Worker struct {
	Store    *store.Store
	Sender   Sender
	Interval time.Duration
	Batch    int32
}

func New(st *store.Store, sender Sender) *Worker {
	return &Worker{Store: st, Sender: sender, Interval: 5 * time.Second, Batch: 20}
}

func (w *Worker) Run(ctx context.Context) error {
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := w.drain(ctx); err != nil {
				slog.ErrorContext(ctx, "mail drain failed", "err", err)
			}
		}
	}
}

func (w *Worker) drain(ctx context.Context) error {
	rows, err := w.Store.Queries().FetchPendingEmails(ctx, db.FetchPendingEmailsParams{
		MaxAttempts: 5, Lim: w.Batch,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		subject, body := Render(row.Template, row.Payload)
		err := w.Sender.Send(ctx, row.ToEmail, subject, body)
		if err != nil {
			slog.WarnContext(ctx, "email send failed", "id", row.ID, "err", err)
			_, _ = w.Store.Queries().MarkEmailFailed(ctx, db.MarkEmailFailedParams{
				ID: row.ID, LastError: strPtr(err.Error()),
			})
			continue
		}
		if _, err := w.Store.Queries().MarkEmailSent(ctx, row.ID); err != nil {
			return err
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }

// Render produces plain-text mail. Plain text on purpose: no tracking pixels,
// no remote images, no HTML that a client must sanitize — a privacy property,
// not an aesthetic compromise.
func Render(template string, payload []byte) (subject, body string) {
	var p struct {
		Username string `json:"username"`
		URL      string `json:"url"`
		TTLMin   int    `json:"ttl_min"`
	}
	_ = unmarshal(payload, &p)

	switch template {
	case "magic_link":
		return "Your Alexandria sign-in link", fmt.Sprintf(
			"Hello %s,\n\nSomeone — hopefully you — asked to sign in to Alexandria.\n\n"+
				"Follow this link to continue. It expires in %d minutes and works once:\n\n%s\n\n"+
				"If you did not ask for this, ignore this email: the link dies on its own.\n\n"+
				"— Alexandria\nBooks, people, ideas, forever.\n",
			orDefault(p.Username, "reader"), orDefaultInt(p.TTLMin, 15), p.URL)
	case "welcome":
		return "Welcome to Alexandria", fmt.Sprintf(
			"Hello %s,\n\nYour shelf is ready. Alexandria is a library first and a "+
				"social network a distant second: no ads, no engagement algorithms, "+
				"no AI ghostwriting. Just books, and the people who love them.\n\n"+
				"Add a passkey from your profile to keep your account yours.\n\n"+
				"— Alexandria\n", orDefault(p.Username, "reader"))
	case "moderation_notice":
		return "A note about your Alexandria account",
			"A moderator has reviewed a report concerning your account. " +
				"Details are on your profile. Every moderation action at Alexandria " +
				"carries a written rationale, and every action can be appealed.\n"
	default:
		return "A message from Alexandria", string(payload)
	}
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
