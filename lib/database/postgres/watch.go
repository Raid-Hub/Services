package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"raidhub/lib/utils/logging"
	"time"
)

// PostgreSQL query watch constants
const (
	QUERY_WATCH_STARTED = "QUERY_WATCH_STARTED"
	QUERY_WATCH_ERROR   = "QUERY_WATCH_ERROR"
	QUERY_PROGRESS      = "QUERY_PROGRESS"
	QUERY_COMPLETED     = "QUERY_COMPLETED"
)

// progressData is generic struct for progress info we log.
type progressData struct {
	Phase           string
	BlocksDone      int64
	BlocksTotal     int64
	TuplesDone      int64
	TuplesTotal     int64
	PartitionsDone  int64
	PartitionsTotal int64
}

// monitorProgress polls the DB with the provided query and scanner function.
// It logs progress periodically until ctx is cancelled.
func monitorProgress(ctx context.Context, poll time.Duration, query string, args []any, label string) {
	go func() {
		start := time.Now()
		ticker := time.NewTicker(poll)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info(QUERY_COMPLETED, map[string]any{
					logging.OPERATION: label,
					logging.DURATION:  time.Since(start).String(),
					"status":          "stopped",
				})
				return
			case <-ticker.C:
				row := DB.QueryRowContext(ctx, query, args...)
				var progress progressData
				err := row.Scan(&progress.Phase, &progress.BlocksDone, &progress.BlocksTotal, &progress.TuplesDone, &progress.TuplesTotal, &progress.PartitionsDone, &progress.PartitionsTotal)
				if err == sql.ErrNoRows {
					// Finished or no progress info yet
					continue
				}
				if err != nil {
					logger.Warn(QUERY_WATCH_ERROR, map[string]any{
						logging.OPERATION: label,
						logging.ERROR:     err.Error(),
					})
					return
				}

				var percent float64
				switch {
				case progress.TuplesDone != progress.TuplesTotal:
					percent = float64(progress.TuplesDone) / float64(progress.TuplesTotal) * 100
					logger.Info(QUERY_PROGRESS, map[string]any{
						logging.OPERATION:        label,
						logging.PHASE:            progress.Phase,
						logging.PROGRESS_PERCENT: percent,
						"tuples_done":            progress.TuplesDone,
						"tuples_total":           progress.TuplesTotal,
						"unit":                   "tuples",
					})
				case progress.BlocksDone != progress.BlocksTotal:
					percent = float64(progress.BlocksDone) / float64(progress.BlocksTotal) * 100
					logger.Info(QUERY_PROGRESS, map[string]any{
						logging.OPERATION:        label,
						logging.PHASE:            progress.Phase,
						logging.PROGRESS_PERCENT: percent,
						"blocks_done":            progress.BlocksDone,
						"blocks_total":           progress.BlocksTotal,
						"unit":                   "blocks",
					})
				case progress.PartitionsDone != progress.PartitionsTotal:
					percent = float64(progress.PartitionsDone) / float64(progress.PartitionsTotal) * 100
					logger.Info(QUERY_PROGRESS, map[string]any{
						logging.OPERATION:        label,
						logging.PHASE:            progress.Phase,
						logging.PROGRESS_PERCENT: percent,
						logging.PARTITIONS_DONE:  progress.PartitionsDone,
						logging.PARTITIONS_TOTAL: progress.PartitionsTotal,
						logging.UNIT:             "partitions",
					})
				default:
					logger.Info(QUERY_PROGRESS, map[string]any{
						logging.OPERATION:         label,
						"phase":                   progress.Phase,
						logging.PROGRESS_MEASURED: false,
					})
				}
			}
		}
	}()
}

// MonitorIndexCreationProgress monitors CREATE INDEX progress
func MonitorIndexCreationProgress(ctx context.Context, tableName, indexName string, poll time.Duration) {
	query := `SELECT
		p.phase,
		p.blocks_done,
		p.blocks_total,
		p.tuples_done,
		p.tuples_total,
		p.partitions_done,
		p.partitions_total
	FROM pg_stat_progress_create_index p
	WHERE p.relid = $1::regclass
	LIMIT 1`

	monitorProgress(ctx, poll, query, []any{tableName}, fmt.Sprintf("CREATE INDEX '%s' ON '%s'", indexName, tableName))
}
