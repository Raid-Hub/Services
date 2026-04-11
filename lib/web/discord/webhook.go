package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// FlagIsComponentsV2 enables Discord Components V2 (Text Display, Container, etc.); embeds/content must not be used.
// See https://discord.com/developers/docs/resources/message#message-object-message-flags
const FlagIsComponentsV2 = 1 << 15 // 32768

type Webhook struct {
	Username   *string            `json:"username,omitempty"`
	AvatarURL  *string            `json:"avatar_url,omitempty"`
	Content    *string            `json:"content,omitempty"`
	Embeds     []Embed            `json:"embeds,omitempty"`
	Flags      *int               `json:"flags,omitempty"`
	Components []MessageComponent `json:"components,omitempty"`
}

type Embed struct {
	Author      *Author    `json:"author,omitempty"`
	Title       string     `json:"title"`
	URL         *string    `json:"url"`
	Description *string    `json:"description"`
	Color       int        `json:"color"`
	Fields      []Field    `json:"fields"`
	Thumbnail   *Thumbnail `json:"thumbnail"`
	Image       *Image     `json:"image"`
	Footer      Footer     `json:"footer"`
	Timestamp   string     `json:"timestamp"`
}

type Author struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	IconURL string `json:"icon_url"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type Thumbnail struct {
	URL string `json:"url"`
}

type Image struct {
	URL string `json:"url"`
}

type Footer struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url"`
}

var CommonFooter = Footer{
	Text:    "RaidHub Alerts",
	IconURL: "https://raidhub.io/_next/image?url=%2Flogo.png&w=48&q=100",
}

func SendWebhook(webhookURL string, webhook *Webhook) error {
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

	jsonPayload, err := json.Marshal(&webhook)
	if err != nil {
		return err
	}

	resp, err := http.Post(execURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read discord webhook response: %w", err)
	}

	// Execute Webhook returns 204 No Content or 200 OK with a message body on success.
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	var errorResponse map[string]any
	if err := json.Unmarshal(body, &errorResponse); err != nil {
		return fmt.Errorf("error sending discord webhook: status %d: %s", resp.StatusCode, string(body))
	}
	return fmt.Errorf("error sending discord webhook: %s (status code: %d)", errorResponse, resp.StatusCode)
}
