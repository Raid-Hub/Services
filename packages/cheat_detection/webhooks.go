package cheat_detection

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"raidhub/packages/bungie"
	"raidhub/packages/discord"
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
		_cheatCheckWebhookUrl = os.Getenv("CHEAT_CHECK_WEBHOOK_URL")
		_nemesisWebhookUrl = os.Getenv("NEMESIS_WEBHOOK_URL")
	})
	return _cheatCheckWebhookUrl
}

func getNemesisWebhookUrl() string {
	once.Do(func() {
		_cheatCheckWebhookUrl = os.Getenv("CHEAT_CHECK_WEBHOOK_URL")
		_nemesisWebhookUrl = os.Getenv("NEMESIS_WEBHOOK_URL")
	})
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
		log.Fatalf("Failed to wait for rate limiter: %v", err)
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
		log.Fatalf("Failed to wait for rate limiter: %v", err)
	}

	discord.SendWebhook(getCheatCheckWebhookUrl(), &webhook)
}

func (flag PlayerInstanceFlagStats) SendBlacklistedPlayerWebhook(profile *bungie.DestinyProfileComponent, clears int, ageInDays float64, bungieName string, iconPath string, cheaterAccountChance float64, flags uint64) {

	ctx := context.Background()
	err := webhookRl.Wait(ctx)
	if err != nil {
		log.Fatalf("Failed to wait for rate limiter: %v", err)
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

	log.Printf("Blacklisted player %d: %s with clears %d, age %.0f days, chance %.3f, flags [%s], class-A flags %d",
		flag.MembershipId,
		bungieName,
		clears,
		ageInDays,
		cheaterAccountChance,
		strings.Join(GetCheaterAccountFlagsStrings(flags), ", "),
		flag.FlagsA,
	)

	discord.SendWebhook(getNemesisWebhookUrl(), &webhook)
}
