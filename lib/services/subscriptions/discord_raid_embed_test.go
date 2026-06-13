package subscriptions

import (
	"strings"
	"testing"

	"raidhub/lib/services/player"
)

func TestFireteamPlayerLineMarkdown_classEmojiOutsideLink(t *testing.T) {
	line := fireteamPlayerLineMarkdown(player.PlayerProfileForDelivery{
		MembershipID: 4611686018488107374,
		DisplayName:  "TestPlayer",
		ClassHash:    hunterClassHash,
	}, false, map[int64]InstancePlayerStats{
		4611686018488107374: {Kills: 10, Assists: 2, Deaths: 1},
	})

	if strings.Contains(line, "["+hunterEmoji) {
		t.Fatalf("class emoji must be outside masked link brackets, got: %q", line)
	}
	if !strings.HasPrefix(line, "- "+hunterEmoji+" [") {
		t.Fatalf("expected emoji before link, got: %q", line)
	}
	if !strings.Contains(line, "10 / 2 / 1") {
		t.Fatalf("expected K/A/D stats, got: %q", line)
	}
}

func TestFireteamPlayerLineMarkdown_strikethroughKeepsEmojiOutsideLink(t *testing.T) {
	line := fireteamPlayerLineMarkdown(player.PlayerProfileForDelivery{
		MembershipID: 1,
		DisplayName:  "DNF",
		ClassHash:    titanClassHash,
		Finished:     false,
	}, true, nil)

	if strings.Contains(line, "["+titanEmoji) {
		t.Fatalf("class emoji must be outside masked link brackets, got: %q", line)
	}
	if !strings.Contains(line, "[~~") {
		t.Fatalf("expected strikethrough inside link only, got: %q", line)
	}
}
