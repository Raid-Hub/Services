package subscriptions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"raidhub/lib/dto"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/services/clan"
	"raidhub/lib/services/player"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/discord"
)

var logger = logging.NewLogger("SUBSCRIPTIONS_SERVICE")

const (
	LargeInstanceThreshold = 25
	// raidCompletionGold is a warm gold for raid clears (distinct from generic blue alerts).
	raidCompletionGold = 16034501 // #F4C26A
)

func NewSubscriptionEvent(inst *dto.Instance) messages.SubscriptionEventMessage {
	participants := make([]messages.SubscriptionParticipantMessage, 0, len(inst.Players))
	for _, playerActivity := range inst.Players {
		participants = append(participants, messages.SubscriptionParticipantMessage{
			MembershipId:   playerActivity.Player.MembershipId,
			MembershipType: playerActivity.Player.MembershipType,
			Finished:       playerActivity.Finished,
		})
	}

	return messages.SubscriptionEventMessage{
		InstanceId:       inst.InstanceId,
		ActivityHash:     inst.Hash,
		PlayerCount:      inst.PlayerCount,
		DateCompleted:    inst.DateCompleted,
		DurationSeconds:  inst.DurationSeconds,
		Completed:        inst.Completed,
		ParticipantCount: len(participants),
		Participants:     participants,
	}
}

func PrepareParticipants(event messages.SubscriptionEventMessage) (messages.SubscriptionMatchMessage, error) {
	if event.PlayerCount >= LargeInstanceThreshold {
		logger.Info("SUBSCRIPTIONS_SKIPPING_LARGE_INSTANCE", map[string]any{
			logging.INSTANCE_ID: event.InstanceId,
			logging.COUNT:       event.PlayerCount,
		})
		return messages.SubscriptionMatchMessage{}, nil
	}

	participantResults := make([]messages.ParticipantResult, 0, len(event.Participants))
	for _, participant := range event.Participants {
		result := messages.ParticipantResult{
			MembershipId:   participant.MembershipId,
			MembershipType: participant.MembershipType,
		}

		if participant.MembershipType == nil || *participant.MembershipType == 0 {
			reason := "membership_type_missing"
			result.Status = messages.ParticipantPlayerUnresolved
			result.FailureReason = &reason
		} else {
			result.Status = messages.ParticipantClanUnresolved
		}

		participantResults = append(participantResults, result)
	}

	return messages.SubscriptionMatchMessage{
		InstanceId:        event.InstanceId,
		ActivityHash:      event.ActivityHash,
		PlayerCount:       event.PlayerCount,
		DateCompleted:     event.DateCompleted,
		DurationSeconds:   event.DurationSeconds,
		Completed:         event.Completed,
		ParticipantData:   participantResults,
	}, nil
}

// MatchEvent is stage 2 of the subscription pipeline (see README.md). Order of operations:
//  1. applySubscriptionRules — privacy, clan lookup, rules → one row per matched destination
//  2. enrichDeliveryRaidContext — instance-wide raid fields on each row
//  3. attachDestinationWebhooks — batch-load webhook URLs (Postgres; not repeated in stage 3)
//  4. preloadDiscordEmbedData — batch-load embed body (activity, players, stats, clan names)
func MatchEvent(ctx context.Context, message messages.SubscriptionMatchMessage) ([]messages.SubscriptionDeliveryMessage, error) {
	deliveries, err := applySubscriptionRules(ctx, message)
	if err != nil {
		return nil, err
	}
	if len(deliveries) == 0 {
		return deliveries, nil
	}
	for i := range deliveries {
		enrichDeliveryRaidContext(&deliveries[i], message)
	}
	if err := attachDestinationWebhooks(ctx, deliveries); err != nil {
		return nil, err
	}
	if err := preloadDiscordEmbedData(ctx, deliveries); err != nil {
		return nil, err
	}
	return deliveries, nil
}

