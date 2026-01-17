package queueworkers

import (
	"errors"
	"sync"
	"time"

	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/player"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PlayerCrawlTopic creates a new player crawl topic
func PlayerCrawlTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PlayerCrawl,
		MinWorkers:            5,
		MaxWorkers:            70,
		DesiredWorkers:        20,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.5, // Add 50% more workers (more aggressive)
		ScaleDownPercent:      0.1,
		MinWorkersPerStep:     3,
		MaxWorkersPerStep:     25,               // Can add up to 25 workers at once (more aggressive)
		ConsecutiveChecksUp:   1,                // Scale up after just 1 check (immediate)
		ConsecutiveChecksDown: 3,                // More conservative for scale-down
		ScaleCooldown:         30 * time.Second, // Shorter cooldown for faster scaling
		BungieSystemDeps:      []string{"Destiny2", "D2Profiles", "Activities"},
		MaxRetryCount:         12, // Important for player data collection
	}, processPlayerCrawl)
}

// processPlayerCrawl handles player crawl messages
func processPlayerCrawl(worker processing.WorkerInterface, message amqp.Delivery) error {
	membershipId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	if !tryStartPlayerCrawl(membershipId) {
		worker.Debug("PLAYER_CRAWL_DEDUPED", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			"reason":              "inflight",
		})
		return nil
	}
	defer finishPlayerCrawl(membershipId)

	fields := map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
	}
	worker.Debug("PROCESSING_PLAYER_CRAWL", fields)

	wasUpdated, err := player.Crawl(worker.Context(), membershipId)
	if err != nil {
		worker.Warn("PLAYER_CRAWL_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}

	status := "success"
	if !wasUpdated {
		status = "not_updated"
	}

	worker.Debug("PLAYER_CRAWL_COMPLETE", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.STATUS:        status,
	})
	return nil
}

const (
	playerCrawlInflightTTL   = 10 * time.Second
	playerCrawlSweepInterval = 30 * time.Second
)

type playerCrawlEntry struct {
	expires time.Time
}

var (
	playerCrawlInflight sync.Map
)

func tryStartPlayerCrawl(membershipId int64) bool {
	now := time.Now()
	if value, ok := playerCrawlInflight.Load(membershipId); ok {
		entry := value.(playerCrawlEntry)
		if now.Before(entry.expires) {
			return false
		}
		playerCrawlInflight.Delete(membershipId)
	}

	playerCrawlInflight.Store(membershipId, playerCrawlEntry{
		expires: now.Add(playerCrawlInflightTTL),
	})
	return true
}

func finishPlayerCrawl(membershipId int64) {
	playerCrawlInflight.Delete(membershipId)
}

func init() {
	logger := logging.NewLogger("PLAYER_CRAWL_CACHE")
	go func() {
		ticker := time.NewTicker(playerCrawlSweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			playerCrawlInflight.Range(func(key, value any) bool {
				entry := value.(playerCrawlEntry)
				if now.After(entry.expires) {
					err := errors.New("item expired")
					// ideally, all items should get cleaned up by the time they expire, but just in case, log a warning
					logger.Warn("PLAYER_CRAWL_ITEM_EXPIRED", err, map[string]any{
						logging.MEMBERSHIP_ID: key,
						"expires":             entry.expires,
					})
					playerCrawlInflight.Delete(key)
				}
				return true
			})
		}
	}()
}
