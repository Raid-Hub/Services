package main

import (
	"fmt"
	"strings"

	"raidhub/lib/utils/logging"
)

type ConsumerConfig struct {
	LatestId       int64
	OffloadChannel chan int64
	Skip           int // Number of instances to skip between each processed instance (dev mode)
}

type WorkerResult struct {
	Lag       []float64
	NotFounds int
}

type AtlasConfig struct {
	Workers          int
	Buffer           int64
	TargetInstanceId int64
	DevMode          bool
	DevSkip          int
	MaxWorkers       int
}

// AtlasWorker represents a worker goroutine that processes PGCR instances
type AtlasWorker struct {
	ID             int
	logger         logging.Logger
	offloadChannel chan int64
}

func NewAtlasWorker(workerID int, offloadChannel chan int64) *AtlasWorker {
	return &AtlasWorker{
		ID:             workerID,
		logger:         logging.NewLogger(fmt.Sprintf("Atlas::Worker#%d", workerID)),
		offloadChannel: offloadChannel,
	}
}

func (w *AtlasWorker) addWorkerFields(fields map[string]any) map[string]any {
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["worker_id"] = w.ID
	return fields
}

func (w *AtlasWorker) Debug(key string, fields map[string]any) {
	w.logger.Debug(key, w.addWorkerFields(fields))
}

func (w *AtlasWorker) Info(key string, fields map[string]any) {
	w.logger.Info(key, w.addWorkerFields(fields))
}

func (w *AtlasWorker) Warn(key string, fields map[string]any) {
	w.logger.Warn(key, w.addWorkerFields(fields))
}

func (w *AtlasWorker) Error(key string, fields map[string]any) {
	w.logger.Error(key, w.addWorkerFields(fields))
}

func (w *AtlasWorker) Fatal(key string, fields map[string]any) {
	w.logger.Fatal(key, w.addWorkerFields(fields))
}

// LogPGCRError logs PGCR fetch errors, handling timeout and connection errors more gracefully
func (w *AtlasWorker) LogPGCRError(err error, instanceID int64, attempt int) {
	errStr := err.Error()
	isTimeout := strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")
	isConnectionError := strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset")

	fields := map[string]any{
		logging.INSTANCE_ID: instanceID,
		logging.ERROR:       errStr,
		logging.ATTEMPT:     attempt,
	}

	if isTimeout {
		w.Warn("PGCR_FETCH_TIMEOUT", fields)
		return
	}

	if isConnectionError {
		w.Warn("PGCR_CONNECTION_ERROR", fields)
		return
	}

	w.Error("PGCR_REQUEST_ERROR", fields)
}
