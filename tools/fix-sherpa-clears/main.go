package main

import (
	"context"
	"os"
	"os/signal"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"sync"
	"syscall"
	"time"
)

var logger = logging.NewLogger("fix-sherpa-clears")

// FixSherpaClears is the command function
// This script is used to rebuild the sherpa and first clear columns in the instance_player table
// due to the race condition that can occur when activity clears are processed in parallel or come
// in out of order.

func FixSherpaClears() {
	scriptStart := time.Now()
	db := postgres.DB

	ctx, cancel := context.WithCancel(context.Background())

	// Set up signal catching
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		logger.Info("SIGNAL_RECEIVED", map[string]any{"action": "cancelling_context"})
		cancel()
	}()

	wg := sync.WaitGroup{}

	// This first section here resets the sherpas and first clear columns
	logger.Info("CREATING_INDEX", map[string]any{"table": "instance_player", "index": "idx_instance_player_completed"})
	monitorCtx, endMonitor := context.WithCancel(ctx)
	defer endMonitor()
	postgres.MonitorIndexCreationProgress(monitorCtx, "instance_player", "idx_instance_player_completed", 10*time.Second)

	_, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_instance_player_completed ON instance_player (completed) INCLUDE (membership_id, instance_id) WHERE completed`)
	if err != nil {
		logger.Error("ERROR_CREATING_INDEX", err, map[string]any{"table": "instance_player", "index": "idx_instance_player_completed"})
	}
	endMonitor() // Stop monitoring after index creation

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("ERROR_BEGINNING_TRANSACTION", err, map[string]any{})
	}
	defer tx.Rollback()

	// acquire lock on player and instance_player tables
	logger.Info("ACQUIRING_LOCK", map[string]any{"tables": "player, instance_player, player_stats"})
	_, err = tx.ExecContext(ctx, `LOCK TABLE player, instance_player, player_stats IN EXCLUSIVE MODE`)
	if err != nil {
		logger.Error("ERROR_ACQUIRING_LOCK", err, map[string]any{"tables": "player, instance_player, player_stats"})
	}
	logger.Info("LOCK_ACQUIRED", map[string]any{})

	logger.Info("UPDATING_MATERIALIZED_VIEW", map[string]any{logging.VIEW: "firsts_clears_tmp"})
	start := time.Now()

	_, err = tx.ExecContext(ctx, `REFRESH MATERIALIZED VIEW firsts_clears_tmp WITH DATA`)
	if err != nil {
		logger.Error("ERROR_REFRESHING_MATERIALIZED_VIEW", err, map[string]any{logging.VIEW: "firsts_clears_tmp"})
	}
	logger.Info("MATERIALIZED_VIEW_UPDATED", map[string]any{logging.VIEW: "firsts_clears_tmp", "duration": time.Since(start).String()})

	logger.Info("UPDATING_MATERIALIZED_VIEW", map[string]any{logging.VIEW: "noob_counts"})
	start = time.Now()

	_, err = tx.ExecContext(ctx, `REFRESH MATERIALIZED VIEW noob_counts WITH DATA`)
	if err != nil {
		logger.Error("ERROR_REFRESHING_MATERIALIZED_VIEW", err, map[string]any{logging.VIEW: "noob_counts"})
	}
	logger.Info("MATERIALIZED_VIEW_UPDATED", map[string]any{logging.VIEW: "noob_counts", "duration": time.Since(start).String()})

	logger.Info("UPDATING_SHERPAS_AND_FIRST_CLEAR", map[string]any{})
	start = time.Now()
	_, err = tx.ExecContext(ctx, `UPDATE instance_player _ap
		SET is_first_clear = f.instance_id IS NOT NULL,
		sherpas =
			CASE WHEN f.instance_id IS NULL
				THEN COALESCE(s.newb_count, 0)
				ELSE 0
			END
		FROM instance_player ap
		LEFT JOIN firsts_clears_tmp f ON ap.instance_id = f.instance_id
			AND ap.membership_id = f.membership_id
		LEFT JOIN noob_counts s ON ap.instance_id = s.instance_id
		WHERE ap.completed
			AND ap.membership_id = _ap.membership_id
			AND ap.instance_id = _ap.instance_id`)
	if err != nil {
		logger.Error("ERROR_UPDATING_SHERPAS_AND_FIRST_CLEAR", err, map[string]any{})
	}
	logger.Info("SHERPAS_AND_FIRST_CLEAR_UPDATED", map[string]any{"duration": time.Since(start).String()})

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("DROPPING_INDEX", map[string]any{"table": "instance_player", "index": "idx_instance_player_completed"})
		start := time.Now()
		_, err = tx.ExecContext(ctx, `DROP INDEX IF EXISTS idx_instance_player_completed`)
		if err != nil {
			logger.Error("ERROR_DROPPING_INDEX", err, map[string]any{"table": "instance_player", "index": "idx_instance_player_completed"})
		}
		logger.Info("INDEX_DROPPED", map[string]any{"table": "instance_player", "index": "idx_instance_player_completed", "duration": time.Since(start).String()})
	}()

	// part 1 end, can restart from part 2 if needed

	// Once the first section is done, we can update the materialized view which seeds the player_stats and global_stats tables
	logger.Info("UPDATING_MATERIALIZED_VIEW", map[string]any{logging.VIEW: "p_stats_cache"})
	start = time.Now()

	_, err = tx.ExecContext(ctx, `REFRESH MATERIALIZED VIEW p_stats_cache WITH DATA`)
	if err != nil {
		logger.Error("ERROR_REFRESHING_MATERIALIZED_VIEW", err, map[string]any{logging.VIEW: "p_stats_cache"})
	}
	logger.Info("MATERIALIZED_VIEW_UPDATED", map[string]any{logging.VIEW: "p_stats_cache", "duration": time.Since(start).String()})

	// This update the player_stats and global_stats tables
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("UPDATING_PLAYER_STATS", map[string]any{})
		start := time.Now()
		_, err := tx.ExecContext(ctx, `UPDATE player_stats _ps
            SET 
                clears = COALESCE(p.clears, 0),
                fresh_clears = COALESCE(p.fresh_clears, 0),
                sherpas =  COALESCE(p.sherpa_count, 0),
                total_time_played_seconds =  COALESCE(p.total_time_played, 0),
				fastest_instance_id = p.fastest_instance_id
			FROM player_stats ps
            LEFT JOIN p_stats_cache p USING (membership_id, activity_id)
            WHERE ps.membership_id = _ps.membership_id AND ps.activity_id = _ps.activity_id`)
		if err != nil {
			logger.Error("ERROR_UPDATING_PLAYER_STATS", err, map[string]any{})
		}
		logger.Info("PLAYER_STATS_UPDATED", map[string]any{"duration": time.Since(start).String()})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("UPDATING_GLOBAL_STATS", map[string]any{})
		start := time.Now()
		_, err := tx.ExecContext(ctx, `
            WITH active_raid_count AS (
                SELECT COUNT(*) as expected 
                FROM activity_definition 
                WHERE is_raid = true AND is_sunset = false
            ),
            g_stats AS (
                SELECT 
                    membership_id, 
                    SUM(clears) as clears,
                    SUM(fresh_clears) as fresh_clears,
                    SUM(sherpa_count) as sherpas,
                    SUM(total_time_played) AS total_time_played_seconds,
                    SUM(fast.duration) AS speed_total_duration,
                    COUNT(fast.instance_id) = (SELECT expected FROM active_raid_count) as is_duration_valid
                FROM p_stats_cache
                JOIN activity_definition ad ON p_stats_cache.activity_id = ad.id
                LEFT JOIN instance fast ON p_stats_cache.fastest_instance_id = fast.instance_id AND is_raid AND NOT is_sunset
                GROUP BY membership_id
            )
            UPDATE player _p SET 
                clears = COALESCE(g.clears, 0),
                fresh_clears = COALESCE(g.fresh_clears, 0),
                sherpas = COALESCE(g.sherpas, 0),
                total_time_played_seconds = COALESCE(g.total_time_played_seconds, 0),
                sum_of_best = CASE WHEN is_duration_valid THEN g.speed_total_duration ELSE NULL END
            FROM player p 
			LEFT JOIN g_stats g USING (membership_id)
            WHERE p.membership_id = _p.membership_id`)
		if err != nil {
			logger.Error("ERROR_UPDATING_GLOBAL_STATS", err, map[string]any{})
		}
		logger.Info("GLOBAL_STATS_UPDATED", map[string]any{"duration": time.Since(start).String()})
	}()

	wg.Wait()
	if err := tx.Commit(); err != nil {
		logger.Error("ERROR_COMMITTING_TRANSACTION", err, map[string]any{})
	}
	logger.Info("TRANSACTION_COMMITTED", map[string]any{})

	logger.Info("COMPLETED", map[string]any{"duration": time.Since(scriptStart).String()})
}

func main() {
	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	FixSherpaClears()
}
