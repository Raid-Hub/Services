package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/discord"

	"golang.org/x/time/rate"
)

var (
	_atlasWebhookURL          string
	once                      sync.Once
	missedInstanceRateLimiter = rate.NewLimiter(rate.Every(time.Minute), 1)
	atlasAlerting             *discord.DiscordAlerting
)

func init() {
	once.Do(func() {
		_atlasWebhookURL = env.AtlasWebhookURL
		atlasAlerting = discord.NewDiscordAlerting(_atlasWebhookURL, AtlasLogger)
	})
}

// formatLag formats lag in seconds as a human-readable duration string
// Deprecated: Use discord.FormatDuration instead
func formatLag(seconds float64) string {
	return discord.FormatDuration(seconds)
}

func handlePanic(r any) {
	err := fmt.Errorf("%v", r)
	atlasAlerting.SendError("Fatal error in Atlas", err, env.AlertsRoleID)
}

func sendStartUpAlert() {
	atlasAlerting.SendInfo("Starting up...", nil, "ATLAS_STARTING", map[string]any{
		logging.STATUS: "initializing",
	})
}

func logIntervalState(p20Lag float64, countWorkers int, percentNotFound, errorPercentage float64) {
	fields := []discord.Field{{
		Name:  "Lag Behind Head (P20)",
		Value: discord.FormatDuration(p20Lag),
	}, {
		Name:  "404 Percentage",
		Value: fmt.Sprintf("%.3f%%", percentNotFound),
	}, {
		Name:  "Error Percentage",
		Value: fmt.Sprintf("%.3f%%", errorPercentage),
	}, {
		Name:  "Workers Used",
		Value: fmt.Sprintf("%d", countWorkers),
	}}
	atlasAlerting.SendStatus("Status Update", fields, "STATUS_UPDATE", map[string]any{
		"lag":                discord.FormatDuration(p20Lag),
		logging.WORKER_COUNT: countWorkers,
		"percent_not_found":  percentNotFound,
		"error_percentage":   errorPercentage,
	})
}

func logWorkersStarting(numWorkers int, period int, latestId int64) {
	fields := []discord.Field{{
		Name:  "Count",
		Value: fmt.Sprintf("%d", numWorkers),
	}, {
		Name:  "Period",
		Value: fmt.Sprintf("%d", period),
	}, {
		Name:  "Current Instance Id",
		Value: fmt.Sprintf("`%d`", latestId),
	}}
	atlasAlerting.SendStatus("Workers Starting", fields, "WORKERS_STARTING", map[string]any{
		logging.WORKER_COUNT: numWorkers,
		"period":             period,
		logging.INSTANCE_ID:  latestId,
	})
}

func logMissedInstance(instanceId int64, startTime time.Time) {
	instance_storage.WriteMissedLog(instanceId)
	elapsed := time.Since(startTime).Seconds()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := missedInstanceRateLimiter.Wait(ctx)

	if err == nil {
		fields := []discord.Field{{
			Name:  "Instance Id",
			Value: fmt.Sprintf("`%d`", instanceId),
		}, {
			Name:  "Time Elapsed",
			Value: discord.FormatDuration(elapsed),
		}}
		// Use custom webhook for red color (error variant)
		webhook := discord.Webhook{
			Embeds: []discord.Embed{{
				Title:     "Unresolved Instance",
				Color:     discord.ColorRed,
				Fields:    fields,
				Timestamp: time.Now().Format(time.RFC3339),
				Footer:    discord.CommonFooter,
			}},
		}
		atlasAlerting.SendCustom(&webhook, "", nil)
	}
	AtlasLogger.Warn("MISSED_PGCR", nil, map[string]any{
		logging.INSTANCE_ID: instanceId,
		logging.DURATION:    fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
	})
}

func logMissedInstanceWarning(instanceId int64, startTime time.Time) {
	elapsed := time.Since(startTime).Seconds()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := missedInstanceRateLimiter.Wait(ctx)

	if err == nil {
		fields := []discord.Field{{
			Name:  "Instance Id",
			Value: fmt.Sprintf("`%d`", instanceId),
		}, {
			Name:  "Time Elapsed",
			Value: discord.FormatDuration(elapsed),
		}}
		atlasAlerting.SendWarning("Unresolved Instance (Warning)", fields, "", nil)
	}
	AtlasLogger.Warn("INSTANCE_RESOLUTION_WARNING", nil, map[string]any{
		logging.INSTANCE_ID: instanceId,
		logging.DURATION:    fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
		logging.ACTION:      "monitoring",
	})
}

