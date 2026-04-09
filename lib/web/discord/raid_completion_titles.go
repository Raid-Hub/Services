package discord

import (
	"fmt"
	"strconv"
	"strings"
)

const embedTitleMaxRunes = 200

// RaidCompletionMainTitle is the primary embed title: the raid/activity name when known,
// otherwise a short player-facing fallback (no product jargon).
func RaidCompletionMainTitle(activityName string, completed bool) string {
	s := strings.TrimSpace(activityName)
	if s != "" {
		return truncateEmbedTitle(s)
	}
	if completed {
		return "Raid completed"
	}
	return "Raid"
}

// RaidCompletionGuardianTitle titles per-guardian thumbnail embeds: display name when known.
func RaidCompletionGuardianTitle(displayName string, membershipID int64) string {
	s := strings.TrimSpace(displayName)
	if s != "" {
		return truncateEmbedTitle(s)
	}
	return fmt.Sprintf("Guardian · %s", strconv.FormatInt(membershipID, 10))
}

func truncateEmbedTitle(s string) string {
	r := []rune(s)
	if len(r) <= embedTitleMaxRunes {
		return s
	}
	return string(r[:embedTitleMaxRunes-1]) + "…"
}
