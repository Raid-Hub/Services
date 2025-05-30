package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"raidhub/packages/discord"
	"raidhub/packages/monitoring"
	"raidhub/packages/pgcr"

	"golang.org/x/time/rate"
)

var (
	_atlasWebhookURL          string
	once                      sync.Once
	missedInstanceRateLimiter = rate.NewLimiter(rate.Every(time.Minute), 1)
)

func getAtlasWebhookURL() string {
	once.Do(func() {
		_atlasWebhookURL = os.Getenv("ATLAS_WEBHOOK_URL")
	})
	return _atlasWebhookURL
}

func handlePanic(r interface{}) {
	content := fmt.Sprintf("<@&%s>", os.Getenv("ALERTS_ROLE_ID"))
	webhook := discord.Webhook{
		Content: &content,
		Embeds: []discord.Embed{{
			Title: "Fatal error in Atlas",
			Color: 10038562, // DarkRed
			Fields: []discord.Field{{
				Name:  "Error",
				Value: fmt.Sprintf("%s", r),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Fatal error in Atlas: %s", r)
}

func sendStartUpAlert() {
	msg := "Info: Starting up..."
	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title:     "Starting up...",
			Color:     3447003, // Blue
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Println(msg)
}

func logIntervalState(medianLag float64, countWorkers int, percentNotFound, errorPercentage float64) {
	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "Status Update",
			Color: 9807270, // Gray
			Fields: []discord.Field{{
				Name:  "Lag Behind Head",
				Value: fmt.Sprintf("%1.f seconds", medianLag),
			}, {
				Name:  "404 Percentage",
				Value: fmt.Sprintf("%.3f%%", percentNotFound),
			}, {
				Name:  "Error Percentage",
				Value: fmt.Sprintf("%.3f%%", errorPercentage),
			}, {
				Name:  "Workers Used",
				Value: fmt.Sprintf("%d", countWorkers),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Info: Head is behind by %1.f seconds with %.3f%% not found using %d workers ", medianLag, percentNotFound, countWorkers)
}

func logWorkersStarting(numWorkers int, period int, latestId int64) {
	monitoring.ActiveWorkers.Set(float64(numWorkers))

	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "Workers Starting",
			Color: 9807270, // Gray
			Fields: []discord.Field{{
				Name:  "Count",
				Value: fmt.Sprintf("%d", numWorkers),
			}, {
				Name:  "Period",
				Value: fmt.Sprintf("%d", period),
			}, {
				Name:  "Current Instance Id",
				Value: fmt.Sprintf("`%d`", latestId),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Info: %d workers starting at %d", numWorkers, latestId)
}

func logMissedInstance(instanceId int64, startTime time.Time) {
	pgcr.WriteMissedLog(instanceId)
	elapsed := time.Since(startTime).Seconds()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := missedInstanceRateLimiter.Wait(ctx)

	if err == nil {
		webhook := discord.Webhook{
			Embeds: []discord.Embed{{
				Title: "Unresolved Instance",
				Color: 15548997, // Red
				Fields: []discord.Field{{
					Name:  "Instance Id",
					Value: fmt.Sprintf("`%d`", instanceId),
				}, {
					Name:  "Time Elapsed",
					Value: fmt.Sprintf("%1.f seconds", elapsed),
				}},
				Timestamp: time.Now().Format(time.RFC3339),
				Footer:    discord.CommonFooter,
			}},
		}

		discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	}
	log.Printf("Missed PGCR %d after %1.f seconds", instanceId, time.Since(startTime).Seconds())
}

func logMissedInstanceWarning(instanceId int64, startTime time.Time) {
	elapsed := time.Since(startTime).Seconds()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := missedInstanceRateLimiter.Wait(ctx)

	if err == nil {
		webhook := discord.Webhook{
			Embeds: []discord.Embed{{
				Title: "Unresolved Instance (Warning)",
				Color: 15105570, // Orange
				Fields: []discord.Field{{
					Name:  "Instance Id",
					Value: fmt.Sprintf("`%d`", instanceId),
				}, {
					Name:  "Time Elapsed",
					Value: fmt.Sprintf("%1.f seconds", elapsed),
				}},
				Timestamp: time.Now().Format(time.RFC3339),
				Footer:    discord.CommonFooter,
			}},
		}
		discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	}
	log.Printf("Warning: instance id %d has not resolved in %1.f seconds", instanceId, time.Since(startTime).Seconds())
}

func logHigh404Rate(count int, rate float64) {
	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "High 404 Rate Detected",
			Color: 16737792, // Dark Orange
			Fields: []discord.Field{{
				Name:  "Rate",
				Value: fmt.Sprintf("%.3f%%", rate),
			}, {
				Name:  "Count",
				Value: fmt.Sprintf("%d", count),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Warning: High 404 rate of %.3f%% with %d instances", rate, count)
}

func logExitGapSupercharge(percentNotFound float64, medianLag float64) {
	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "Gap Supercharge Exiting",
			Color: 9807270, // Gray
			Fields: []discord.Field{{
				Name:  "404 Rate",
				Value: fmt.Sprintf("%.3f%%", percentNotFound),
			}, {
				Name:  "Median Lag",
				Value: fmt.Sprintf("%1.f seconds", medianLag),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Info: Gap check exiting with 404 rate %.3f%%, behind by %1.f seconds", percentNotFound, medianLag)

}

func logRunawayError(percentNotFound float64, currentInstanceId, latestInstanceId int64, latestInstanceCompletionDate time.Time) {
	ping := fmt.Sprintf("<@&%s>", os.Getenv("ALERTS_ROLE_ID"))
	webhook := discord.Webhook{
		Content: &ping,
		Embeds: []discord.Embed{{
			Title: "Runaway Error",
			Color: 15548997, // Red
			Fields: []discord.Field{{
				Name:  "404 Rate",
				Value: fmt.Sprintf("%.3f%%", percentNotFound),
			}, {
				Name:  "Crawling Near Instance Id",
				Value: fmt.Sprintf("`%d`", currentInstanceId),
			}, {
				Name:  "Latest Instance Id",
				Value: fmt.Sprintf("`%d`", latestInstanceId),
			}, {
				Name:  "Latest Instance Completion Date",
				Value: fmt.Sprintf("%d minutes ago", int(time.Since(latestInstanceCompletionDate).Minutes())),
			}, {
				Name:  "Ahead By",
				Value: fmt.Sprintf("%d", currentInstanceId-latestInstanceId),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Error: Atlas is runaway with 404 rate %.3f%%, latest instance id %d, latest instance completion date %s", percentNotFound, latestInstanceId, latestInstanceCompletionDate.Format(time.RFC3339))
}

func logGapCheckBlockSkip(oldId int64, newId int64) {
	ping := fmt.Sprintf("<@&%s>", os.Getenv("ALERTS_ROLE_ID"))
	webhook := discord.Webhook{
		Content: &ping,
		Embeds: []discord.Embed{{
			Title: "Gap Mode Block Skip",
			Color: 3447003, // Blue
			Fields: []discord.Field{{
				Name:  "Previous Id",
				Value: fmt.Sprintf("`%d`", oldId),
			}, {
				Name:  "New Starting Id",
				Value: fmt.Sprintf("`%d`", newId),
			}, {
				Name:  "Minimum Block Size",
				Value: fmt.Sprintf("%d", newId-oldId),
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(getAtlasWebhookURL(), &webhook)
	log.Printf("Info: Gap mode block skip from %d to %d", oldId, newId)
}
