package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const webhookHTTPTimeout = 45 * time.Second

var webhookHTTPClient = &http.Client{Timeout: webhookHTTPTimeout}

// PermanentDeliveryError indicates the outbound request should not be retried (e.g. 404 deleted webhook).
type PermanentDeliveryError struct {
	Status int
	Detail string
}

func (e *PermanentDeliveryError) Error() string {
	if e == nil {
		return "discord webhook: permanent failure"
	}
	return fmt.Sprintf("discord webhook: status %d: %s", e.Status, e.Detail)
}

// IsPermanentDeliveryError reports whether err is a PermanentDeliveryError.
func IsPermanentDeliveryError(err error) bool {
	var p *PermanentDeliveryError
	return errors.As(err, &p)
}

func discordPermanentHTTPStatus(code int) bool {
	if code < 400 || code >= 500 {
		return false
	}
	switch code {
	case 408, 425, 429:
		return false
	default:
		return true
	}
}

func truncateWebhookErrBody(body []byte, max int) string {
	s := strings.TrimSpace(string(body))
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// discordWebhookExecuteQuery sets Execute Webhook query params per Discord API docs:
//   - wait=true: wait for server confirmation so failures return an error (default wait=false can succeed even if the message is not saved).
//   - with_components=true: required for components / IS_COMPONENTS_V2 on non-application-owned webhooks.
//
// https://discord.com/developers/docs/resources/webhook#execute-webhook
func discordWebhookExecuteURL(base string, withComponents bool) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse webhook url: %w", err)
	}
	q := u.Query()
	q.Set("wait", "true")
	if withComponents {
		q.Set("with_components", "true")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// discordAPIErrorBody matches common Discord REST JSON error shapes (reference#error-messages, rate-limits#exceeding-a-rate-limit).
type discordAPIErrorBody struct {
	Code       int     `json:"code"`
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"`
	Global     bool    `json:"global"`
}

func formatDiscordWebhookError(status int, body []byte, retryAfterHeader string) string {
	const max = 768
	var d discordAPIErrorBody
	if len(body) > 0 && json.Unmarshal(body, &d) == nil && strings.TrimSpace(d.Message) != "" {
		var b strings.Builder
		b.WriteString(d.Message)
		if d.Code != 0 {
			fmt.Fprintf(&b, " (code=%d)", d.Code)
		}
		if status == http.StatusTooManyRequests {
			if d.RetryAfter > 0 {
				fmt.Fprintf(&b, "; retry_after=%gs", d.RetryAfter)
			}
			if d.Global {
				b.WriteString("; global=true")
			}
			if strings.TrimSpace(retryAfterHeader) != "" {
				fmt.Fprintf(&b, "; Retry-After=%s", strings.TrimSpace(retryAfterHeader))
			}
		}
		return truncateWebhookErrBody([]byte(b.String()), max)
	}
	if strings.TrimSpace(retryAfterHeader) != "" && status == http.StatusTooManyRequests {
		return truncateWebhookErrBody([]byte(fmt.Sprintf("HTTP %d; Retry-After=%s", status, strings.TrimSpace(retryAfterHeader))), max)
	}
	out := truncateWebhookErrBody(body, max)
	if out == "" {
		return fmt.Sprintf("empty response body (HTTP %d)", status)
	}
	return out
}

func applyDefaultWebhookIdentity(w *Webhook) {
	if w.Username == nil || strings.TrimSpace(*w.Username) == "" {
		u := DefaultWebhookUsername
		w.Username = &u
	}
	if w.AvatarURL == nil || strings.TrimSpace(*w.AvatarURL) == "" {
		a := CommonFooter.IconURL
		w.AvatarURL = &a
	}
}

// SendWebhook POSTs a webhook payload. Uses ctx for cancellation; honors a 45s client timeout.
//
// Query params are set per Execute Webhook (wait, with_components); see discordWebhookExecuteURL.
// When username and avatar_url are unset (or blank), DefaultWebhookUsername and CommonFooter.IconURL are applied.
func SendWebhook(ctx context.Context, webhookURL string, webhook *Webhook) error {
	if ctx == nil {
		ctx = context.Background()
	}
	payload := *webhook
	applyDefaultWebhookIdentity(&payload)

	withComponents := len(payload.Components) > 0 || (payload.Flags != nil && (*payload.Flags&FlagIsComponentsV2) != 0)
	execURL, err := discordWebhookExecuteURL(webhookURL, withComponents)
	if err != nil {
		return err
	}

	jsonPayload, err := json.Marshal(&payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, execURL, bytes.NewReader(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read discord webhook response: %w", err)
	}

	// With wait=true, Discord returns 200 + message body on success; still treat any 2xx as success.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	detail := formatDiscordWebhookError(resp.StatusCode, body, resp.Header.Get("Retry-After"))
	if discordPermanentHTTPStatus(resp.StatusCode) {
		return &PermanentDeliveryError{Status: resp.StatusCode, Detail: detail}
	}

	return fmt.Errorf("discord webhook: status %d: %s", resp.StatusCode, detail)
}