func logHigh404Rate(count int, rate float64) {
	fields := []discord.Field{{
		Name:  "Rate",
		Value: fmt.Sprintf("%.3f%%", rate),
	}, {
		Name:  "Count",
		Value: fmt.Sprintf("%d", count),
	}}
	// Use custom webhook for dark orange color
	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title:     "High 404 Rate Detected",
			Color:     discord.ColorDarkOrange,
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	atlasAlerting.SendCustom(&webhook, "", nil)
	AtlasLogger.Warn("HIGH_404_RATE_DETECTED", nil, map[string]any{
		logging.RATE:   rate,
		logging.COUNT:  count,
		logging.ACTION: "gap_supercharge_initiated",
	})
}

func logExitGapSupercharge(percentNotFound float64, p20Lag float64) {
	fields := []discord.Field{{
		Name:  "404 Rate",
		Value: fmt.Sprintf("%.3f%%", percentNotFound),
	}, {
		Name:  "Lag Behind Head (P20)",
		Value: discord.FormatDuration(p20Lag),
	}}
	atlasAlerting.SendStatus("Gap Supercharge Exiting", fields, "GAP_SUPERCHARGE_EXITING", map[string]any{
		"percent_not_found": percentNotFound,
		logging.LAG:         discord.FormatDuration(p20Lag),
	})
}

func logRunawayError(percentNotFound float64, currentInstanceId, latestInstanceId int64, latestInstanceCompletionDate time.Time) {
	ping := fmt.Sprintf("<@&%s>", env.AlertsRoleID)
	webhook := discord.Webhook{
		Content: &ping,
		Embeds: []discord.Embed{
			{
				Title: "Runaway Error",
				Color: 15548997, // Red
				Fields: []discord.Field{{
					Name:  "404 Rate",
					Value: fmt.Sprintf("%.3f%%", percentNotFound),
				}, {
					Name:  "Crawling Near Instance Id",
					Value: fmt.Sprintf("`%d`", currentInstanceId),
				}, {
					Name:  "Latest Instance Completion Date",
					Value: fmt.Sprintf("<t:%d:t>", latestInstanceCompletionDate.Unix()),
				}},
				Timestamp: time.Now().Format(time.RFC3339),
				Footer:    discord.CommonFooter,
			},
			{
				Title: "Resetting Channel",
				Color: 3447003, // Blue
				Fields: []discord.Field{{
					Name:  "Returning to Instance Id",
					Value: fmt.Sprintf("`%d`", latestInstanceId),
				}, {
					Name:  "Backtrack Count",
					Value: fmt.Sprintf("%d", currentInstanceId-latestInstanceId),
				}},
				Timestamp: time.Now().Format(time.RFC3339),
				Footer:    discord.CommonFooter,
			}},
	}
	atlasAlerting.SendCustom(&webhook, "", nil)
	AtlasLogger.Error("RUNAWAY_ERROR", nil, map[string]any{
		"percent_not_found":               percentNotFound,
		"current_instance_id":             currentInstanceId,
		"latest_instance_id":              latestInstanceId,
		"latest_instance_completion_date": latestInstanceCompletionDate.Format(time.RFC3339),
		logging.ACTION:                    "resetting_to_latest",
	})
}

func logGapCheckBlockSkip(oldId int64, newId int64) {
	ping := fmt.Sprintf("<@&%s>", env.AlertsRoleID)
	fields := []discord.Field{{
		Name:  "Previous Id",
		Value: fmt.Sprintf("`%d`", oldId),
	}, {
		Name:  "New Starting Id",
		Value: fmt.Sprintf("`%d`", newId),
	}, {
		Name:  "Minimum Block Size",
		Value: fmt.Sprintf("%d", newId-oldId),
	}}
	webhook := discord.Webhook{
		Content: &ping,
		Embeds: []discord.Embed{{
			Title:     "Gap Mode Block Skip",
			Color:     3447003, // Blue
			Fields:    fields,
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	atlasAlerting.SendCustom(&webhook, "", nil)
	AtlasLogger.Info("GAP_MODE_BLOCK_SKIP", map[string]any{
		logging.FROM:   oldId,
		logging.TO:     newId,
		"block_size":   newId - oldId,
		logging.ACTION: "skipping_gap",
	})
}
