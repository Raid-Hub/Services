// Components V2 message types for webhook payloads (Text Display, Container, etc.).
// Adapted from github.com/bwmarrin/discordgo (BSD-3-Clause); see https://github.com/bwmarrin/discordgo/blob/master/components.go
//
// Discord reference: https://discord.com/developers/docs/components/reference

package discord

import (
	"encoding/json"
)

// ComponentType identifies a message component.
type ComponentType uint

const (
	ActionsRowComponent   ComponentType = 1
	ButtonComponent       ComponentType = 2
	SectionComponent      ComponentType = 9
	TextDisplayComponent  ComponentType = 10
	ThumbnailComponent    ComponentType = 11
	MediaGalleryComponent ComponentType = 12
	SeparatorComponent    ComponentType = 14
	ContainerComponent    ComponentType = 17
)

// MessageComponent is implemented by all V2 layout/content components we send.
type MessageComponent interface {
	json.Marshaler
	Type() ComponentType
}

// TextDisplay is markdown body text (type 10).
type TextDisplay struct {
	Content string `json:"content"`
}

func (TextDisplay) Type() ComponentType { return TextDisplayComponent }

func (t TextDisplay) MarshalJSON() ([]byte, error) {
	type td TextDisplay
	return json.Marshal(struct {
		td
		Type ComponentType `json:"type"`
	}{
		td:   td(t),
		Type: t.Type(),
	})
}

// UnfurledMediaItem is a URL reference for gallery / media.
type UnfurledMediaItem struct {
	URL string `json:"url"`
}

// MediaGalleryItem is one cell in a gallery.
type MediaGalleryItem struct {
	Media       UnfurledMediaItem `json:"media"`
	Description *string           `json:"description,omitempty"`
	Spoiler     bool              `json:"spoiler,omitempty"`
}

// MediaGallery shows one or more images (type 12).
type MediaGallery struct {
	Items []MediaGalleryItem `json:"items"`
	ID    int                `json:"id,omitempty"`
}

func (MediaGallery) Type() ComponentType { return MediaGalleryComponent }

func (m MediaGallery) MarshalJSON() ([]byte, error) {
	type mg MediaGallery
	return json.Marshal(struct {
		mg
		Type ComponentType `json:"type"`
	}{
		mg:   mg(m),
		Type: m.Type(),
	})
}

// ButtonStyle is a Discord button style (link = 5).
type ButtonStyle uint

const LinkButtonStyle ButtonStyle = 5

// Button is an interactive or link button (type 2).
type Button struct {
	Label    string      `json:"label"`
	Style    ButtonStyle `json:"style"`
	URL      string      `json:"url,omitempty"`
	CustomID string      `json:"custom_id,omitempty"`
	Disabled bool        `json:"disabled,omitempty"`
	ID       int         `json:"id,omitempty"`
}

func (Button) Type() ComponentType { return ButtonComponent }

func (b Button) MarshalJSON() ([]byte, error) {
	type btn Button
	if b.Style == 0 {
		b.Style = LinkButtonStyle
	}
	return json.Marshal(struct {
		btn
		Type ComponentType `json:"type"`
	}{
		btn:  btn(b),
		Type: b.Type(),
	})
}

// ActionsRow holds up to five buttons or one select (type 1).
type ActionsRow struct {
	Components []MessageComponent `json:"components"`
	ID         int                `json:"id,omitempty"`
}

func (ActionsRow) Type() ComponentType { return ActionsRowComponent }

func (r ActionsRow) MarshalJSON() ([]byte, error) {
	type ar ActionsRow
	return json.Marshal(struct {
		ar
		Type ComponentType `json:"type"`
	}{
		ar:   ar(r),
		Type: r.Type(),
	})
}

// Section joins 1–3 Text Displays with a Thumbnail or Button accessory (type 9).
type Section struct {
	ID         int                `json:"id,omitempty"`
	Components []MessageComponent `json:"components"`
	Accessory  MessageComponent   `json:"accessory"`
}

func (Section) Type() ComponentType { return SectionComponent }

func (s Section) MarshalJSON() ([]byte, error) {
	type sec Section
	return json.Marshal(struct {
		sec
		Type ComponentType `json:"type"`
	}{
		sec:  sec(s),
		Type: s.Type(),
	})
}

// ThumbnailAccessory is a small image for use as a Section accessory (type 11; distinct from classic embed Thumbnail).
type ThumbnailAccessory struct {
	ID          int               `json:"id,omitempty"`
	Media       UnfurledMediaItem `json:"media"`
	Description *string           `json:"description,omitempty"`
	Spoiler     bool              `json:"spoiler,omitempty"`
}

