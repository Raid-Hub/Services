package subscriptions

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/services/player"
	"raidhub/lib/web/discord"
)

const (
	// raidAccentCleared / raidAccentIncomplete tint the Container accent bar (Components V2).
	raidAccentCleared    = 5763719  // #57F287
	raidAccentIncomplete = 15548997 // #ED4245

	hunterClassHash  = 671679327
	warlockClassHash = 2271682572
	titanClassHash   = 3655393761

	hunterEmoji  = "<:hunter:1082054321413300246>"
	titanEmoji   = "<:titan:1082054336777035937>"
	warlockEmoji = "<:warlock:1082054347694809192>"

	// Unicode U+1F3C1 — Discord :checkered_flag: (PGCR fresh == false, checkpoint / not fresh start).
	checkpointFlagEmoji = "\U0001F3C1"
)

func raidContainerAccent(completed bool) int {
	if completed {
		return raidAccentCleared
	}
	return raidAccentIncomplete
}

// buildRaidHeaderMarkdown returns the main summary Text Display: ## title, then blocks separated by blank lines.
// All completion times use Discord timestamp tokens (<t:unix:F> full locale datetime, <t:unix:R> relative).
func buildRaidHeaderMarkdown(title string, pre *messages.DiscordEmbedPreload, versionName string) string {
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(discordEscape(title))
	b.WriteString("\n\n")
	b.WriteString(raidHeaderBody(pre, versionName))
	return b.String()
}

func raidHeaderBody(pre *messages.DiscordEmbedPreload, versionName string) string {
	var blocks []string

	if v := strings.TrimSpace(versionName); v != "" && !strings.EqualFold(v, "standard") {
		blocks = append(blocks, fmt.Sprintf("**Version:** %s", discordEscape(v)))
	}

	if pre != nil && !pre.DateCompleted.IsZero() {
		end := pre.DateCompleted
		if pre.DurationSeconds > 0 {
			start := end.Add(-time.Duration(pre.DurationSeconds) * time.Second)
			blocks = append(blocks, fmt.Sprintf("<t:%d:R>\n%s — %s",
				end.Unix(),
				fmt.Sprintf("<t:%d:f>", start.Unix()),
				fmt.Sprintf("<t:%d:f>", end.Unix())))
			blocks = append(blocks, fmt.Sprintf("**Duration:** %s", formatRaidDuration(pre.DurationSeconds)))
		} else {
			blocks = append(blocks, fmt.Sprintf("<t:%d:R>\n<t:%d:f>", end.Unix(), end.Unix()))
		}
	}

	if len(blocks) == 0 {
		return "—"
	}
	return strings.Join(blocks, "\n\n")
}

// featDiscordComponents renders feats as plain text labels. This is the cleanest reliable V2 layout:
// Text Display supports text, Thumbnail forces a right-side accessory, and Media Gallery renders large tiles.
func featDiscordComponents(feats []messages.DiscordFeat) []discord.MessageComponent {
	if len(feats) == 0 {
		return nil
	}
	labels := make([]string, 0, len(feats))
	for _, f := range feats {
		lbl := strings.TrimSpace(f.Label)
		if lbl == "" {
			continue
		}
		labels = append(labels, discordEscape(lbl))
	}
	if len(labels) == 0 {
		return nil
	}
	body := "### Feats\n\n" + strings.Join(labels, " · ")
	return []discord.MessageComponent{discord.NewTextDisplay(truncateDiscordV2Text(body))}
}

