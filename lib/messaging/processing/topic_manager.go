package processing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/monitoring"
	"raidhub/lib/utils"
	"raidhub/lib/utils/logging"
)

// scalingState tracks the state for dead zone and cooldown logic
type scalingState struct {
	lastScaleTime         time.Time
	lastQueueDepth        int
	lastScaleDirection    string // "up", "down", "none"
	consecutiveChecksUp   int
	consecutiveChecksDown int
}

// TopicManager manages a single topic with self-scaling worker goroutines
type TopicManager struct {
	topic  Topic
	config TopicConfig

	// All fields are private to encapsulate the manager's internal state
	activeWorkers int
	workers       map[int]*Worker // Map of worker ID to Worker struct
	mutex         sync.RWMutex
	wg            *utils.ReadOnlyWaitGroup
	ctx           context.Context
	logger        logging.Logger

	// Scaling state for dead zone and cooldown
	scalingState scalingState
	scalingMutex sync.Mutex // Separate mutex for scaling state to avoid blocking worker operations
}

func (tm *TopicManager) addTopicFields(fields map[string]any) map[string]any {
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["topic"] = tm.config.QueueName
	return fields
}

// Info logs a structured informational message
func (tm *TopicManager) Info(message string, fields map[string]any) {
	tm.logger.Info(message, tm.addTopicFields(fields))
}

func (tm *TopicManager) Debug(message string, fields map[string]any) {
	tm.logger.Debug(message, tm.addTopicFields(fields))
}

func (tm *TopicManager) Warn(message string, fields map[string]any) {
	tm.logger.Warn(message, tm.addTopicFields(fields))
}

func (tm *TopicManager) Error(message string, fields map[string]any) {
	tm.logger.Error(message, tm.addTopicFields(fields))
}

// GetInitialWorkers returns the initial worker count for this topic
func (tm *TopicManager) GetInitialWorkers() int {
	if env.IsContestWeekend {
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

// Stop gracefully stops all workers and waits for them to finish
func (tm *TopicManager) WaitForWorkersToFinish() {
	// Wait until context is cancelled
	<-tm.ctx.Done()

	tm.mutex.Lock()
	tm.Debug("WAITING_FOR_WORKERS_TO_FINISH", nil)
	for _, worker := range tm.workers {
		worker.Wait()
	}
	tm.Debug("WORKERS_FINISHED", nil)
	tm.mutex.Unlock()
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

	// Set default hysteresis (dead zone) - prevents oscillation
	if topicConfig.ScaleUpHysteresis == 0 {
		// Scale-down threshold is lower than scale-up threshold by default
		topicConfig.ScaleUpHysteresis = topicConfig.ScaleUpThreshold
	}
	if topicConfig.ScaleDownHysteresis == 0 {
		// Scale-down happens at a lower threshold than scale-up
		topicConfig.ScaleDownHysteresis = topicConfig.ScaleDownThreshold
	}

	// Set default cooldown period
	if topicConfig.ScaleCooldown == 0 {
		topicConfig.ScaleCooldown = 2 * time.Minute // Don't scale again within 2 minutes
	}

	// Set default max workers per step
	if topicConfig.MaxWorkersPerStep == 0 {
		topicConfig.MaxWorkersPerStep = 10 // Add/remove at most 10 workers per step
	}

	// Set default min workers per step
	if topicConfig.MinWorkersPerStep == 0 {
		topicConfig.MinWorkersPerStep = 1 // Always add/remove at least 1 worker
	}

	// Set default consecutive checks required
	if topicConfig.ConsecutiveChecksUp == 0 {
		topicConfig.ConsecutiveChecksUp = 2 // Require 2 checks (10 minutes) before scaling up
	}
	if topicConfig.ConsecutiveChecksDown == 0 {
		topicConfig.ConsecutiveChecksDown = 3 // Require 3 checks (15 minutes) before scaling down (more conservative)
	}

	prefix := fmt.Sprintf("%s#manager", topicConfig.QueueName)
	topicManager := &TopicManager{
		topic:         topic,
		activeWorkers: 0,
		workers:       make(map[int]*Worker),
		config:        topicConfig,
		wg:            managerConfig.Wg,
		ctx:           managerConfig.Context,
		logger:        logging.NewLogger(prefix),
		scalingState: scalingState{
			lastScaleDirection: "none",
		},
	}

	// Check if this is a contest weekend
	isContestWeekend := env.IsContestWeekend
	initialWorkers := topicConfig.DesiredWorkers
	if isContestWeekend {
		initialWorkers = topicConfig.ContestWeekendWorkers
		topicManager.logger.Info("CONTEST_WEEKEND_SCALING", map[string]any{
			logging.WORKER_COUNT: initialWorkers,
		})
	}

	// Start with initial worker count
	err := topicManager.scaleToInitial(initialWorkers)
	if err != nil {
		return nil, err
	}

	// Export initial worker count metric
	monitoring.QueueWorkerCount.WithLabelValues(topicConfig.QueueName).Set(float64(initialWorkers))

	// Start goroutine to manually cancel all workers when TopicManager context is cancelled
	go topicManager.handleShutdown()

	// Start self-scaling monitor

	// Skip autoscaling during contest weekends
	if !env.IsContestWeekend {
		go func() {
			topicManager.monitorSelfScaling()
		}()
	}

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
			worker, err := tm.startWorkerGoroutine(i)
			if err != nil {
				tm.Warn("WORKER_START_ERROR", map[string]any{
					"worker_id":   i,
					logging.ERROR: err.Error(),
				})
				continue
			}
			tm.workers[i] = worker
			tm.activeWorkers++
		}
		if !isInitial {
			tm.Info("WORKERS_SCALED_UP", map[string]any{
				logging.FROM: currentWorkers,
				logging.TO:   tm.activeWorkers,
			})
			// Update metrics
			monitoring.QueueWorkerCount.WithLabelValues(tm.config.QueueName).Set(float64(tm.activeWorkers))
		}
	} else if targetWorkers < currentWorkers {
		// Scale down - remove workers
		for i := currentWorkers - 1; i >= targetWorkers; i-- {
			if w, exists := tm.workers[i]; exists {
				w.ScaleIn() // Cancel the context
				delete(tm.workers, i)
				tm.activeWorkers--
			}
		}
		if !isInitial {
			tm.Info("WORKERS_SCALED_DOWN", map[string]any{
				logging.FROM: currentWorkers,
				logging.TO:   tm.activeWorkers,
			})
			// Update metrics
			monitoring.QueueWorkerCount.WithLabelValues(tm.config.QueueName).Set(float64(tm.activeWorkers))
		}
	}

	return nil
}

