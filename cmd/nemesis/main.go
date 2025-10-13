package main

import (
	"fmt"
	"log"
	"raidhub/packages/cheat_detection"
	"raidhub/packages/postgres"
	"sync"
	"time"
)

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

func main() {
	log.Println("Starting...")

	db, err := postgres.Connect()
	if err != nil {
		log.Fatalf("Error connecting to the database: %s", err)
	}
	defer db.Close()

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
				lvl, cheaterAccountProbability, cheaterFlags := cheat_detection.UpdatePlayerCheatLevel(db, flag)

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

	rows := cheat_detection.GetAllInstanceFlagsByPlayer(db, flags, fmt.Sprintf("%s%%", versionPrefix))
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
			log.Fatalf("Error scanning row: %s", err)
		}
		flags <- flag
	}
	close(flags)
	wg.Wait()

	// calculate average of each flag type, total flags, and flag count
	log.Printf("Cheat Level Stats:\n")
	for i, levelStats := range stats {
		log.Printf("Cheat Level %d: %d players\n", i, len(levelStats))
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

		log.Printf("  Average Flags A: %.2f, B: %.2f, C: %.2f, D: %.2f, Total: %.2f, Cheater Account Probability: %.2f",
			avgFlagsA, avgFlagsB, avgFlagsC, avgFlagsD, avgTotalFlags, avgCheaterProbability)
	}

	log.Println("Finished processing all flags.")

	// step 2: re-cheat check all level 3+ player instances. can remove this step later.
	instanceIds := make(chan int64)
	wg.Add(numCheatCheckWorkers)
	for i := 0; i < numCheatCheckWorkers; i++ {
		go func() {
			defer wg.Done()
			for instanceId := range instanceIds {
				_, _, _, _, err := cheat_detection.CheckForCheats(instanceId, db)
				if err != nil {
					log.Fatalf("Failed to process cheat_check for instance %d: %s", instanceId, err)
				}
			}
		}()
	}

	rows, err = db.Query(`SELECT DISTINCT instance_id 
		FROM instance_player 
		JOIN player USING (membership_id)
		WHERE cheat_level >= 3 AND last_seen > NOW() - INTERVAL '60 days'`)
	if err != nil {
		log.Fatalf("Error querying blacklisted instance ids: %s", err)
	}
	defer rows.Close()

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			log.Fatalf("Error scanning instance ID: %s", err)
		}
		instanceIds <- instanceId
	}
	close(instanceIds)
	wg.Wait()

	log.Println("Finished re-checking blacklisted player instances.")

	// step 3: upgrade high flagged instances to blacklisted
	countBlacklisted, err := cheat_detection.BlacklistFlaggedInstances(db)
	if err != nil {
		log.Fatalf("Error blacklisting flagged instances: %s", err)
	}
	log.Printf("Blacklisted %d flagged instances", countBlacklisted)

	// step 4: select players with cheat level 4 and mark their instances as blacklisted
	now := time.Now()
	players, err := cheat_detection.GetRecentlyPlayedBlacklistedPlayers(db, now.Add(-14*24*time.Hour))
	if err != nil {
		log.Fatalf("Error getting recently played blacklisted players: %s", err)
	}

	log.Printf("Found %d blacklisted players who have played recently", len(players))
	var totalBlacklistedCount int64
	var totalElligibleCount int64
	for _, player := range players {
		count, elligible, err := cheat_detection.BlacklistRecentInstances(db, player)
		if err != nil {
			log.Fatalf("Error blacklisting instances for player %d: %s", player.MembershipId, err)
		}
		totalBlacklistedCount += count
		totalElligibleCount += elligible
		if count > 0 {
			log.Printf("Blacklisted %d recent instances for player %d", count, player.MembershipId)
		}
	}
	log.Printf("Updated blacklist for %d instances across all players", totalBlacklistedCount)

	log.Println("Done.")
}
