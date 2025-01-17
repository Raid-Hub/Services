package main

import (
	"context"
	"log"
	"raidhub/packages/postgres"
	"sync"
	"time"
)

func main() {
	wg := sync.WaitGroup{}

	scriptStart := time.Now()
	db, err := postgres.Connect()
	if err != nil {
		log.Fatalf("Error connecting to the database: %s", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// This first section here resets the sherpas and first clear columns
	log.Println("Creating index on instance_player.completed...")
	start := time.Now()
	_, err = db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_instance_player_completed ON instance_player (completed) INCLUDE (membership_id, instance_id)`)
	if err != nil {
		log.Fatalf("Error creating index on instance_player.completed: %s", err)
	}
	log.Printf("Index on instance_player.completed created in %s", time.Since(start))

	log.Println("Updating materialized view firsts_clears_tmp...")
	start = time.Now()
	_, err = db.ExecContext(ctx, `REFRESH MATERIALIZED VIEW firsts_clears_tmp WITH DATA`)
	if err != nil {
		log.Fatalf("Error refreshing materialized view firsts_clears_tmp: %s", err)
	}
	log.Printf("firsts_clears_tmp updated in %s", time.Since(start))

	log.Println("Updating materialized view noob_counts")
	start = time.Now()
	_, err = db.ExecContext(ctx, `REFRESH MATERIALIZED VIEW noob_counts WITH DATA`)
	if err != nil {
		log.Fatalf("Error refreshing materialized view noob_counts: %s", err)
	}
	log.Printf("noob_counts updated in %s", time.Since(start))

	log.Println("Resetting sherpa and first clear columns...")
	start = time.Now()
	_, err = db.ExecContext(ctx, `ALTER TABLE instance_player
					DROP COLUMN is_first_clear,
					DROP COLUMN sherpas,
					ADD COLUMN is_first_clear BOOLEAN DEFAULT false,
					ADD COLUMN sherpas INT DEFAULT 0
	`)
	if err != nil {
		log.Fatalf("Error resetting columns: %s", err)
	}
	log.Printf("Reset sherpa and first clear columns in %s", time.Since(start))

	log.Println("Setting sherpas and first clear...")
	start = time.Now()
	_, err = db.ExecContext(ctx, `UPDATE instance_player _ap
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
		log.Fatalf("Error updating sherpas and first clear: %s", err)
	}
	log.Printf("Sherpas and first clears set in %s", time.Since(start))

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Dropping index on instance_player.completed...")
		start := time.Now()
		_, err = db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_instance_player_completed`)
		if err != nil {
			log.Fatalf("Error dropping index on instance_player.completed: %s", err)
		}
		log.Printf("Index on instance_player.completed dropped in %s", time.Since(start))
	}()

	// Once the first section is done, we can update the materialized view which seeds the player_stats and global_stats tables
	log.Println("Updating materialized view p_stats_cache...")
	start = time.Now()
	_, err = db.ExecContext(ctx, `REFRESH MATERIALIZED VIEW p_stats_cache WITH DATA`)
	if err != nil {
		log.Fatalf("Error refreshing materialized view p_stats_cache: %s", err)
	}
	log.Printf("Materialized View p_stats_cache updated in %s", time.Since(start))

	// This update the player_stats and global_stats tables
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Updating player_stats...")
		start := time.Now()
		_, err := db.ExecContext(ctx, `UPDATE player_stats _ps
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
			log.Fatalf("Error updating player_stats: %s", err)
		}
		log.Printf("player_stats updated in %s", time.Since(start))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("Updating global_stats...")
		start := time.Now()
		_, err := db.ExecContext(ctx, `
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
			log.Fatalf("Error updating global_stats: %s", err)
		}
		log.Printf("global_stats updated in %s", time.Since(start))
	}()

	wg.Wait()

	log.Printf("Done in %s", time.Since(scriptStart))
}
