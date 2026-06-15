package main

import (
	"context"
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"
	"sync"
	"time"
)

// CheatDetection logging constants
const (
	PROCESSING_COMPLETE            = "PROCESSING_COMPLETE"
	BLACKLIST_UPDATE_ERROR         = "BLACKLIST_UPDATE_ERROR"
	BLACKLIST_UPDATED              = "BLACKLIST_UPDATED"
	PLAYER_INSTANCES_BLACKLISTED   = "PLAYER_INSTANCES_BLACKLISTED"
	CHEAT_RECHECK_ENQUEUE_STARTED  = "CHEAT_RECHECK_ENQUEUE_STARTED"
	CHEAT_RECHECK_ENQUEUE_COMPLETE = "CHEAT_RECHECK_ENQUEUE_COMPLETE"
)

var logger = logging.NewLogger("cheat-detection")

const (
	numBungieWorkers = 15
	versionPrefix    = "beta-2.2.0"
)

// Instances played by level 3+ accounts that have not yet been checked at the current version.
// Scoped to instances completed in the last 60 days so clean passes (which write no flag row) are not re-queued every run.
const level3PlusUncheckInstanceQuery = `
	SELECT DISTINCT ip.instance_id
	FROM instance_player ip
	JOIN player p USING (membership_id)
	JOIN instance i ON i.instance_id = ip.instance_id
	WHERE p.cheat_level >= 3
		AND p.last_seen > NOW() - INTERVAL '60 days'
		AND i.date_completed > NOW() - INTERVAL '60 days'
		AND NOT p.is_whitelisted
		AND NOT i.is_whitelisted
		AND NOT EXISTS (
			SELECT 1
			FROM flagging.flag_instance fi
			WHERE fi.instance_id = ip.instance_id
				AND fi.cheat_check_version = $1
		)`

type LevelsDTO struct {
	Flag                      cheat_detection.PlayerInstanceFlagStats
	CheaterAccountProbability float64
	CheaterAccountFlags       uint64
}

