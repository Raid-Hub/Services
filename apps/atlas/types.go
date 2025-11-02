package main

import (
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
		logger:         AtlasLogger,
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

func (w *AtlasWorker) Warn(key string, err error, fields map[string]any) {
	w.logger.Warn(key, err, w.addWorkerFields(fields))
}

func (w *AtlasWorker) Error(key string, err error, fields map[string]any) {
	w.logger.Error(key, err, w.addWorkerFields(fields))
}

func (w *AtlasWorker) Fatal(key string, err error, fields map[string]any) {
	w.logger.Fatal(key, err, w.addWorkerFields(fields))
}
