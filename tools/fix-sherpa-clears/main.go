package main

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"os/signal"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"sync"
	"syscall"
	"time"
)

var logger = logging.NewLogger("fix-sherpa-clears")

// FixSherpaClears rebuilds sherpa/first-clear columns on instance_player and optionally
// reconciles player_stats / player from cache.p_stats_cache.
//
// Creates cache.firsts_clears_tmp, cache.noob_counts, and cache.p_stats_cache if missing.

func FixSherpaClears(ctx context.Context, statsOnly bool) error {
	scriptStart := time.Now()
	db := postgres.DB

	if err := ensureRepairViews(ctx, db); err != nil {
		return err
	}
	if !statsOnly {
		logger.Info("CREATING_INDEX", map[string]any{"table": "instance_player", "index": "idx_instance_player_completed"})
		monitorCtx, endMonitor := context.WithCancel(ctx)
		postgres.MonitorIndexCreationProgress(monitorCtx, "instance_player", "idx_instance_player_completed", 10*time.Second)

		if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_instance_player_completed ON core.instance_player (completed) INCLUDE (membership_id, instance_id) WHERE completed`); err != nil {
			endMonitor()
			return err
		}
		endMonitor()
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	logger.Info("ACQUIRING_LOCK", map[string]any{"tables": "player, instance_player, player_stats"})
	if _, err := tx.ExecContext(ctx, `LOCK TABLE core.player, core.instance_player, core.player_stats IN EXCLUSIVE MODE`); err != nil {
		return err
	}

	wg := sync.WaitGroup{}

	if !statsOnly {
		if err := refreshView(ctx, tx, "cache.firsts_clears_tmp"); err != nil {
			return err
		}
		if err := refreshView(ctx, tx, "cache.noob_counts"); err != nil {
			return err
		}

		start := time.Now()
		if _, err := tx.ExecContext(ctx, `UPDATE core.instance_player _ap
			SET is_first_clear = f.instance_id IS NOT NULL,
			sherpas =
				CASE WHEN f.instance_id IS NULL
					THEN COALESCE(s.newb_count, 0)
					ELSE 0
				END
			FROM core.instance_player ap
			LEFT JOIN cache.firsts_clears_tmp f ON ap.instance_id = f.instance_id
				AND ap.membership_id = f.membership_id
			LEFT JOIN cache.noob_counts s ON ap.instance_id = s.instance_id
			WHERE ap.completed
				AND ap.membership_id = _ap.membership_id
				AND ap.instance_id = _ap.instance_id`); err != nil {
			return err
		}
		logger.Info("SHERPAS_AND_FIRST_CLEAR_UPDATED", map[string]any{"duration": time.Since(start).String()})

		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			if _, err := tx.ExecContext(ctx, `DROP INDEX IF EXISTS core.idx_instance_player_completed`); err != nil {
				logger.Error("ERROR_DROPPING_INDEX", err, map[string]any{"table": "instance_player", "index": "idx_instance_player_completed"})
			} else {
				logger.Info("INDEX_DROPPED", map[string]any{"duration": time.Since(start).String()})
			}
		}()
	}

	if err := refreshView(ctx, tx, "cache.p_stats_cache"); err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		_, err := tx.ExecContext(ctx, `UPDATE core.player_stats _ps
            SET
                clears = p.clears,
                fresh_clears = p.fresh_clears,
                sherpas = p.sherpa_count,
                total_time_played_seconds = p.total_time_played,
                fastest_instance_id = p.fastest_instance_id
            FROM cache.p_stats_cache p
            WHERE _ps.membership_id = p.membership_id
              AND _ps.activity_id = p.activity_id`)
		if err != nil {
			logger.Error("ERROR_UPDATING_PLAYER_STATS", err, map[string]any{})
			return
		}
		logger.Info("PLAYER_STATS_UPDATED", map[string]any{"duration": time.Since(start).String()})
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		_, err := tx.ExecContext(ctx, `
            WITH active_raid_count AS (
                SELECT COUNT(*)::int AS expected
                FROM definitions.activity_definition
                WHERE is_raid = true AND is_sunset = false
            ),
            g_stats AS (
                SELECT
                    psc.membership_id,
                    SUM(psc.clears)::int AS clears,
                    SUM(psc.fresh_clears)::int AS fresh_clears,
                    SUM(psc.sherpa_count)::int AS sherpas,
                    SUM(psc.total_time_played)::int AS total_time_played_seconds,
                    SUM(fast.duration)::int AS speed_total_duration,
                    COUNT(fast.instance_id) = (SELECT expected FROM active_raid_count) AS is_duration_valid
                FROM cache.p_stats_cache psc
                JOIN definitions.activity_definition ad ON psc.activity_id = ad.id
                LEFT JOIN core.instance fast
                    ON psc.fastest_instance_id = fast.instance_id
                   AND ad.is_raid
                   AND NOT ad.is_sunset
                GROUP BY psc.membership_id
            )
            UPDATE core.player _p SET
                clears = COALESCE(g.clears, 0),
                fresh_clears = COALESCE(g.fresh_clears, 0),
                sherpas = COALESCE(g.sherpas, 0),
                total_time_played_seconds = COALESCE(g.total_time_played_seconds, 0),
                sum_of_best = CASE WHEN g.is_duration_valid THEN g.speed_total_duration ELSE NULL END
            FROM core.player p
            LEFT JOIN g_stats g USING (membership_id)
            WHERE p.membership_id = _p.membership_id`)
		if err != nil {
			logger.Error("ERROR_UPDATING_GLOBAL_STATS", err, map[string]any{})
			return
		}
		logger.Info("GLOBAL_STATS_UPDATED", map[string]any{"duration": time.Since(start).String()})
	}()

	wg.Wait()

	if err := tx.Commit(); err != nil {
		return err
	}

	logger.Info("COMPLETED", map[string]any{"duration": time.Since(scriptStart).String(), "stats_only": statsOnly})
	return nil
}

func refreshView(ctx context.Context, tx *sql.Tx, view string) error {
	logger.Info("REFRESHING_VIEW", map[string]any{logging.VIEW: view})
	start := time.Now()
	if _, err := tx.ExecContext(ctx, `REFRESH MATERIALIZED VIEW `+view+` WITH DATA`); err != nil {
		return err
	}
	logger.Info("VIEW_REFRESHED", map[string]any{logging.VIEW: view, "duration": time.Since(start).String()})
	return nil
}

func ensureRepairViews(ctx context.Context, db *sql.DB) error {
	type repairView struct {
		name     string
		ddl      string
		indexDDL []string
	}

	views := []repairView{
		{
			name: "firsts_clears_tmp",
			ddl: `CREATE MATERIALIZED VIEW cache.firsts_clears_tmp AS
SELECT DISTINCT ON (ip.membership_id, av.activity_id)
    ip.instance_id,
    ip.membership_id,
    av.activity_id
FROM core.instance_player ip
JOIN core.instance i USING (instance_id)
JOIN definitions.activity_version av ON av.hash = i.hash
WHERE ip.completed
ORDER BY ip.membership_id, av.activity_id, i.date_completed ASC, i.instance_id ASC`,
			indexDDL: []string{
				`CREATE UNIQUE INDEX idx_firsts_clears_tmp_membership_activity
    ON cache.firsts_clears_tmp (membership_id, activity_id)`,
			},
		},
		{
			name: "noob_counts",
			ddl: `CREATE MATERIALIZED VIEW cache.noob_counts AS
SELECT
    instance_id,
    COUNT(*)::int AS newb_count
FROM cache.firsts_clears_tmp
GROUP BY instance_id`,
			indexDDL: []string{
				`CREATE UNIQUE INDEX idx_noob_counts_instance_id ON cache.noob_counts (instance_id)`,
			},
		},
		{
			name: "p_stats_cache",
			ddl: `CREATE MATERIALIZED VIEW cache.p_stats_cache AS
WITH ordered_instances AS (
    SELECT
        ip.membership_id,
        av.activity_id,
        i.instance_id,
        ROW_NUMBER() OVER (
            PARTITION BY ip.membership_id, av.activity_id
            ORDER BY i.duration ASC, i.date_completed ASC, i.instance_id ASC
        ) AS rank
    FROM core.instance_player ip
    JOIN core.instance i USING (instance_id)
    JOIN definitions.activity_version av ON av.hash = i.hash
    WHERE ip.completed
      AND i.completed
      AND i.fresh IS TRUE
),
agg_stats AS (
    SELECT
        ip.membership_id,
        av.activity_id,
        COUNT(*) FILTER (WHERE ip.completed AND i.completed)::int AS clears,
        COUNT(*) FILTER (WHERE ip.completed AND i.completed AND i.fresh IS TRUE)::int AS fresh_clears,
        COALESCE(SUM(ip.sherpas) FILTER (WHERE ip.completed AND i.completed), 0)::int AS sherpa_count,
        COALESCE(SUM(ip.time_played_seconds), 0)::int AS total_time_played
    FROM core.instance_player ip
    JOIN core.instance i USING (instance_id)
    JOIN definitions.activity_version av ON av.hash = i.hash
    GROUP BY ip.membership_id, av.activity_id
)
SELECT
    agg_stats.membership_id,
    agg_stats.activity_id,
    agg_stats.clears,
    agg_stats.fresh_clears,
    agg_stats.sherpa_count,
    agg_stats.total_time_played,
    oi.instance_id AS fastest_instance_id
FROM agg_stats
LEFT JOIN ordered_instances oi
    ON oi.membership_id = agg_stats.membership_id
   AND oi.activity_id = agg_stats.activity_id
   AND oi.rank = 1`,
			indexDDL: []string{
				`CREATE UNIQUE INDEX idx_p_stats_cache_membership_activity
    ON cache.p_stats_cache (membership_id, activity_id)`,
			},
		},
	}

	for _, view := range views {
		var exists bool
		if err := db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM pg_matviews
				WHERE schemaname = 'cache' AND matviewname = $1
			)`, view.name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		logger.Info("CREATING_VIEW", map[string]any{logging.VIEW: "cache." + view.name})
		start := time.Now()
		if _, err := db.ExecContext(ctx, view.ddl); err != nil {
			return err
		}
		for _, indexDDL := range view.indexDDL {
			if _, err := db.ExecContext(ctx, indexDDL); err != nil {
				return err
			}
		}
		logger.Info("VIEW_CREATED", map[string]any{logging.VIEW: "cache." + view.name, "duration": time.Since(start).String()})
	}

	return nil
}

func main() {
	statsOnly := flag.Bool("stats-only", false, "Skip sherpa/first-clear repair; refresh p_stats_cache and reconcile player_stats/player only")
	logging.ParseFlags()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	postgres.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		logger.Info("SIGNAL_RECEIVED", map[string]any{"action": "cancelling_context"})
		cancel()
	}()

	if err := FixSherpaClears(ctx, *statsOnly); err != nil {
		logger.Fatal("FIX_SHERPA_CLEARS_FAILED", err, map[string]any{})
	}
}