// applySubscriptionRules resolves privacy, clan membership, active rules, and produces one
// SubscriptionDeliveryMessage per destination that matched (no webhook or embed preload yet).
func applySubscriptionRules(ctx context.Context, message messages.SubscriptionMatchMessage) ([]messages.SubscriptionDeliveryMessage, error) {
	membershipIDs := make([]int64, 0, len(message.ParticipantData))
	for _, p := range message.ParticipantData {
		if p.Status != messages.ParticipantPlayerUnresolved {
			membershipIDs = append(membershipIDs, p.MembershipId)
		}
	}

	privacy, err := player.PrivateFlagsByMembershipIDs(ctx, membershipIDs)
	if err != nil {
		return nil, err
	}
	clansByMember, err := clan.GroupIDsByMembershipIDs(ctx, membershipIDs)
	if err != nil {
		return nil, err
	}

	clanGroupIDs := uniqueClanGroupIDs(clansByMember, membershipIDs)
	rules, err := loadSubscriptionRulesForMatch(ctx, membershipIDs, clanGroupIDs, message.ActivityHash)
	if err != nil {
		return nil, err
	}

	return matchRulesToDeliveries(message.InstanceId, message.ParticipantData, rules, privacy, clansByMember)
}

func enrichDeliveryRaidContext(d *messages.SubscriptionDeliveryMessage, msg messages.SubscriptionMatchMessage) {
	d.ActivityHash = msg.ActivityHash
	d.DateCompleted = msg.DateCompleted
	d.DurationSeconds = msg.DurationSeconds
	d.Completed = msg.Completed
	d.PlayerCount = msg.PlayerCount
	d.FireteamMembershipIds = fireteamMembershipIDs(msg.ParticipantData)
}

func fireteamMembershipIDs(participants []messages.ParticipantResult) []int64 {
	if len(participants) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(participants))
	for _, p := range participants {
		ids = append(ids, p.MembershipId)
	}
	return ids
}

// attachDestinationWebhooks loads webhook URLs for all destinations in one batch so the delivery
// worker does not query Postgres.
func attachDestinationWebhooks(ctx context.Context, deliveries []messages.SubscriptionDeliveryMessage) error {
	if len(deliveries) == 0 {
		return nil
	}
	seen := make(map[int64]struct{})
	ids := make([]int64, 0, len(deliveries))
	for _, d := range deliveries {
		if _, ok := seen[d.DestinationChannelId]; ok {
			continue
		}
		seen[d.DestinationChannelId] = struct{}{}
		ids = append(ids, d.DestinationChannelId)
	}
	byID, err := loadActiveDestinationsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range deliveries {
		id := deliveries[i].DestinationChannelId
		row, ok := byID[id]
		if !ok {
			return fmt.Errorf("subscription destination %d not found or inactive", id)
		}
		if strings.TrimSpace(row.WebhookURL) == "" {
			return fmt.Errorf("subscription destination %d has no webhook URL", id)
		}
		deliveries[i].WebhookURL = row.WebhookURL
	}
	return nil
}

