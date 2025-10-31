package cheatdetection

import (
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"
	"sync"
	"time"
)

// CheatDetection logging constants
const (
	SERVICE_STARTED              = "SERVICE_STARTED"
	PROCESSING_COMPLETE          = "PROCESSING_COMPLETE"
	BLACKLIST_UPDATE_ERROR       = "BLACKLIST_UPDATE_ERROR"
	BLACKLIST_UPDATED            = "BLACKLIST_UPDATED"
	PLAYER_INSTANCES_BLACKLISTED = "PLAYER_INSTANCES_BLACKLISTED"
)

var logger = logging.NewLogger("CHEAT_DETECTION")

const (
	numBungieWorkers     = 15
	numCheatCheckWorkers = 25
	versionPrefix        = "beta-2.1"
)

type LevelsDTO struct {
	Flag                      cheat_detection.PlayerInstanceFlagStats
	CheaterAccountProbability float64
	CheaterAccountFlags       uint64
}

// CheatDetection is the command function for cheat detection and player account maintenance
// Usage: ./bin/tools cheat-detection
func CheatDetection() {
	logger.Info(SERVICE_STARTED, map[string]any{
		logging.SERVICE: "cheat-detection",
		"workers":       []string{"cheat_detection", "account_maintenance"},
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
			logger.Warn("ROW_SCAN_ERROR", map[string]any{
				logging.ERROR: err.Error(),
			})
		}
		flags <- flag
	}
	close(flags)
	wg.Wait()

	// calculate average of each flag type, total flags, and flag count
	logger.Info("CHEAT_LEVEL_STATS", nil)
	for i, levelStats := range stats {
		logger.Info("CHEAT_LEVEL_COUNT", map[string]any{
			"level":        i,
			"player_count": len(levelStats),
		})
		if len(levelStats) == 0 {
			continue
		}
		var totalFlagsA, totalFlagsB, totalFlagsC, totalFlagsD, totalFlags int
		var totalCheaterProbability float64
		for _, stat := range levelStats {
			totalFlagsA += stat.Flag.FlagsA
			totalFlagsB += stat.Flag.FlagsB
			totalFlagsC += stat.Flag.FlagsC
			totalFlagsD += stat.Flag.FlagsD
			totalFlags += stat.Flag.FlaggedCount
			totalCheaterProbability += stat.CheaterAccountProbability
		}
		avgFlagsA := float64(totalFlagsA) / float64(len(levelStats))
		avgFlagsB := float64(totalFlagsB) / float64(len(levelStats))
		avgFlagsC := float64(totalFlagsC) / float64(len(levelStats))
		avgFlagsD := float64(totalFlagsD) / float64(len(levelStats))
		avgTotalFlags := float64(totalFlags) / float64(len(levelStats))
		avgCheaterProbability := totalCheaterProbability / float64(len(levelStats))

		logger.Info("AVERAGE_FLAGS", map[string]any{
			"flags_a":           avgFlagsA,
			"flags_b":           avgFlagsB,
			"flags_c":           avgFlagsC,
			"flags_d":           avgFlagsD,
			"total_flags":       avgTotalFlags,
			"cheat_probability": avgCheaterProbability,
		})
	}

	logger.Info(PROCESSING_COMPLETE, map[string]any{
		logging.OPERATION: "all_flags",
	})

	// step 2: re-cheat check all level 3+ player instances. can remove this step later.
	instanceIds := make(chan int64)
	wg.Add(numCheatCheckWorkers)
	for i := 0; i < numCheatCheckWorkers; i++ {
		go func() {
			defer wg.Done()
			for instanceId := range instanceIds {
				_, _, _, _, err := cheat_detection.CheckForCheats(instanceId)
				if err != nil {
					logger.Warn("CHEAT_CHECK_FAILED", map[string]any{
						logging.INSTANCE_ID: instanceId,
						logging.ERROR:       err.Error(),
					})
				}
			}
		}()
	}

	rows, err := postgres.DB.Query(`SELECT DISTINCT instance_id 
		FROM instance_player 
		JOIN player USING (membership_id)
		WHERE cheat_level >= 3 AND last_seen > NOW() - INTERVAL '60 days'`)
	if err != nil {
		logger.Warn("BLACKLIST_QUERY_ERROR", map[string]any{
			logging.OPERATION: "query_blacklisted_instances",
			logging.ERROR:     err.Error(),
		})
	}
	defer rows.Close()

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			logger.Warn("INSTANCE_ID_SCAN_ERROR", map[string]any{
				logging.ERROR: err.Error(),
			})
		}
		instanceIds <- instanceId
	}
	close(instanceIds)
	wg.Wait()

	logger.Info(PROCESSING_COMPLETE, map[string]any{
		logging.OPERATION: "re-check_blacklisted_instances",
		logging.STATUS:    "complete",
	})

	// step 3: upgrade high flagged instances to blacklisted
	countBlacklisted, err := cheat_detection.BlacklistFlaggedInstances()
	if err != nil {
		logger.Warn(BLACKLIST_UPDATE_ERROR, map[string]any{
			logging.OPERATION: "blacklist_flagged_instances",
			logging.ERROR:     err.Error(),
		})
	}
	logger.Info(BLACKLIST_UPDATED, map[string]any{
		"type":        "flagged_instances",
		logging.COUNT: countBlacklisted,
	})

	// step 4: select players with cheat level 4 and mark their instances as blacklisted
	now := time.Now()
	players, err := cheat_detection.GetRecentlyPlayedBlacklistedPlayers(now.Add(-14 * 24 * time.Hour))
	if err != nil {
		logger.Warn("PLAYER_QUERY_ERROR", map[string]any{
			logging.OPERATION: "get_recently_played_blacklisted",
			logging.ERROR:     err.Error(),
		})
	}

	logger.Info("BLACKLISTED_PLAYERS_FOUND", map[string]any{
		logging.COUNT: len(players),
		"criteria":    "recently_played",
	})
	var totalBlacklistedCount int64
	var totalElligibleCount int64
	for _, player := range players {
		count, elligible, err := cheat_detection.BlacklistRecentInstances(player)
		if err != nil {
			logger.Warn(BLACKLIST_UPDATE_ERROR, map[string]any{
				"membership_id":   player.MembershipId,
				logging.OPERATION: "blacklist_player_instances",
				logging.ERROR:     err.Error(),
			})
		}
		totalBlacklistedCount += count
		totalElligibleCount += elligible
		if count > 0 {
			logger.Info(PLAYER_INSTANCES_BLACKLISTED, map[string]any{
				"membership_id": player.MembershipId,
				logging.COUNT:   count,
				"type":          "recent_instances",
			})
		}
	}
	logger.Info("BLACKLIST_SUMMARY", map[string]any{
		"total_blacklisted": totalBlacklistedCount,
		"total_eligible":    totalElligibleCount,
		"scope":             "all_players",
	})

	logger.Info(PROCESSING_COMPLETE, map[string]any{
		logging.SERVICE: "cheat-detection",
		logging.STATUS:  "complete",
	})
}
