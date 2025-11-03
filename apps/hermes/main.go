package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"raidhub/lib/database/clickhouse"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/publishing"
	qw "raidhub/lib/messaging/queue-workers"
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/monitoring"
	"raidhub/lib/utils/logging"
	"sync"
	"syscall"
)

const (
	STARTING_TOPIC        = "STARTING_TOPIC"
	STARTED_TOPIC         = "STARTED_TOPIC"
	FAILED_TO_START_TOPIC = "FAILED_TO_START_TOPIC"
)

var HermesLogger = logging.NewLogger("hermes")

func main() {

	topicType := flag.String("topic", "", "Name of topic to run. If empty, starts all topics.")
	metricsPort := flag.String("metrics-port", "", "Override metrics port (default: env HERMES_METRICS_PORT)")

	logging.ParseFlags()

	flushSentry, recoverSentry := HermesLogger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	// Create a cancellable context
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// preserves operational ordering of logs and shutdowns
	mutex := sync.Mutex{}
	go func() {
		sig := <-sigChan
		mutex.Lock()
		HermesLogger.Info("RECEIVED_SHUTDOWN_SIGNAL", map[string]any{
			"signal": sig.String(),
		})
		cancel(fmt.Errorf("shutdown_requested"))
		mutex.Unlock()
	}()

	monitoring.RegisterHermesMetrics(*metricsPort)

	HermesLogger.Debug("WAITING_ON_CONNECTIONS", map[string]any{
		"services": []string{"postgres", "clickhouse", "rabbit", "publishing"},
	})
	postgres.Wait()
	clickhouse.Wait()
	rabbit.Wait()
	publishing.Wait()

	topics := []processing.Topic{
		qw.PlayerCrawlTopic(),
		qw.PgcrBlockedTopic(),
		qw.ActivityHistoryTopic(),
		qw.CharacterFillTopic(),
		qw.ClanCrawlTopic(),
		qw.PgcrCrawlTopic(),
		qw.InstanceCheatCheckTopic(),
		qw.InstanceStoreTopic(),
	}

	var topicManagers []*TopicManager

	if *topicType == "" {
		// Start all topics
		HermesLogger.Info("STARTING_ALL_TOPICS", map[string]any{})
		for _, t := range topics {
			// Check if shutdown was requested
			mutex.Lock()
			select {
			case <-ctx.Done():
				HermesLogger.Warn("STARTUP_INTERRUPTED", nil, nil)
				mutex.Unlock()
				goto shutdown
			default:
			}

			HermesLogger.Info(STARTING_TOPIC, map[string]any{
				"topic": t.Config.QueueName,
				"mode":  "all",
			})
			tm, err := startTopicManager(t, ctx)
			if err != nil {
				HermesLogger.Error(FAILED_TO_START_TOPIC, err, map[string]any{
					"topic": t.Config.QueueName,
				})
			} else {
				topicManagers = append(topicManagers, tm)
				tm.Info(STARTED_TOPIC, map[string]any{
					"mode": "all",
				})
			}
			mutex.Unlock()
		}
	} else {
		// Start a specific topic
		HermesLogger.Info(STARTING_TOPIC, map[string]any{
			"topic": *topicType,
			"mode":  "individual",
		})

		var topicInstance processing.Topic
		var found bool
		for _, t := range topics {
			if t.Config.QueueName == *topicType {
				topicInstance = t
				found = true
				break
			}
		}

		if !found {
			HermesLogger.Fatal("UNKNOWN_TOPIC_TYPE", nil, map[string]any{
				"topic": *topicType,
			})
			return
		}

		mutex.Lock()
		HermesLogger.Info(STARTING_TOPIC, map[string]any{
			"topic": topicInstance.Config.QueueName,
			"mode":  "individual",
		})
		tm, err := startTopicManager(topicInstance, ctx)
		if err != nil {
			HermesLogger.Fatal(FAILED_TO_START_TOPIC, err, map[string]any{
				"topic": topicInstance.Config.QueueName,
			})
			return
		} else {
			topicManagers = append(topicManagers, tm)
			tm.Info(STARTED_TOPIC, map[string]any{
				"mode": "individual",
			})
		}
		mutex.Unlock()
	}

	// Keep running until cancelled
	HermesLogger.Info("ALL_TOPICS_STARTED", map[string]any{})
	<-ctx.Done()

shutdown:
	HermesLogger.Info("SHUTTING_DOWN", map[string]any{
		"context":        ctx.Err().Error(),
		"topic_managers": len(topicManagers),
	})

	for _, tm := range topicManagers {
		tm.WaitForWorkersToFinish()
	}

	HermesLogger.Info("SHUTDOWN_COMPLETE", map[string]any{})
}