// preloadDiscordEmbedData loads activity metadata, fireteam profiles, instance stats, and clan names
// once per match batch (embed body only; URL is attached in attachDestinationWebhooks).
func preloadDiscordEmbedData(ctx context.Context, deliveries []messages.SubscriptionDeliveryMessage) error {
	if len(deliveries) == 0 {
		return nil
	}
	d0 := deliveries[0]

	meta, err := loadActivityRaidMeta(ctx, d0.ActivityHash)
	if err != nil {
		return err
	}
	var actName, verName, pathSeg string
	if meta != nil {
		actName = meta.ActivityName
		verName = meta.VersionName
		pathSeg = meta.PathSegment
	}

	profiles, err := player.PlayerProfilesForDelivery(ctx, d0.FireteamMembershipIds)
	if err != nil {
		return err
	}
	ftProf := make([]messages.DiscordFireteamProfile, 0, len(profiles))
	for _, p := range profiles {
		ftProf = append(ftProf, messages.DiscordFireteamProfile{
			MembershipID: p.MembershipID,
			DisplayName:  p.DisplayName,
		})
	}

	statsMap, statsErr := loadInstancePlayerStats(ctx, d0.InstanceId)
	statsUnavailable := statsErr != nil
	if statsErr != nil {
		logger.Warn("SUBSCRIPTIONS_INSTANCE_STATS_UNAVAILABLE", statsErr, map[string]any{
			logging.INSTANCE_ID: d0.InstanceId,
		})
	}
	statsSlice := make([]messages.DiscordInstanceStat, 0, len(d0.FireteamMembershipIds))
	for _, mid := range d0.FireteamMembershipIds {
		s := InstancePlayerStats{}
		if statsMap != nil {
			s = statsMap[mid]
		}
		statsSlice = append(statsSlice, messages.DiscordInstanceStat{
			MembershipID:      mid,
			Kills:             s.Kills,
			Deaths:            s.Deaths,
			Assists:           s.Assists,
			TimePlayedSeconds: s.TimePlayedSeconds,
			FirstClear:        s.FirstClear,
		})
	}

	seen := make(map[int64]struct{})
	var allGIDs []int64
	for _, d := range deliveries {
		for _, g := range d.Scope.ClanGroupIds {
			if _, ok := seen[g]; ok {
				continue
			}
			seen[g] = struct{}{}
			allGIDs = append(allGIDs, g)
		}
	}
	var clanNames map[int64]string
	if len(allGIDs) > 0 {
		clanNames, err = clan.NamesByGroupIDs(ctx, allGIDs)
		if err != nil {
			return err
		}
	}

	for i := range deliveries {
		clanField := formatClanFieldLines(deliveries[i].Scope.ClanGroupIds, clanNames)
		deliveries[i].EmbedPreload = &messages.DiscordEmbedPreload{
			ActivityName:       actName,
			VersionName:        verName,
			PathSegment:        pathSeg,
			FireteamProfiles:   ftProf,
			InstanceStats:      statsSlice,
			StatsUnavailable:   statsUnavailable,
			ClanFieldMarkdown: clanField,
		}
	}
	return nil
}

// SendSubscriptionDelivery POSTs the Discord webhook. When WebhookURL is set (match pipeline), no
// Postgres is used. Otherwise the destination row is loaded (legacy / manual publishes).
func SendSubscriptionDelivery(ctx context.Context, message messages.SubscriptionDeliveryMessage) error {
	webhookURL := strings.TrimSpace(message.WebhookURL)
	if webhookURL == "" && message.DestinationChannelId <= 0 {
		logger.Info("SUBSCRIPTIONS_SKIP_LEGACY_OR_INVALID_DESTINATION", map[string]any{
			logging.INSTANCE_ID: message.InstanceId,
			"channel_id":        message.DestinationChannelId,
		})
		return nil
	}

	if webhookURL == "" {
		dest, err := loadDestination(ctx, message.DestinationChannelId)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("subscription destination %d not found or inactive", message.DestinationChannelId)
			}
			return err
		}
		webhookURL = strings.TrimSpace(dest.WebhookURL)
		if webhookURL == "" {
			return fmt.Errorf("subscription destination %d has no webhook URL", message.DestinationChannelId)
		}
	}

	if message.ChannelType != messages.DeliveryChannelDiscordWebhook {
		return fmt.Errorf("unsupported channel type %q", message.ChannelType)
	}

	wh, err := buildRaidSubscriptionWebhook(ctx, message)
	if err != nil {
		return err
	}
	return discord.SendWebhook(webhookURL, wh)
}

func buildRaidSubscriptionWebhook(ctx context.Context, msg messages.SubscriptionDeliveryMessage) (*discord.Webhook, error) {
	if msg.EmbedPreload != nil {
		return buildRaidWebhookFromEmbedPreload(msg)
	}
	return buildRaidSubscriptionWebhookLegacy(ctx, msg)
}