func (ThumbnailAccessory) Type() ComponentType { return ThumbnailComponent }

func (t ThumbnailAccessory) MarshalJSON() ([]byte, error) {
	type th ThumbnailAccessory
	return json.Marshal(struct {
		th
		Type ComponentType `json:"type"`
	}{
		th:   th(t),
		Type: t.Type(),
	})
}

// SeparatorSpacingSize is vertical spacing around a Separator.
type SeparatorSpacingSize uint

const (
	SeparatorSpacingSmall SeparatorSpacingSize = 1
	SeparatorSpacingLarge SeparatorSpacingSize = 2
)

// Separator adds padding and an optional divider line (type 14).
type Separator struct {
	ID      int                   `json:"id,omitempty"`
	Divider *bool                 `json:"divider,omitempty"`
	Spacing *SeparatorSpacingSize `json:"spacing,omitempty"`
}

func (Separator) Type() ComponentType { return SeparatorComponent }

func (s Separator) MarshalJSON() ([]byte, error) {
	type sep Separator
	return json.Marshal(struct {
		sep
		Type ComponentType `json:"type"`
	}{
		sep:  sep(s),
		Type: s.Type(),
	})
}

// Container groups components with an optional accent bar (type 17).
type Container struct {
	ID          int                `json:"id,omitempty"`
	AccentColor *int               `json:"accent_color,omitempty"`
	Spoiler     bool               `json:"spoiler,omitempty"`
	Components  []MessageComponent `json:"components"`
}

func (Container) Type() ComponentType { return ContainerComponent }

func (c Container) MarshalJSON() ([]byte, error) {
	type ctr Container
	return json.Marshal(struct {
		ctr
		Type ComponentType `json:"type"`
	}{
		ctr:  ctr(c),
		Type: c.Type(),
	})
}

// NewTextDisplay returns a Text Display component.
func NewTextDisplay(content string) *TextDisplay {
	return &TextDisplay{Content: content}
}

// NewMediaGallerySingleImage builds a one-image Media Gallery.
func NewMediaGallerySingleImage(imageURL string) *MediaGallery {
	return &MediaGallery{
		Items: []MediaGalleryItem{{
			Media: UnfurledMediaItem{URL: imageURL},
		}},
	}
}

// NewLinkButtonRow is one action row with a single link-style button.
func NewLinkButtonRow(label, targetURL string) *ActionsRow {
	return &ActionsRow{
		Components: []MessageComponent{
			&Button{Label: label, Style: LinkButtonStyle, URL: targetURL},
		},
	}
}

// NewContainer wraps children; accentRGB is optional (0 = omit).
func NewContainer(accentRGB int, children []MessageComponent) *Container {
	c := &Container{Components: children}
	if accentRGB != 0 {
		a := accentRGB
		c.AccentColor = &a
	}
	return c
}

// NewSeparatorDivider adds a horizontal rule with large vertical spacing (between content blocks).
func NewSeparatorDivider() *Separator {
	div := true
	sp := SeparatorSpacingLarge
	return &Separator{Divider: &div, Spacing: &sp}
}

// NewSectionTextWithThumbnail is one Text Display with a small Thumbnail accessory (use instead of Media Gallery when a compact preview is enough).
func NewSectionTextWithThumbnail(markdown string, imageURL string, thumbnailDescription *string) *Section {
	return &Section{
		Components: []MessageComponent{
			NewTextDisplay(markdown),
		},
		Accessory: &ThumbnailAccessory{
			Media:       UnfurledMediaItem{URL: imageURL},
			Description: thumbnailDescription,
		},
	}
}

// NewPlayerSectionLines is a Section with 1–3 Text Display rows and a Thumbnail accessory (Discord limit: three text rows per Section).
func NewPlayerSectionLines(lines []string, imageURL string, thumbnailDescription *string) *Section {
	if len(lines) < 1 {
		lines = []string{"—"}
	}
	if len(lines) > 3 {
		lines = lines[:3]
	}
	comps := make([]MessageComponent, len(lines))
	for i, ln := range lines {
		comps[i] = NewTextDisplay(ln)
	}
	return &Section{
		Components: comps,
		Accessory: &ThumbnailAccessory{
			Media:       UnfurledMediaItem{URL: imageURL},
			Description: thumbnailDescription,
		},
	}
}
