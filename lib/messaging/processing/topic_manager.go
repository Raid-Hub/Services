package processing

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/utils"
)

// TopicManager manages a single topic with self-scaling worker goroutines
type TopicManager struct {
	topic  Topic
	config TopicConfig

	// All fields are private to encapsulate the manager's internal state
	activeWorkers int
	workers       map[int]context.CancelFunc
	mutex         sync.RWMutex
	wg            *utils.ReadOnlyWaitGroup
	ctx           context.Context
	logger        utils.Logger
}

// InfoF logs a formatted informational message
func (tm *TopicManager) InfoF(format string, args ...any) {
	tm.logger.InfoF(format, args...)
}

// GetInitialWorkers returns the initial worker count for this topic
func (tm *TopicManager) GetInitialWorkers() int {
	if isContestWeekend() {
		return tm.config.ContestWeekendWorkers
	}
	return tm.config.DesiredWorkers
}

// TopicManagerConfig contains configuration and resources for a TopicManager
type TopicManagerConfig struct {
	Context context.Context          // Go context for cancellation and timeouts
	Wg      *utils.ReadOnlyWaitGroup // Wait group for coordinating API calls
}

func (tm *TopicManager) GetContext() TopicManagerConfig {
	return TopicManagerConfig{
		Context: tm.ctx,
		Wg:      tm.wg,
	}
}

// StartTopicManager starts a TopicManager with self-scaling worker goroutines
func StartTopicManager(topic Topic, managerConfig TopicManagerConfig) (*TopicManager, error) {
	// Get the topic config directly from the topic
	topicConfig := topic.Config

	// Set default scaling configuration if not specified
	if topicConfig.ScaleUpThreshold == 0 {
		topicConfig.ScaleUpThreshold = 100 // Scale up when queue has 100+ messages
	}
	if topicConfig.ScaleDownThreshold == 0 {
		topicConfig.ScaleDownThreshold = 10 // Scale down when queue has <10 messages
	}
	if topicConfig.ScaleUpPercent == 0 {
		topicConfig.ScaleUpPercent = 0.2 // Add 20% more workers
	}
	if topicConfig.ScaleDownPercent == 0 {
		topicConfig.ScaleDownPercent = 0.1 // Remove 10% of workers
	}
	if topicConfig.ScaleCheckInterval == 0 {
		topicConfig.ScaleCheckInterval = 5 * time.Minute
	}

	topicManager := &TopicManager{
		topic:         topic,
		activeWorkers: 0,
		workers:       make(map[int]context.CancelFunc),
		config:        topicConfig,
		wg:            managerConfig.Wg,
		ctx:           managerConfig.Context,
		logger:        utils.NewLogger(topicConfig.QueueName),
	}

	// Check if this is a contest weekend
	isContestWeekend := isContestWeekend()
	initialWorkers := topicConfig.DesiredWorkers
	if isContestWeekend {
		initialWorkers = topicConfig.ContestWeekendWorkers
		topicManager.logger.InfoF("Contest weekend detected - scaling workers count=%d", initialWorkers)
	}

	// Start with initial worker count
	err := topicManager.scaleToInitial(initialWorkers)
	if err != nil {
		return nil, err
	}

	// Start self-scaling monitor
	go topicManager.monitorSelfScaling()

	return topicManager, nil
}

// scaleToInitial scales to the target number of workers during initial startup (no logging)
func (tm *TopicManager) scaleToInitial(targetWorkers int) error {
	return tm.scaleToInternal(targetWorkers, true)
}

// scaleTo scales to the target number of workers
func (tm *TopicManager) scaleTo(targetWorkers int) error {
	return tm.scaleToInternal(targetWorkers, false)
}