func buildRaidWebhookFromEmbedPreload(msg messages.SubscriptionDeliveryMessage) (*discord.Webhook, error) {
	pre := msg.EmbedPreload
	profiles := make([]player.PlayerProfileForDelivery, 0, len(pre.FireteamProfiles))
	for _, p := range pre.FireteamProfiles {
		profiles = append(profiles, player.PlayerProfileForDelivery{
			MembershipID: p.MembershipID,
			DisplayName:  p.DisplayName,
		})
	}
	statsMap := make(map[int64]InstancePlayerStats, len(pre.InstanceStats))
	for _, s := range pre.InstanceStats {
		statsMap[s.MembershipID] = InstancePlayerStats{
			Kills:             s.Kills,
			Deaths:            s.Deaths,
			Assists:           s.Assists,
			TimePlayedSeconds: s.TimePlayedSeconds,
			FirstClear:        s.FirstClear,
		}
	}
	return assembleRaidDiscordEmbed(msg, pre.ActivityName, pre.VersionName, pre.PathSegment, pre.ClanFieldMarkdown,
		profiles, statsMap, pre.StatsUnavailable), nil
}

func buildRaidSubscriptionWebhookLegacy(ctx context.Context, msg messages.SubscriptionDeliveryMessage) (*discord.Webhook, error) {
	meta, err := loadActivityRaidMeta(ctx, msg.ActivityHash)
	if err != nil {
		return nil, err
	}

	fireteamProfiles, err := player.PlayerProfilesForDelivery(ctx, msg.FireteamMembershipIds)
	if err != nil {
		return nil, err
	}
	clanNames, err := clan.NamesByGroupIDs(ctx, msg.Scope.ClanGroupIds)
	if err != nil {
		return nil, err
	}

	activityName := ""
	versionName := ""
	pathSeg := ""
	if meta != nil {
		activityName = meta.ActivityName
		versionName = meta.VersionName
		pathSeg = meta.PathSegment
	}
	clanField := formatClanFieldLines(msg.Scope.ClanGroupIds, clanNames)

	statsMap, statsErr := loadInstancePlayerStats(ctx, msg.InstanceId)
	if statsErr != nil {
		logger.Warn("SUBSCRIPTIONS_INSTANCE_STATS_UNAVAILABLE", statsErr, map[string]any{
			logging.INSTANCE_ID: msg.InstanceId,
		})
	}
	return assembleRaidDiscordEmbed(msg, activityName, versionName, pathSeg, clanField,
		fireteamProfiles, statsMap, statsErr != nil), nil
}

func assembleRaidDiscordEmbed(
	msg messages.SubscriptionDeliveryMessage,
	activityName, versionName, pathSegment, clanFieldMarkdown string,
	fireteamProfiles []player.PlayerProfileForDelivery,
	statsMap map[int64]InstancePlayerStats,
	statsUnavailable bool,
) *discord.Webhook {
	title := discord.RaidCompletionMainTitle(activityName, msg.Completed)

	var descLines []string
	if !msg.DateCompleted.IsZero() {
		descLines = append(descLines, msg.DateCompleted.Format("January 2, 2006"))
	}
	if versionName != "" {
		descLines = append(descLines, versionName)
	}

	status := "Incomplete"
	if msg.Completed {
		status = "Cleared"
	}
	dur := discord.FormatDuration(float64(msg.DurationSeconds))
	summaryParts := []string{"**" + status + "**", dur}
	if msg.PlayerCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d in fireteam", msg.PlayerCount))
	}
	if !msg.DateCompleted.IsZero() {
		summaryParts = append(summaryParts, fmt.Sprintf("Ended <t:%d:R>", msg.DateCompleted.Unix()))
	}
	descLines = append(descLines, strings.Join(summaryParts, " · "))

	description := strings.Join(descLines, "\n")
	if strings.TrimSpace(description) == "" {
		description = strings.TrimSpace(strings.Join(summaryParts, " · "))
		if description == "" {
			description = "—"
		}
	}

	pgcr := fmt.Sprintf("https://raidhub.io/pgcr/%d", msg.InstanceId)

	var activityThumb *discord.Thumbnail
	var activityImage *discord.Image
	if pathSegment != "" {
		base := fmt.Sprintf("https://cdn.raidhub.io/content/splash/%s", pathSegment)
		activityThumb = &discord.Thumbnail{URL: base + "/tiny.jpg"}
		activityImage = &discord.Image{URL: base + "/tiny.jpg"}
	}

	fireteamField := formatFireteamStatsTable(fireteamProfiles, statsMap, statsUnavailable)

	fields := []discord.Field{
		{Name: "PGCR", Value: fmt.Sprintf("[`%s`](%s)", strconv.FormatInt(msg.InstanceId, 10), pgcr), Inline: false},
	}
	if clanFieldMarkdown != "" {
		fields = append(fields, discord.Field{Name: "Clans", Value: clanFieldMarkdown, Inline: false})
	}
	if fireteamField != "" {
		fields = append(fields, discord.Field{Name: "Fireteam", Value: fireteamField, Inline: false})
	}

	footer := discord.Footer{
		Text:    "RaidHub",
		IconURL: discord.CommonFooter.IconURL,
	}

	main := discord.Embed{
		Title:       title,
		Description: &description,
		URL:         &pgcr,
		Color:       raidCompletionGold,
		Fields:      fields,
		Thumbnail:   activityThumb,
		Image:       activityImage,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Footer:      footer,
	}

	return &discord.Webhook{Embeds: []discord.Embed{main}}
}

