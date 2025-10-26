package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
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
				log.Printf("%s monitoring stopped after %s", label, time.Since(start))
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
					log.Printf("%s monitoring error: %v", label, err)
					return
				}

				var percent float64
				switch {
				case progress.TuplesDone != progress.TuplesTotal:
					percent = float64(progress.TuplesDone) / float64(progress.TuplesTotal) * 100
					log.Printf("%s: phase=%s, progress=%.1f%% (%d/%d tuples processed)",
						label, progress.Phase, percent, progress.TuplesDone, progress.TuplesTotal)
				case progress.BlocksDone != progress.BlocksTotal:
					percent = float64(progress.BlocksDone) / float64(progress.BlocksTotal) * 100
					log.Printf("%s: phase=%s, progress=%.1f%% (%d/%d blocks scanned)",
						label, progress.Phase, percent, progress.BlocksDone, progress.BlocksTotal)
				case progress.PartitionsDone != progress.PartitionsTotal:
					percent = float64(progress.PartitionsDone) / float64(progress.PartitionsTotal) * 100
					log.Printf("%s: phase=%s, progress=%.1f%% (%d/%d partitions processed)",
						label, progress.Phase, percent, progress.PartitionsDone, progress.PartitionsTotal)
				default:
					log.Printf("%s: phase=%s (progress not measured)", label, progress.Phase)
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