func assembleRaidDiscordEmbed(
	instanceId int64,
	pre *messages.DiscordEmbedPreload,
	activityName, versionName, pathSegment string,
	feats []messages.DiscordFeat,
	fireteamProfiles []player.PlayerProfileForDelivery,
	statsMap map[int64]InstancePlayerStats,
	statsUnavailable bool,
) *discord.Webhook {
	title := discord.RaidCompletionMainTitle(activityName, pre.Completed)
	if pre.Fresh != nil && !*pre.Fresh {
		title = checkpointFlagEmoji + " " + title
	}

	pgcrURL := fmt.Sprintf("https://raidhub.io/pgcr/%d", instanceId)

	// Components V2: IS_COMPONENTS_V2 replaces embeds; see https://discord.com/developers/docs/components/reference
	// Header: optional version (non-Standard) → date range → duration → relative time.
	header := truncateDiscordV2Text(buildRaidHeaderMarkdown(title, pre, versionName))

	featBlock := featDiscordComponents(feats)

	var topInner []discord.MessageComponent
	if pathSegment != "" {
		slug := strings.Trim(pathSegment, "/")
		splash := fmt.Sprintf("https://cdn.raidhub.io/content/splash/%s/tiny.jpg", slug)
		thumbDesc := fireteamThumbnailDescription(title)
		topInner = []discord.MessageComponent{
			discord.NewSectionTextWithThumbnail(header, splash, thumbDesc),
		}
		topInner = append(topInner, featBlock...)
	} else {
		topInner = []discord.MessageComponent{
			discord.NewTextDisplay(header),
		}
		topInner = append(topInner, featBlock...)
	}

	playerInner := fireteamPlayerComponents(fireteamProfiles, statsMap, statsUnavailable)
	playerInner = append(playerInner, discord.NewLinkButtonRow("View PGCR", pgcrURL))
	playerInner = append(playerInner, discord.NewSeparatorDivider())
	playerInner = append(playerInner, raidEmbedFooter())

	// Each Container may include at most 10 child components. Raid splash uses Section+Thumbnail (compact), not Media Gallery.
	const maxContainerChildren = 10
	all := make([]discord.MessageComponent, 0, len(topInner)+len(playerInner))
	all = append(all, topInner...)
	all = append(all, playerInner...)

	accent := raidContainerAccent(pre.Completed)
	var roots []discord.MessageComponent
	if len(all) <= maxContainerChildren {
		roots = []discord.MessageComponent{discord.NewContainer(accent, all)}
	} else {
		roots = []discord.MessageComponent{
			discord.NewContainer(accent, topInner),
			discord.NewContainer(accent, playerInner),
		}
	}
	trimTextDisplaysInComponents(roots)

	flags := discord.FlagIsComponentsV2
	return &discord.Webhook{
		Flags:      &flags,
		Components: roots,
	}
}

const raidHubEmoji = "<:RaidHub:1131584991227293717>"

func raidEmbedFooter() *discord.TextDisplay {
	return discord.NewTextDisplay("-# " + raidHubEmoji + " Powered by [RaidHub](https://raidhub.io)")
}

// Discord limits combined markdown text across all Text Display components in one message (Components V2).
const discordV2MaxCombinedTextRunes = 4000

// fireteamPlayerComponents renders a compact linked player list. We still sort by kills when stats are
// available, but the display itself is just bullet-separated profile links.
func fireteamPlayerComponents(profiles []player.PlayerProfileForDelivery, stats map[int64]InstancePlayerStats, statsUnavailable bool) []discord.MessageComponent {
	if len(profiles) == 0 {
		return []discord.MessageComponent{
			discord.NewTextDisplay("### Fireteam\n\n_No fireteam data._"),
		}
	}

	ordered := append([]player.PlayerProfileForDelivery(nil), profiles...)
	if !statsUnavailable && stats != nil && len(ordered) > 1 {
		sort.Slice(ordered, func(i, j int) bool {
			ki := stats[ordered[i].MembershipID].Kills
			kj := stats[ordered[j].MembershipID].Kills
			if ki != kj {
				return ki > kj
			}
			return ordered[i].MembershipID < ordered[j].MembershipID
		})
	}

	var b strings.Builder
	b.WriteString("### Fireteam\n\n")
	for _, p := range ordered {
		name := formatFireteamPlayerFieldName(p)
		b.WriteString("- [")
		if em := classEmoji(p.ClassHash); em != "" {
			b.WriteString(em)
			b.WriteString(" ")
		}
		b.WriteString(name)
		b.WriteString("](https://raidhub.io/profile/")
		b.WriteString(strconv.FormatInt(p.MembershipID, 10))
		b.WriteString(")\n")
	}
	return []discord.MessageComponent{
		discord.NewTextDisplay(truncateDiscordV2Text(strings.TrimSpace(b.String()))),
	}
}

