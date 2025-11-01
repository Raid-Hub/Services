package discord

import (
	"fmt"
	"time"

	"raidhub/lib/utils/logging"
)

// FormatDuration formats a duration in seconds as a human-readable duration string
// Examples: "500ms", "45.5s", "5m 30.5s", "1h 30m 15.5s"
func FormatDuration(seconds float64) string {
	duration := time.Duration(seconds * float64(time.Second))

	if duration < time.Second {
		return fmt.Sprintf("%.0fms", float64(duration.Milliseconds()))
	} else if duration < time.Minute {
		return fmt.Sprintf("%.1fs", seconds)
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		remainingSeconds := seconds - float64(minutes*60)
		return fmt.Sprintf("%dm %.1fs", minutes, remainingSeconds)
	} else {
		hours := int(duration.Hours())
		remainingMinutes := int((duration % time.Hour).Minutes())
		remainingSeconds := seconds - float64(hours*3600) - float64(remainingMinutes*60)
		return fmt.Sprintf("%dh %dm %.1fs", hours, remainingMinutes, remainingSeconds)
	}
}

// Discord color constants
const (
	ColorDarkRed    = 10038562
	ColorBlue       = 3447003
	ColorGray       = 9807270
	ColorOrange     = 15105570
	ColorDarkOrange = 16737792
	ColorRed        = 15548997
)

// DiscordAlerting provides Discord webhook alerting functionality for a service
type DiscordAlerting struct {
	webhookURL string
	logger     logging.Logger
}

// NewDiscordAlerting creates a new DiscordAlerting instance
func NewDiscordAlerting(webhookURL string, logger logging.Logger) *DiscordAlerting {
	return &DiscordAlerting{
		webhookURL: webhookURL,
		logger:     logger,
	}
}

// Send sends a Discord webhook and optionally logs it
func (da *DiscordAlerting) Send(webhook *Webhook, logKey string, logFields map[string]any) {
	err := SendWebhook(da.webhookURL, webhook)
	if err != nil && da.logger != nil {
		da.logger.Warn("DISCORD_WEBHOOK_SEND_FAILED", map[string]any{
			logging.ERROR: err.Error(),
		})
	}
	if logKey != "" && da.logger != nil {
		if logFields != nil {
			da.logger.Info(logKey, logFields)
		} else {
			da.logger.Info(logKey, nil)
		}
	}
}

// SendError sends an error alert with optional role mention
func (da *DiscordAlerting) SendError(title string, err error, mentionRoleID string) {
	var content *string
	if mentionRoleID != "" {
		ping := fmt.Sprintf("<@&%s>", mentionRoleID)
		content = &ping
	}

	webhook := Webhook{
		Content: content,
		Embeds: []Embed{{
			Title:     title,
			Color:     ColorDarkRed,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    CommonFooter,
		}},
	}

	if err != nil {
		webhook.Embeds[0].Fields = []Field{{
			Name:  "Error",
			Value: fmt.Sprintf("%s", err.Error()),
		}}
		da.logger.Error(title, map[string]any{
			logging.ERROR: err.Error(),
		})
	}

	da.Send(&webhook, "", nil)
}

// SendInfo sends an informational alert (blue)
func (da *DiscordAlerting) SendInfo(title string, fields []Field, logKey string, logFields map[string]any) {
	webhook := Webhook{
		Embeds: []Embed{{
			Title:     title,
			Color:     ColorBlue,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    CommonFooter,
		}},
	}
	da.Send(&webhook, logKey, logFields)
}

// SendStatus sends a status update alert (gray)
func (da *DiscordAlerting) SendStatus(title string, fields []Field, logKey string, logFields map[string]any) {
	webhook := Webhook{
		Embeds: []Embed{{
			Title:     title,
			Color:     ColorGray,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    CommonFooter,
		}},
	}
	da.Send(&webhook, logKey, logFields)
}

// SendWarning sends a warning alert (orange)
func (da *DiscordAlerting) SendWarning(title string, fields []Field, logKey string, logFields map[string]any) {
	webhook := Webhook{
		Embeds: []Embed{{
			Title:     title,
			Color:     ColorOrange,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    CommonFooter,
		}},
	}
	da.Send(&webhook, logKey, logFields)
}

// SendCustom sends a custom Discord webhook with full control
func (da *DiscordAlerting) SendCustom(webhook *Webhook, logKey string, logFields map[string]any) {
	da.Send(webhook, logKey, logFields)
}
