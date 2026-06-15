package main

import (
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"
	"sync"
	"sync/atomic"
	"time"
)

// CheatDetection logging constants
const (
	PROCESSING_COMPLETE          = "PROCESSING_COMPLETE"
	BLACKLIST_UPDATE_ERROR       = "BLACKLIST_UPDATE_ERROR"
	BLACKLIST_UPDATED            = "BLACKLIST_UPDATED"
	PLAYER_INSTANCES_BLACKLISTED = "PLAYER_INSTANCES_BLACKLISTED"
	CHEAT_RECHECK_STARTED        = "CHEAT_RECHECK_STARTED"
	CHEAT_RECHECK_PROGRESS       = "CHEAT_RECHECK_PROGRESS"
	CHEAT_RECHECK_COMPLETE       = "CHEAT_RECHECK_COMPLETE"
)

var logger = logging.NewLogger("cheat-detection")

const (
	numBungieWorkers        = 15
	numCheatCheckWorkers    = 25
	versionPrefix           = "beta-2.2.0"
	cheatRecheckLogInterval = 30 * time.Second
)

const level3PlusInstanceQuery = `SELECT DISTINCT instance_id 
		FROM instance_player 
		JOIN player USING (membership_id)
		WHERE cheat_level >= 3 AND last_seen > NOW() - INTERVAL '60 days'
			AND NOT player.is_whitelisted`

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

	// postgres.DB is initialized in init()
	postgres.Wait()

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

	// step 2: re-cheat check all level 3+ player instances. can remove this step later.
	var totalInstances int64
	err := postgres.DB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM (%s) AS instances`, level3PlusInstanceQuery)).Scan(&totalInstances)
	if err != nil {
		logger.Warn("CHEAT_RECHECK_COUNT_ERROR", err, map[string]any{
			logging.OPERATION: "count_level3_instances",
		})
	}

	logger.Info(CHEAT_RECHECK_STARTED, map[string]any{
		"total_instances": totalInstances,
		"workers":         numCheatCheckWorkers,
	})

	instanceIds := make(chan int64)
	var processedCount int32
	var failedCount int32
	recheckStart := time.Now()
	recheckDone := make(chan struct{})

	wg.Add(numCheatCheckWorkers)
	for i := 0; i < numCheatCheckWorkers; i++ {
		go func() {
			defer wg.Done()
			for instanceId := range instanceIds {
				_, _, _, _, err := cheat_detection.CheckForCheats(instanceId)
				if err != nil {
					atomic.AddInt32(&failedCount, 1)
					logger.Warn("CHEAT_CHECK_FAILED", err, map[string]any{
						logging.INSTANCE_ID: instanceId,
					})
				}
				atomic.AddInt32(&processedCount, 1)
			}
		}()
	}

	go func() {
		ticker := time.NewTicker(cheatRecheckLogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				logCheatRecheckProgress(
					atomic.LoadInt32(&processedCount),
					atomic.LoadInt32(&failedCount),
					totalInstances,
					recheckStart,
				)
			case <-recheckDone:
				return
			}
		}
	}()

	rows, err = postgres.DB.Query(level3PlusInstanceQuery)
	if err != nil {
		logger.Warn("BLACKLIST_QUERY_ERROR", err, map[string]any{
			logging.OPERATION: "query_blacklisted_instances",
		})
	}
	defer rows.Close()

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			logger.Warn("INSTANCE_ID_SCAN_ERROR", err, nil)
			continue
		}
		instanceIds <- instanceId
	}
	if err := rows.Err(); err != nil {
		logger.Warn("INSTANCE_ID_ROWS_ERROR", err, nil)
	}
	close(instanceIds)
	wg.Wait()
	close(recheckDone)

	logCheatRecheckProgress(
		atomic.LoadInt32(&processedCount),
		atomic.LoadInt32(&failedCount),
		totalInstances,
		recheckStart,
	)
	logger.Info(CHEAT_RECHECK_COMPLETE, map[string]any{
		"processed":       atomic.LoadInt32(&processedCount),
		"failed":          atomic.LoadInt32(&failedCount),
		"total_instances": totalInstances,
		"elapsed":         time.Since(recheckStart).String(),
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

func logCheatRecheckProgress(processed int32, failed int32, totalInstances int64, start time.Time) {
	fields := map[string]any{
		"processed": processed,
		"failed":    failed,
		"elapsed":   time.Since(start).String(),
	}
	if totalInstances > 0 {
		fields["total"] = totalInstances
		fields["percent"] = fmt.Sprintf("%.1f%%", float64(processed)/float64(totalInstances)*100)
	}
	logger.Info(CHEAT_RECHECK_PROGRESS, fields)
}