// startWorkerGoroutine starts a single worker goroutine and returns the Worker
func (tm *TopicManager) startWorkerGoroutine(workerID int) (*Worker, error) {
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

	// Create worker context independently - TopicManager will manually cancel it on shutdown
	workerCtx, workerCancel := context.WithCancelCause(context.Background())

	// Create Worker struct for this goroutine
	topicManagerConfig := tm.GetContext()
	prefix := fmt.Sprintf("%s#%d", tm.config.QueueName, workerID)
	worker := &Worker{
		ID:          workerID,
		QueueName:   tm.config.QueueName,
		Config:      topicManagerConfig,
		logger:      logging.NewLogger(prefix),
		ctx:         workerCtx,
		cancel:      workerCancel,
		amqpChannel: ch,
		channel:     msgs,
		processor:   tm.topic.processor,
		done:        make(chan struct{}),
	}

	go worker.Run()

	return worker, nil
}

// monitorSelfScaling monitors queue depth and scales workers with improved dead zone and cooldown logic
func (tm *TopicManager) monitorSelfScaling() {
	ticker := time.NewTicker(tm.config.ScaleCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		queueDepth, err := tm.getQueueDepth()
		if err != nil {
			tm.logger.Error("QUEUE_DEPTH_ERROR", map[string]any{
				logging.ERROR: err.Error(),
			})
			continue
		}

		tm.mutex.RLock()
		currentWorkers := tm.activeWorkers
		tm.mutex.RUnlock()

		// Export metrics
		monitoring.QueueDepth.WithLabelValues(tm.config.QueueName).Set(float64(queueDepth))
		monitoring.QueueWorkerCount.WithLabelValues(tm.config.QueueName).Set(float64(currentWorkers))

		// Check if scaling should happen with dead zone and cooldown
		shouldScale, direction, targetWorkers := tm.shouldScaleWorkers(queueDepth, currentWorkers)
		if !shouldScale {
			continue
		}

		tm.Info("SCALING_WORKERS", map[string]any{
			logging.QUEUE_DEPTH: queueDepth,
			logging.FROM:        currentWorkers,
			logging.TO:          targetWorkers,
			"direction":         direction,
		})

		// Record scaling decision metric
		monitoring.QueueScalingDecisions.WithLabelValues(tm.config.QueueName, direction).Inc()

		err = tm.scaleTo(targetWorkers)
		if err != nil {
			tm.Warn("SCALING_FAILED", map[string]any{
				logging.ERROR:    err.Error(),
				"target_workers": targetWorkers,
			})
		} else {
			// Update scaling state after successful scaling
			tm.scalingMutex.Lock()
			tm.scalingState.lastScaleTime = time.Now()
			tm.scalingState.lastScaleDirection = direction
			tm.scalingState.lastQueueDepth = queueDepth
			tm.scalingState.consecutiveChecksUp = 0
			tm.scalingState.consecutiveChecksDown = 0
			tm.scalingMutex.Unlock()
		}
	}
}

