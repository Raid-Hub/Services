package discord

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

// DefaultWebhookUsername is the Execute Webhook display name when username is omitted or empty.
const DefaultWebhookUsername = "RaidHub"