// fireteamThumbnailDescription is optional alt text for Section thumbnails (Discord max 1024).
func fireteamThumbnailDescription(displayName string) *string {
	s := strings.TrimSpace(displayName)
	if s == "" {
		return nil
	}
	s = discordEscape(s)
	const max = 256
	if utf8.RuneCountInString(s) <= max {
		return &s
	}
	r := []rune(s)
	s = string(r[:max-1]) + "…"
	return &s
}

// trimTextDisplaysInComponents walks Containers / Sections / ActionRows and trims TextDisplay.Content from the end until total runes ≤ cap.
func trimTextDisplaysInComponents(roots []discord.MessageComponent) {
	var flat []*discord.TextDisplay
	var walk func(discord.MessageComponent)
	walk = func(c discord.MessageComponent) {
		switch t := c.(type) {
		case *discord.Container:
			for _, ch := range t.Components {
				walk(ch)
			}
		case *discord.Section:
			for _, ch := range t.Components {
				walk(ch)
			}
		case *discord.TextDisplay:
			flat = append(flat, t)
		case *discord.ActionsRow:
			for _, ch := range t.Components {
				walk(ch)
			}
		}
	}
	for _, r := range roots {
		walk(r)
	}

	total := 0
	for _, td := range flat {
		total += utf8.RuneCountInString(td.Content)
	}
	if total <= discordV2MaxCombinedTextRunes {
		return
	}
	for i := len(flat) - 1; i >= 0 && total > discordV2MaxCombinedTextRunes; i-- {
		td := flat[i]
		co := td.Content
		n := utf8.RuneCountInString(co)
		over := total - discordV2MaxCombinedTextRunes
		if n <= over {
			td.Content = "_…_"
			total -= n - utf8.RuneCountInString("_…_")
			continue
		}
		runes := []rune(co)
		keep := len(runes) - over
		if keep < 1 {
			keep = 1
		}
		newCo := string(runes[:keep]) + "…"
		total -= n - utf8.RuneCountInString(newCo)
		td.Content = newCo
	}
}

func truncateDiscordV2Text(s string) string {
	const maxRunes = 3900
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-1]) + "…"
}

// formatFireteamPlayerFieldName is the Discord field title (plain text; Discord API max 256 chars on names).
func formatFireteamPlayerFieldName(p player.PlayerProfileForDelivery) string {
	name := strings.TrimSpace(p.DisplayName)
	if name != "" {
		return truncateDiscordFieldName(discordEscape(name))
	}
	return truncateDiscordFieldName(strconv.FormatInt(p.MembershipID, 10))
}

func formatKillDeathRatio(kills, deaths int) string {
	if kills == 0 && deaths == 0 {
		return "—"
	}
	if deaths == 0 {
		return "∞"
	}
	return fmt.Sprintf("%.2f", float64(kills)/float64(deaths))
}

func formatCompactDuration(sec int) string {
	if sec <= 0 {
		return "—"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", sec%60)
}

func formatRaidDuration(sec int) string {
	if sec <= 0 {
		return "—"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60

	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	return strings.Join(parts, " ")
}

func classEmoji(classHash uint32) string {
	switch classHash {
	case hunterClassHash:
		return hunterEmoji
	case titanClassHash:
		return titanEmoji
	case warlockClassHash:
		return warlockEmoji
	default:
		return ""
	}
}

// discordEscape escapes minimal markdown in display names for embed text.
func discordEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}

func truncateDiscordFieldName(s string) string {
	const maxRunes = 256
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-3]) + "..."
}