func main() {
	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	logger.Info("JOB_STARTED", map[string]any{
		logging.SERVICE: "cheat-detection",
		logging.VERSION: versionPrefix,
	})

	postgres.Wait()
	publishing.Wait()

	// step 1: get all player instance flags and check their cheat levels
	flags := make(chan cheat_detection.PlayerInstanceFlagStats)

	wg := sync.WaitGroup{}
	wg.Add(numBungieWorkers)
	mu := sync.Mutex{}
	stats := make([][]LevelsDTO, 5)
	for i := 0; i < numBungieWorkers; i++ {
		go func() {
			defer wg.Done()
			for flag := range flags {
				lvl, cheaterAccountProbability, cheaterFlags := cheat_detection.UpdatePlayerCheatLevel(flag)

				if lvl >= 0 {
					mu.Lock()
					stats[lvl] = append(stats[lvl], LevelsDTO{
						Flag:                      flag,
						CheaterAccountProbability: cheaterAccountProbability,
						CheaterAccountFlags:       cheaterFlags,
					})
					mu.Unlock()
				}
			}
		}()
	}

	rows := cheat_detection.GetAllInstanceFlagsByPlayer(flags, fmt.Sprintf("%s%%", versionPrefix))
	defer rows.Close()
	for rows.Next() {
		var flag cheat_detection.PlayerInstanceFlagStats
		if err := rows.Scan(
			&flag.MembershipId,
			&flag.FlaggedCount,
			&flag.FlagsA,
			&flag.FlagsB,
			&flag.FlagsC,
			&flag.FlagsD,
		); err != nil {
			logger.Warn("ROW_SCAN_ERROR", err, nil)
		}
		flags <- flag
	}
	close(flags)
	wg.Wait()

	// Calculate and log cheat level summary
	var levelCounts []int
	var totalPlayers int
	for _, levelStats := range stats {
		playerCount := len(levelStats)
		levelCounts = append(levelCounts, playerCount)
		totalPlayers += playerCount
	}

	logger.Info("CHEAT_LEVEL_SUMMARY", map[string]any{
		"total_players": totalPlayers,
		"level_0":       levelCounts[0],
		"level_1":       levelCounts[1],
		"level_2":       levelCounts[2],
		"level_3":       levelCounts[3],
		"level_4":       levelCounts[4],
	})

	// step 2: enqueue level 3+ instance rechecks for Hermes (instance_cheat_check queue).
	// Step 3 below uses flags already in DB; newly queued checks are picked up on the next run.
	ctx := context.Background()
	enqueueStart := time.Now()
	var totalInstances int64
	err := postgres.DB.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*) FROM (%s) AS instances`, level3PlusUncheckInstanceQuery),
		cheat_detection.CheatCheckVersion,
	).Scan(&totalInstances)
	if err != nil {
		logger.Warn("CHEAT_RECHECK_COUNT_ERROR", err, map[string]any{
			logging.OPERATION: "count_level3_instances",
		})
	}

	logger.Info(CHEAT_RECHECK_ENQUEUE_STARTED, map[string]any{
		"total_instances": totalInstances,
		"cheat_version":   cheat_detection.CheatCheckVersion,
		"queue":           routing.InstanceCheatCheck,
	})

	var publishedCount int
	var publishFailedCount int
	if totalInstances > 0 {
		rows, err := postgres.DB.QueryContext(ctx, level3PlusUncheckInstanceQuery, cheat_detection.CheatCheckVersion)
		if err != nil {
			logger.Warn("CHEAT_RECHECK_QUERY_ERROR", err, map[string]any{
				logging.OPERATION: "query_level3_instances",
			})
		} else {
			defer rows.Close()
			for rows.Next() {
				var instanceId int64
				if err := rows.Scan(&instanceId); err != nil {
					logger.Warn("INSTANCE_ID_SCAN_ERROR", err, nil)
					continue
				}
				if err := publishing.PublishInt64Message(ctx, routing.InstanceCheatCheck, instanceId); err != nil {
					publishFailedCount++
					logger.Warn("CHEAT_RECHECK_PUBLISH_FAILED", err, map[string]any{
						logging.INSTANCE_ID: instanceId,
					})
					continue
				}
				publishedCount++
			}
			if err := rows.Err(); err != nil {
				logger.Warn("INSTANCE_ID_ROWS_ERROR", err, nil)
			}
		}
	}

	logger.Info(CHEAT_RECHECK_ENQUEUE_COMPLETE, map[string]any{
		"published":       publishedCount,
		"failed":          publishFailedCount,
		"total_instances": totalInstances,
		"elapsed":         time.Since(enqueueStart).String(),
	})

	// step 3: upgrade high flagged instances to blacklisted
	countBlacklisted, err := cheat_detection.BlacklistFlaggedInstances()
	if err != nil {
		logger.Warn(BLACKLIST_UPDATE_ERROR, err, map[string]any{
			logging.OPERATION: "blacklist_flagged_instances",
		})
	}

	// step 4: select players with cheat level 4 and mark their instances as blacklisted
	now := time.Now()
	players, err := cheat_detection.GetRecentlyPlayedBlacklistedPlayers(now.Add(-14 * 24 * time.Hour))
	if err != nil {
		logger.Warn("PLAYER_QUERY_ERROR", err, map[string]any{
			logging.OPERATION: "get_recently_played_blacklisted",
		})
	}

	var totalBlacklistedCount int64
	var totalElligibleCount int64
	for _, player := range players {
		count, elligible, err := cheat_detection.BlacklistRecentInstances(player)
		if err != nil {
			logger.Warn(BLACKLIST_UPDATE_ERROR, err, map[string]any{
				logging.MEMBERSHIP_ID: player.MembershipId,
				logging.OPERATION:     "blacklist_player_instances",
			})
		}
		totalBlacklistedCount += count
		totalElligibleCount += elligible
	}

	logger.Info("BLACKLIST_SUMMARY", map[string]any{
		"flagged_instances":  countBlacklisted,
		"recent_blacklisted": totalBlacklistedCount,
		"total_eligible":     totalElligibleCount,
		"players_processed":  len(players),
	})

	logger.Info(PROCESSING_COMPLETE, map[string]any{
		logging.SERVICE: "cheat-detection",
		logging.STATUS:  "complete",
	})
}
