package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Webhook struct {
	Username  *string `json:"username"`
	AvatarURL *string `json:"avatar_url"`
	Content   *string `json:"content"`
	Embeds    []Embed `json:"embeds"`
}

type Embed struct {
	Author      Author     `json:"author"`
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

func SendWebhook(url string, webhook *Webhook) error {
	// Convert payload to JSON
	jsonPayload, err := json.Marshal(&webhook)
	if err != nil {
		return err
	}

	// Send the JSON payload to the Discord webhook
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		// read the json response body for more details
		decoder := json.NewDecoder(resp.Body)
		var errorResponse map[string]any
		if err := decoder.Decode(&errorResponse); err != nil {
			return fmt.Errorf("error sending discord webhook: error decoding error response %d: %s", resp.StatusCode, err)
		}

		return fmt.Errorf("error sending discord webhook: %s (status code: %d)", errorResponse, resp.StatusCode)
	}

	return nil
}
