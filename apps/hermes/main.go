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

var logger = logging.NewLogger("Hermes")

func main() {
	topicType := flag.String("topic", "", "Name of topic to run. If empty, starts all topics.")

	flag.Parse()

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
		logger.Info(logging.RECEIVED_SHUTDOWN_SIGNAL, map[string]any{
			"signal": sig.String(),
		})
		cancel(fmt.Errorf("shutdown_requested"))
		mutex.Unlock()
	}()

	monitoring.RegisterHermesMetrics()

	logger.Debug(logging.WAITING_ON_CONNECTIONS, map[string]any{
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
		logger.Info("STARTING_ALL_TOPICS", map[string]any{})
		for _, t := range topics {
			// Check if shutdown was requested
			mutex.Lock()
			select {
			case <-ctx.Done():
				logger.Warn("STARTUP_INTERRUPTED", map[string]any{})
				mutex.Unlock()
				goto shutdown
			default:
			}

			logger.Info(STARTING_TOPIC, map[string]any{
				"topic": t.Config.QueueName,
				"mode":  "all",
			})
			tm, err := startTopicManager(t, topicManagerConfig{
				context: ctx,
				wg:      nil,
			})
			if err != nil {
				logger.Error(FAILED_TO_START_TOPIC, map[string]any{
					logging.ERROR: err.Error(),
					"topic":       t.Config.QueueName,
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
		logger.Info(STARTING_TOPIC, map[string]any{
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
			logger.Fatal("UNKNOWN_TOPIC_TYPE", map[string]any{
				"topic": *topicType,
			})
			return
		}

		mutex.Lock()
		logger.Info(STARTING_TOPIC, map[string]any{
			"topic": topicInstance.Config.QueueName,
			"mode":  "individual",
		})
		tm, err := startTopicManager(topicInstance, topicManagerConfig{
			context: ctx,
		})
		if err != nil {
			logger.Fatal(FAILED_TO_START_TOPIC, map[string]any{
				"topic": topicInstance.Config.QueueName,
				"error": err.Error(),
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
	logger.Info("ALL_TOPICS_STARTED", map[string]any{})
	<-ctx.Done()

shutdown:
	logger.Info("SHUTTING_DOWN", map[string]any{
		"context":        ctx.Err().Error(),
		"topic_managers": len(topicManagers),
	})

	for _, tm := range topicManagers {
		tm.WaitForWorkersToFinish()
	}

	logger.Info("SHUTDOWN_COMPLETE", map[string]any{})
}
