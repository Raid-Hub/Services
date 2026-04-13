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

// SendWebhook POSTs a webhook payload. Uses ctx for cancellation; honors a 45s client timeout.
func SendWebhook(ctx context.Context, webhookURL string, webhook *Webhook) error {
	if ctx == nil {
		ctx = context.Background()
	}
	execURL := webhookURL
	if len(webhook.Components) > 0 || (webhook.Flags != nil && (*webhook.Flags&FlagIsComponentsV2) != 0) {
		u, err := url.Parse(webhookURL)
		if err != nil {
			return fmt.Errorf("parse webhook url: %w", err)
		}
		q := u.Query()
		q.Set("with_components", "true")
		u.RawQuery = q.Encode()
		execURL = u.String()
	}

	jsonPayload, err := json.Marshal(webhook)
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

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	detail := truncateWebhookErrBody(body, 512)
	if discordPermanentHTTPStatus(resp.StatusCode) {
		return &PermanentDeliveryError{Status: resp.StatusCode, Detail: detail}
	}

	var errorResponse map[string]any
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return fmt.Errorf("error sending discord webhook: status %d: %s", resp.StatusCode, detail)
	}
	return fmt.Errorf("error sending discord webhook: %s (status code: %d)", errorResponse, resp.StatusCode)
}