// shouldScaleWorkers determines if scaling should happen based on dead zone, hysteresis, and cooldown
// Returns: (shouldScale bool, direction string, targetWorkers int)
func (tm *TopicManager) shouldScaleWorkers(queueDepth, currentWorkers int) (bool, string, int) {
	tm.scalingMutex.Lock()
	defer tm.scalingMutex.Unlock()

	now := time.Now()

	// Check cooldown period: don't scale again within ScaleCooldown
	if now.Sub(tm.scalingState.lastScaleTime) < tm.config.ScaleCooldown {
		return false, "none", currentWorkers
	}

	// Check if we should scale up (using hysteresis)
	if queueDepth > tm.config.ScaleUpThreshold && currentWorkers < tm.config.MaxWorkers {
		// Require consecutive checks above threshold
		if tm.scalingState.lastScaleDirection != "up" || tm.scalingState.lastQueueDepth <= tm.config.ScaleUpThreshold {
			tm.scalingState.consecutiveChecksUp = 1
		} else {
			tm.scalingState.consecutiveChecksUp++
		}
		tm.scalingState.lastQueueDepth = queueDepth
		tm.scalingState.lastScaleDirection = "up"

		if tm.scalingState.consecutiveChecksUp >= tm.config.ConsecutiveChecksUp {
			// Calculate workers to add with bounds
			workersToAdd := max(tm.config.MinWorkersPerStep, int(float64(currentWorkers)*tm.config.ScaleUpPercent))
			workersToAdd = min(workersToAdd, tm.config.MaxWorkersPerStep)
			targetWorkers := min(currentWorkers+workersToAdd, tm.config.MaxWorkers)
			return true, "up", targetWorkers
		}
		return false, "up", currentWorkers
	}

	// Check if we should scale down (using hysteresis - more conservative)
	if queueDepth < tm.config.ScaleDownThreshold && currentWorkers > tm.config.MinWorkers {
		// Require consecutive checks below threshold (more checks for scale-down)
		if tm.scalingState.lastScaleDirection != "down" || tm.scalingState.lastQueueDepth >= tm.config.ScaleDownThreshold {
			tm.scalingState.consecutiveChecksDown = 1
		} else {
			tm.scalingState.consecutiveChecksDown++
		}
		tm.scalingState.lastQueueDepth = queueDepth
		tm.scalingState.lastScaleDirection = "down"

		if tm.scalingState.consecutiveChecksDown >= tm.config.ConsecutiveChecksDown {
			// Calculate workers to remove with bounds
			workersToRemove := max(tm.config.MinWorkersPerStep, int(float64(currentWorkers)*tm.config.ScaleDownPercent))
			workersToRemove = min(workersToRemove, tm.config.MaxWorkersPerStep)
			targetWorkers := max(currentWorkers-workersToRemove, tm.config.MinWorkers)
			return true, "down", targetWorkers
		}
		return false, "down", currentWorkers
	}

	// Queue depth is in dead zone (between ScaleDownThreshold and ScaleUpThreshold)
	// Reset consecutive checks
	tm.scalingState.consecutiveChecksUp = 0
	tm.scalingState.consecutiveChecksDown = 0
	tm.scalingState.lastQueueDepth = queueDepth
	tm.scalingState.lastScaleDirection = "none"
	return false, "none", currentWorkers
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

// handleShutdown watches the TopicManager context and manually cancels all workers when shutdown is requested
func (tm *TopicManager) handleShutdown() {
	<-tm.ctx.Done()
	cause := context.Cause(tm.ctx)

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	reason := "unknown"
	if cause != nil {
		reason = cause.Error()
	}
	tm.Debug("CANCELLING_ALL_WORKERS", map[string]any{
		logging.REASON: reason,
	})

	// Manually cancel all worker contexts with the shutdown cause
	for _, worker := range tm.workers {
		worker.cancel(cause)
	}
}