// scaleToInternal scales to the target number of workers
func (tm *TopicManager) scaleToInternal(targetWorkers int, isInitial bool) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Ensure we stay within min/max bounds
	if targetWorkers < tm.config.MinWorkers {
		targetWorkers = tm.config.MinWorkers
	}
	if targetWorkers > tm.config.MaxWorkers {
		targetWorkers = tm.config.MaxWorkers
	}

	currentWorkers := tm.activeWorkers

	if targetWorkers > currentWorkers {
		// Scale up - add workers
		for i := currentWorkers; i < targetWorkers; i++ {
			cancelFunc, err := tm.startWorkerGoroutine(i)
			if err != nil {
				tm.logger.ErrorF("Failed to start worker workerID=%d: %v", i, err)
				continue
			}
			tm.workers[i] = cancelFunc
			tm.activeWorkers++
		}
		if !isInitial {
			tm.logger.InfoF("Scaled up workers from=%d to=%d", currentWorkers, tm.activeWorkers)
		}
	} else if targetWorkers < currentWorkers {
		// Scale down - remove workers
		for i := currentWorkers - 1; i >= targetWorkers; i-- {
			if cancelFunc, exists := tm.workers[i]; exists {
				cancelFunc() // Cancel the context
				delete(tm.workers, i)
				tm.activeWorkers--
			}
		}
		if !isInitial {
			tm.logger.InfoF("Scaled down workers from=%d to=%d", currentWorkers, tm.activeWorkers)
		}
	}

	return nil
}

// startWorkerGoroutine starts a single worker goroutine and returns the cancel function
func (tm *TopicManager) startWorkerGoroutine(workerID int) (context.CancelFunc, error) {
	ch, err := rabbit.Conn.Channel()
	if err != nil {
		return nil, err
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		tm.config.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		ch.Close()
		return nil, err
	}

	// Set QoS if needed
	if tm.config.KeepInReady {
		prefetch := max(1, tm.config.PrefetchCount)
		err = ch.Qos(prefetch, 0, false)
		if err != nil {
			ch.Close()
			return nil, err
		}
	}

	msgs, err := ch.Consume(
		q.Name,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		ch.Close()
		return nil, err
	}

	// Create a cancellable context for this worker
	workerCtx, cancelFunc := context.WithCancel(tm.ctx)

	// Create Worker struct for this goroutine
	topicManagerConfig := tm.GetContext()
	prefix := fmt.Sprintf("%s::%d", tm.config.QueueName, workerID)
	worker := &Worker{
		ID:        workerID,
		QueueName: tm.config.QueueName,
		Config:    topicManagerConfig,
		logger:    utils.NewLogger(prefix),
		ctx:       workerCtx,
		channel:   msgs,
		processor: tm.topic.processor,
	}

	// Run the worker loop - close channel when worker exits
	go func() {
		defer ch.Close()
		worker.Run()
	}()

	return cancelFunc, nil
}

// monitorSelfScaling monitors queue depth and scales workers every 5 minutes
func (tm *TopicManager) monitorSelfScaling() {
	ticker := time.NewTicker(tm.config.ScaleCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		// Skip autoscaling during contest weekends
		if isContestWeekend() {
			tm.logger.Warn("Contest weekend active - skipping autoscaling")
			continue
		}

		queueDepth, err := tm.getQueueDepth()
		if err != nil {
			tm.logger.ErrorF("Failed to get queue depth: %v", err)
			continue
		}

		tm.mutex.RLock()
		currentWorkers := tm.activeWorkers
		tm.mutex.RUnlock()

		var targetWorkers int

		if queueDepth > tm.config.ScaleUpThreshold && currentWorkers < tm.config.MaxWorkers {
			// Scale up by percentage
			workersToAdd := max(1, int(float64(currentWorkers)*tm.config.ScaleUpPercent))
			targetWorkers = currentWorkers + workersToAdd
		} else if queueDepth < tm.config.ScaleDownThreshold && currentWorkers > tm.config.MinWorkers {
			// Scale down by percentage
			workersToRemove := max(1, int(float64(currentWorkers)*tm.config.ScaleDownPercent))
			targetWorkers = currentWorkers - workersToRemove
		} else {
			// No scaling needed
			continue
		}

		tm.logger.InfoF("Scaling workers queueDepth=%d from=%d to=%d", queueDepth, currentWorkers, targetWorkers)
		tm.scaleTo(targetWorkers)
	}
}

// getQueueDepth gets the current queue depth
func (tm *TopicManager) getQueueDepth() (int, error) {
	ch, err := rabbit.Conn.Channel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		tm.config.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return 0, err
	}

	return q.Messages, nil
}

// isContestWeekend checks if we're in a contest weekend
func isContestWeekend() bool {
	// Check environment variable for contest weekend mode
	contestMode := os.Getenv("CONTEST_WEEKEND")
	return contestMode == "true" || contestMode == "1"
}
