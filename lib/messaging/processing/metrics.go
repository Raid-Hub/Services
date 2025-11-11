package processing

// QueueMetrics provides an interface for recording queue-related metrics.
// This allows the processing package to be decoupled from app-specific metrics implementations.
type QueueMetrics interface {
	SetWorkerCount(queueName string, count float64)
	SetQueueDepth(queueName string, depth float64)
	IncScalingDecision(queueName string, direction string)
	IncMessageProcessed(queueName string, status string)
	ObserveMessageProcessingDuration(queueName string, durationSeconds float64)
}

// NoOpQueueMetrics is a no-op implementation that does nothing.
// Useful when metrics are not needed or when metrics are handled elsewhere.
type NoOpQueueMetrics struct{}

func (m *NoOpQueueMetrics) SetWorkerCount(queueName string, count float64)        {}
func (m *NoOpQueueMetrics) SetQueueDepth(queueName string, depth float64)         {}
func (m *NoOpQueueMetrics) IncScalingDecision(queueName string, direction string) {}
func (m *NoOpQueueMetrics) IncMessageProcessed(queueName string, status string)   {}
func (m *NoOpQueueMetrics) ObserveMessageProcessingDuration(queueName string, durationSeconds float64) {
}
