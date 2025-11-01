package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/monitoring/hermes_metrics"
	"raidhub/lib/utils"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"

	amqp "github.com/rabbitmq/amqp091-go"
)

// scalingState tracks the state for dead zone and cooldown logic
type scalingState struct {
	lastScaleTime         time.Time
	lastQueueDepth        int
	lastScaleDirection    string // "up", "down", "none"
	consecutiveChecksUp   int
	consecutiveChecksDown int
	cachedQueueDepth      int       // Cached queue depth for fallback
	cachedQueueDepthTime  time.Time // When cached depth was retrieved
}

// TopicManager manages a single topic with self-scaling worker goroutines
type TopicManager struct {
	topic         processing.Topic
	config        processing.TopicConfig
	managerConfig topicManagerConfig

	// All fields are private to encapsulate the manager's internal state
	activeWorkers int
	workers       map[int]*Worker // Map of worker ID to Worker struct
	mutex         sync.RWMutex
	logger        logging.Logger

	// Scaling state for dead zone and cooldown
	scalingState scalingState
	scalingMutex sync.Mutex // Separate mutex for scaling state to avoid blocking worker operations

	// Dedicated channel for queue depth checks (reused, thread-safe)
	depthCheckChannel *amqp.Channel
	depthChannelMutex sync.RWMutex

	// API availability wait group - used to block workers when API is disabled
	apiAvailabilityWG sync.WaitGroup
	apiAvailable       bool // Track if API is currently available
	apiMutex           sync.RWMutex
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

type topicManagerConfig struct {
	context context.Context
	wg      *utils.ReadOnlyWaitGroup
}

func (tm *TopicManager) Context() context.Context {
	return tm.managerConfig.context
}

// Stop gracefully stops all workers and waits for them to finish
func (tm *TopicManager) WaitForWorkersToFinish() {
	// Wait until context is cancelled
	<-tm.Context().Done()

	tm.mutex.Lock()
	tm.Debug("WAITING_FOR_WORKERS_TO_FINISH", nil)
	for _, worker := range tm.workers {
		worker.Wait()
	}
	tm.Debug("WORKERS_FINISHED", nil)
	tm.mutex.Unlock()
}

// startTopicManager starts a TopicManager with self-scaling worker goroutines
func startTopicManager(topic processing.Topic, managerConfig topicManagerConfig) (*TopicManager, error) {
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

	topicManager := &TopicManager{
		topic:         topic,
		activeWorkers: 0,
		workers:       make(map[int]*Worker),
		config:        topicConfig,
		managerConfig: managerConfig,
		logger:        HermesLogger,
		scalingState: scalingState{
			lastScaleDirection: "none",
		},
		apiAvailable: true, // Assume API is available on startup
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
	hermes_metrics.QueueWorkerCount.WithLabelValues(topicConfig.QueueName).Set(float64(initialWorkers))

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
			hermes_metrics.QueueWorkerCount.WithLabelValues(tm.config.QueueName).Set(float64(tm.activeWorkers))
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
			hermes_metrics.QueueWorkerCount.WithLabelValues(tm.config.QueueName).Set(float64(tm.activeWorkers))
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
	// Inject the internal API availability wait group into worker config
	apiWG := utils.NewReadOnlyWaitGroup(&tm.apiAvailabilityWG)
	workerManagerConfig := topicManagerConfig{
		context: tm.managerConfig.context,
		wg:      &apiWG,
	}

	worker := &Worker{
		ID:            workerID,
		QueueName:     tm.config.QueueName,
		managerConfig: workerManagerConfig,
		Topic:         tm.topic,
		logger:        HermesLogger,
		ctx:           workerCtx,
		cancel:        workerCancel,
		amqpChannel:   ch,
		channel:       msgs,
		processor:     tm.topic.Processor,
		done:          make(chan struct{}),
	}

	go worker.Run()

	return worker, nil
}

// monitorSelfScaling monitors queue depth and scales workers with improved dead zone and cooldown logic
func (tm *TopicManager) monitorSelfScaling() {
	ticker := time.NewTicker(tm.config.ScaleCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		// Check if Bungie API is enabled
		isAPIDisabled, err := tm.checkBungieAPIDisabled()
		if err != nil {
			tm.logger.Error("BUNGIE_SETTINGS_CHECK_ERROR", map[string]any{
				logging.ERROR: err.Error(),
			})
			// On error, continue with normal scaling logic
		} else {
			tm.apiMutex.Lock()
			wasAvailable := tm.apiAvailable
			tm.apiAvailable = !isAPIDisabled
			tm.apiMutex.Unlock()

			// If API status changed, update the wait group
			if wasAvailable && isAPIDisabled {
				// API was enabled, now disabled - block workers by keeping WG waiting
				tm.Info("BUNGIE_API_DISABLED", map[string]any{
					"action": "blocking_workers",
				})
				tm.apiAvailabilityWG.Add(1)
			} else if !wasAvailable && !isAPIDisabled {
				// API was disabled, now enabled - unblock workers
				tm.Info("BUNGIE_API_ENABLED", map[string]any{
					"action": "unblocking_workers",
				})
				tm.apiAvailabilityWG.Done()
			}
		}

		queueDepth, err := tm.getQueueDepthWithRetry()
		if err != nil {
			tm.logger.Error("QUEUE_DEPTH_ERROR", map[string]any{
				logging.ERROR: err.Error(),
			})
			// Use cached queue depth as fallback if available and recent (< 2 check intervals)
			tm.scalingMutex.Lock()
			cachedDepth := tm.scalingState.cachedQueueDepth
			cacheAge := time.Since(tm.scalingState.cachedQueueDepthTime)
			maxCacheAge := tm.config.ScaleCheckInterval * 2
			tm.scalingMutex.Unlock()

			if cachedDepth > 0 && cacheAge < maxCacheAge {
				tm.Warn("USING_CACHED_QUEUE_DEPTH", map[string]any{
					"cached_depth": cachedDepth,
					"cache_age":    cacheAge.String(),
				})
				queueDepth = cachedDepth
			} else {
				// No valid cached value, skip this check
				continue
			}
		} else {
			// Successfully retrieved queue depth, cache it
			tm.scalingMutex.Lock()
			tm.scalingState.cachedQueueDepth = queueDepth
			tm.scalingState.cachedQueueDepthTime = time.Now()
			tm.scalingMutex.Unlock()
		}

		tm.mutex.RLock()
		currentWorkers := tm.activeWorkers
		tm.mutex.RUnlock()

		// Export metrics
		hermes_metrics.QueueDepth.WithLabelValues(tm.config.QueueName).Set(float64(queueDepth))
		hermes_metrics.QueueWorkerCount.WithLabelValues(tm.config.QueueName).Set(float64(currentWorkers))

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
		hermes_metrics.QueueScalingDecisions.WithLabelValues(tm.config.QueueName, direction).Inc()

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

// checkBungieAPIDisabled checks if the Bungie API is disabled via the Settings API
func (tm *TopicManager) checkBungieAPIDisabled() (bool, error) {
	result, err := bungie.Client.GetCommonSettings()
	if err != nil {
		return false, err
	}
	if !result.Success || result.Data == nil {
		return false, fmt.Errorf("failed to get settings: Success=%t", result.Success)
	}
	
	// Check if D2Core is disabled
	if system, exists := result.Data.Systems["Destiny2"]; exists {
		return !system.Enabled, nil
	}
	
	// System not found, assume enabled
	return false, nil
}

// shouldScaleWorkers determines if scaling should happen based on dead zone, hysteresis, and cooldown
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

// getQueueDepthChannel gets or creates a dedicated channel for queue depth checks
// This channel is ONLY used for QueueDeclare (read-only) - it never consumes messages.
// Workers have their own separate channels for consuming.
func (tm *TopicManager) getQueueDepthChannel() (*amqp.Channel, error) {
	tm.depthChannelMutex.RLock()
	if tm.depthCheckChannel != nil {
		ch := tm.depthCheckChannel
		tm.depthChannelMutex.RUnlock()
		return ch, nil
	}
	tm.depthChannelMutex.RUnlock()

	// Need to create a new channel
	tm.depthChannelMutex.Lock()
	defer tm.depthChannelMutex.Unlock()

	// Double-check after acquiring write lock
	if tm.depthCheckChannel != nil {
		return tm.depthCheckChannel, nil
	}

	// Create new channel
	ch, err := rabbit.Conn.Channel()
	if err != nil {
		return nil, err
	}
	tm.depthCheckChannel = ch
	return ch, nil
}

// getQueueDepthWithRetry gets the current queue depth with exponential backoff retry
// Uses a dedicated reusable channel for consistency
func (tm *TopicManager) getQueueDepthWithRetry() (int, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		queueDepth, err := tm.getQueueDepth()
		if err == nil {
			// Success - reset failure tracking if we had retries
			if attempt > 0 {
				tm.logger.Info("QUEUE_DEPTH_RETRY_SUCCESS", map[string]any{
					"attempt": attempt + 1,
				})
			}
			return queueDepth, nil
		}

		// If channel is closed, invalidate it
		if err == amqp.ErrClosed {
			tm.depthChannelMutex.Lock()
			tm.depthCheckChannel = nil
			tm.depthChannelMutex.Unlock()
		}

		// If this isn't the last attempt, wait before retrying
		if attempt < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<uint(attempt)) // Exponential backoff: 100ms, 200ms, 400ms
			tm.logger.Warn("QUEUE_DEPTH_RETRY", map[string]any{
				"attempt":     attempt + 1,
				"max_retries": maxRetries,
				"delay_ms":    delay.Milliseconds(),
				logging.ERROR: err.Error(),
			})
			time.Sleep(delay)
		}
	}

	// All retries failed
	return 0, fmt.Errorf("failed to get queue depth after %d attempts", maxRetries)
}

// getQueueDepth gets the current queue depth using a reusable channel
// NOTE: QueueDeclare does NOT consume messages - it only returns queue metadata.
// This channel is never used for consuming, only for checking queue depth.
func (tm *TopicManager) getQueueDepth() (int, error) {
	ch, err := tm.getQueueDepthChannel()
	if err != nil {
		return 0, err
	}

	q, err := ch.QueueDeclare(
		tm.config.QueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		// If channel error, invalidate it
		if err == amqp.ErrClosed {
			tm.depthChannelMutex.Lock()
			tm.depthCheckChannel = nil
			tm.depthChannelMutex.Unlock()
		}
		return 0, err
	}

	return q.Messages, nil
}

// handleShutdown watches the TopicManager context and manually cancels all workers when shutdown is requested
func (tm *TopicManager) handleShutdown() {
	<-tm.Context().Done()
	cause := context.Cause(tm.Context())

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

	// Close the queue depth check channel
	tm.depthChannelMutex.Lock()
	if tm.depthCheckChannel != nil {
		tm.depthCheckChannel.Close()
		tm.depthCheckChannel = nil
	}
	tm.depthChannelMutex.Unlock()
}