func formatFireteamStatsTable(profiles []player.PlayerProfileForDelivery, stats map[int64]InstancePlayerStats, statsUnavailable bool) string {
	if len(profiles) == 0 {
		return ""
	}

	mvpID := int64(0)
	maxK := -1
	if !statsUnavailable && stats != nil {
		for _, p := range profiles {
			s := stats[p.MembershipID]
			if s.Kills > maxK {
				maxK = s.Kills
				mvpID = p.MembershipID
			}
		}
		if maxK <= 0 {
			mvpID = 0
		}
	}

	var b strings.Builder
	b.WriteString("```\n")
	fmt.Fprintf(&b, "%-16s %5s %5s %5s %8s %-10s  %s\n", "Player", "K", "D", "A", "K/D", "Time", "Notes")
	for _, p := range profiles {
		rawName := strings.TrimSpace(p.DisplayName)
		if rawName == "" {
			rawName = strconv.FormatInt(p.MembershipID, 10)
		} else {
			rawName = truncatePlayerDisplayName(rawName, 16)
			rawName = discordEscape(rawName)
		}

		if statsUnavailable {
			fmt.Fprintf(&b, "%-16s %5s %5s %5s %8s %-10s  %s\n",
				rawName, "—", "—", "—", "—", "—", "")
			continue
		}

		s := InstancePlayerStats{}
		if stats != nil {
			s = stats[p.MembershipID]
		}
		kd := formatKillDeathRatio(s.Kills, s.Deaths)
		tim := formatCompactDuration(s.TimePlayedSeconds)
		notes := fireteamRowNotes(s, p.MembershipID, mvpID)
		fmt.Fprintf(&b, "%-16s %5d %5d %5d %8s %-10s  %s\n",
			rawName, s.Kills, s.Deaths, s.Assists, kd, tim, notes)
	}
	b.WriteString("```")
	return truncateDiscordField(b.String())
}

func truncatePlayerDisplayName(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	if maxRunes < 2 {
		return string(r[:maxRunes])
	}
	return string(r[:maxRunes-1]) + "…"
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

func fireteamRowNotes(s InstancePlayerStats, membershipID, mvpID int64) string {
	var parts []string
	if s.FirstClear {
		parts = append(parts, "FC")
	}
	if mvpID != 0 && membershipID == mvpID {
		parts = append(parts, "MVP")
	}
	return strings.Join(parts, " ")
}

// discordEscape escapes minimal markdown in display names for embed text.
func discordEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "`", "\\`")
	return s
}

func formatClanFieldLines(ids []int64, names map[int64]string) string {
	if len(ids) == 0 {
		return ""
	}
	lines := make([]string, 0, len(ids))
	for _, id := range ids {
		n := strings.TrimSpace(names[id])
		if n == "" {
			lines = append(lines, fmt.Sprintf("`%d`", id))
		} else {
			lines = append(lines, n)
		}
	}
	return truncateDiscordField(strings.Join(lines, "\n"))
}

func truncateDiscordField(s string) string {
	const maxRunes = 1024
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-3]) + "..."
}
