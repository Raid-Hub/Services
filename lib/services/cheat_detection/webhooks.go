package cheat_detection

import (
	"context"
	"fmt"
	"math"
	"raidhub/lib/env"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"raidhub/lib/web/discord"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var (
	_cheatCheckWebhookUrl string
	_nemesisWebhookUrl    string
	once                  sync.Once
	webhookRl             = rate.NewLimiter(rate.Every(3*time.Second), 5)
)

func getCheatCheckWebhookUrl() string {
	once.Do(func() {
		_cheatCheckWebhookUrl = env.CheatCheckWebhookURL
		_nemesisWebhookUrl = env.NemesisWebhookURL
	})
	return _cheatCheckWebhookUrl
}

func getNemesisWebhookUrl() string {
	getCheatCheckWebhookUrl() // Initialize once
	return _nemesisWebhookUrl
}

func getRedColor(probability float64) int {
	// Convert probability to a color value from 0xFF0000 (red) to 0xFFFFEF (pale orange)
	adjustedP := math.Pow(probability, 0.5)
	red := 0xFF
	green := 0xFF - int(0xFF*adjustedP)
	blue := 0xEF - int(0xEF*adjustedP)
	return (red << 16) + (green << 8) + blue
}

func SendFlaggedInstanceWebhook(instance *Instance, result ResultTuple, playerResults []ResultTuple, isSolo bool) {
	ctx := context.Background()

	url := fmt.Sprintf("https://raidhub.io/pgcr/%d", instance.InstanceId)
	description := fmt.Sprintf("%d", instance.InstanceId)

	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title:       "Flagged Instance",
			Color:       getRedColor(result.Probability),
			URL:         &url,
			Description: &description,
			Fields: []discord.Field{
				{
					Name:  "Cheat Probability",
					Value: fmt.Sprintf("%.3f", result.Probability),
				},
				{
					Name:  "Explanation",
					Value: result.Explanation,
				},
			},
			Thumbnail: &discord.Thumbnail{
				URL: fmt.Sprintf("https://cdn.raidhub.io/content/splash/%s/tiny.jpg", instance.RaidPath),
			},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer: discord.Footer{
				Text:    fmt.Sprintf("Cheat Check %s", CheatCheckVersion),
				IconURL: discord.CommonFooter.IconURL,
			},
		}},
	}

	if !isSolo {
		for _, playerResult := range playerResults {
			if playerResult.Probability > Threshold {
				webhook.Embeds[0].Fields = append(webhook.Embeds[0].Fields, discord.Field{
					Name:  fmt.Sprintf("Player %d", playerResult.MembershipId),
					Value: fmt.Sprintf("%.3f - %s", playerResult.Probability, playerResult.Explanation),
				})
			}
		}
	}

	err := webhookRl.Wait(ctx)
	if err != nil {
		logger.Warn(RATE_LIMITER_ERROR, err, map[string]any{
			logging.OPERATION: "webhook_rate_limit",
		})
	}

	discord.SendWebhook(getCheatCheckWebhookUrl(), &webhook)
}

func SendFlaggedPlayerWebhooks(instance *Instance, playerResults []ResultTuple) {
	ctx := context.Background()

	webhook := discord.Webhook{}
	for _, playerResult := range playerResults {
		url := fmt.Sprintf("https://raidhub.io/pgcr/%d?player=%d", instance.InstanceId, playerResult.MembershipId)
		description := fmt.Sprintf("%d", instance.InstanceId)

		if playerResult.Probability > Threshold {
			embed := discord.Embed{
				Title:       "Flagged Player",
				Color:       getRedColor(playerResult.Probability),
				URL:         &url,
				Description: &description,
				Fields: []discord.Field{
					{
						Name:  "Cheat Probability",
						Value: fmt.Sprintf("%.3f", playerResult.Probability),
					},
					{
						Name:  "Explanation",
						Value: playerResult.Explanation,
					},
				},
				Thumbnail: &discord.Thumbnail{
					URL: fmt.Sprintf("https://cdn.raidhub.io/content/splash/%s/tiny.jpg", instance.RaidPath),
				},
				Timestamp: time.Now().Format(time.RFC3339),
				Footer: discord.Footer{
					Text:    fmt.Sprintf("Cheat Check %s", CheatCheckVersion),
					IconURL: discord.CommonFooter.IconURL,
				},
			}
			webhook.Embeds = append(webhook.Embeds, embed)
		}
	}
	err := webhookRl.Wait(ctx)
	if err != nil {
		logger.Warn(RATE_LIMITER_ERROR, err, map[string]any{
			logging.OPERATION: "webhook_rate_limit",
		})
	}

	discord.SendWebhook(getCheatCheckWebhookUrl(), &webhook)
}

func (flag PlayerInstanceFlagStats) SendBlacklistedPlayerWebhook(profile *bungie.DestinyProfileComponent, clears int, ageInDays float64, bungieName string, iconPath string, cheaterAccountChance float64, flags uint64) {

	ctx := context.Background()
	err := webhookRl.Wait(ctx)
	if err != nil {
		logger.Warn(RATE_LIMITER_ERROR, err, map[string]any{
			logging.OPERATION: "webhook_rate_limit",
		})
	}

	url := fmt.Sprintf("https://raidhub.io/profile/%d", flag.MembershipId)
	description := fmt.Sprintf("%d", flag.MembershipId)

	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "Blacklisted Player",
			Color: 15548997, // Red
			Thumbnail: &discord.Thumbnail{
				URL: fmt.Sprintf("https://www.bungie.net%s", iconPath),
			},
			URL:         &url,
			Description: &description,
			Fields: []discord.Field{
				{
					Name:  "Name",
					Value: bungieName,
				},
				{
					Name:  "Last Seen",
					Value: fmt.Sprintf("<t:%d:R>", profile.DateLastPlayed.Unix()),
				},
				{
					Name:  "Clears",
					Value: fmt.Sprintf("%d", clears),
				},
				{
					Name:  "Account Age",
					Value: fmt.Sprintf("%.0f days", ageInDays),
				},
				{
					Name:  "Class A Flags",
					Value: fmt.Sprintf("%d", flag.FlagsA),
				},
				{
					Name:  "Cheater Account Flags",
					Value: fmt.Sprintf("%.3f \n- %s", cheaterAccountChance, strings.Join(GetCheaterAccountFlagsStrings(flags), "\n- ")),
				},
			},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}

	logger.Info(PLAYER_BLACKLISTED, map[string]any{
		logging.MEMBERSHIP_ID: flag.MembershipId,
		"bungie_name":         bungieName,
		"clears":              clears,
		"age_days":            ageInDays,
		"cheat_chance":        cheaterAccountChance,
		"flags":               strings.Join(GetCheaterAccountFlagsStrings(flags), ", "),
		"class_a_flags":       flag.FlagsA,
	})

	discord.SendWebhook(getNemesisWebhookUrl(), &webhook)
}
