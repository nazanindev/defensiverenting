// Package mail sends the one kind of email this server sends: a sign-in
// link. The forms Worker cannot do this for us — its send_email binding is
// pinned to a single verified inbox on purpose — so the server talks to
// Resend directly over HTTPS. No SDK: the request is one JSON POST.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Mailer delivers a plain-text message to one address.
type Mailer interface {
	Send(ctx context.Context, to, subject, text string) error
}

// Resend sends through https://resend.com. From must be an address on a
// domain verified in the Resend dashboard.
type Resend struct {
	APIKey string
	From   string
	Client *http.Client
}

func (r Resend) Send(ctx context.Context, to, subject, text string) error {
	body, err := json.Marshal(map[string]any{
		"from":    r.From,
		"to":      []string{to},
		"subject": subject,
		"text":    text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("resend: status %d: %s", res.StatusCode, msg)
	}
	return nil
}

// Log writes the message to the log instead of sending it. This is the
// development mailer: the link shows up in the server output, so signing in
// locally needs no API key and no inbox.
type Log struct {
	Logger *slog.Logger
}

func (l Log) Send(ctx context.Context, to, subject, text string) error {
	l.Logger.InfoContext(ctx, "mail (not sent: no RESEND_API_KEY)",
		slog.String("to", to), slog.String("subject", subject), slog.String("text", text))
	return nil
}
