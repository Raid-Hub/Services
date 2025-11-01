package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/clan"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"sync"
	"sync/atomic"
	"time"
)

var logger = logging.NewLogger("LEADERBOARD_CLAN_CRAWL")

type PlayerTransport struct {
	membershipId   int64
	membershipType int
}

// LeaderboardClanCrawl crawls clans for top leaderboard players
// Usage: ./bin/leaderboard-clan-crawl [--top=<number>] [--reqs=<number>]
func LeaderboardClanCrawl() {
	fs := flag.NewFlagSet("leaderboard-clan-crawl", flag.ExitOnError)
	topPlayers := fs.Int("top", 1500, "number of top players to get")
	reqs := fs.Int("reqs", 14, "number of concurrent Bungie API requests")
	fs.Parse(flag.Args())

	logger.Info("SERVICE_STARTED", map[string]any{
		logging.SERVICE: "leaderboard-clan-crawl",
		"purpose":       "leaderboard_maintenance",
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postgres.Wait()
	rabbit.Wait()

	ch, err := rabbit.Conn.Channel()
	if err != nil {
		logger.Fatal("ERROR_CREATING_CHANNEL", map[string]any{logging.ERROR: err.Error()})
	}
	defer ch.Close()

	// Query top players from leaderboards
	players, err := queryTopPlayers(ctx, *topPlayers)
	if err != nil {
		logger.Fatal("ERROR_QUERYING_DATABASE", map[string]any{logging.ERROR: err.Error()})
	}

	logger.Info("PLAYERS_SELECTED", map[string]any{
		"query_limit": *topPlayers,
		logging.COUNT: len(players),
	})

	if len(players) == 0 {
		logger.Warn("NO_PLAYERS_FOUND", map[string]any{
			"message": "No top players found in leaderboards. Views may need refreshing.",
		})
		return
	}

	// Fetch clans for all players
	startTime := time.Now()
	logger.Info("FETCHING_CLANS_STARTED", map[string]any{
		"total_players": len(players),
		"concurrency":   *reqs,
	})
	clans, processedCount := fetchClansForPlayers(ctx, players, *reqs)
	duration := time.Since(startTime)
	logger.Info("CLANS_FETCHED", map[string]any{
		logging.COUNT:           len(clans),
		"players_processed_api": processedCount,
		logging.DURATION:        duration.String(),
	})

	if len(clans) == 0 {
		logger.Warn("NO_CLANS_FOUND", map[string]any{
			"message": "No clans found for top players",
		})
		return
	}

	// Process clan members and update database
	logger.Info("PROCESSING_CLANS_STARTED", map[string]any{
		"total_clans": len(clans),
		"concurrency": *reqs,
	})
	stats := processClanMembers(ctx, clans, *reqs)
	logger.Info("PROCESSING_COMPLETE", map[string]any{
		"clans_processed":      len(clans),
		"members_successful":   stats.successful,
		"members_total":        stats.total,
		"members_failed":       stats.failed,
		"members_queued_crawl": stats.queuedForCrawl,
	})

	// Refresh clan leaderboard
	if err := refreshClanLeaderboard(ctx); err != nil {
		logger.Warn("ERROR_REFRESHING_CLAN_LEADERBOARD", map[string]any{logging.ERROR: err.Error()})
	}
}

func queryTopPlayers(ctx context.Context, limit int) ([]PlayerTransport, error) {
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT DISTINCT ON (p.membership_id) p.membership_id, p.membership_type
		FROM (
			SELECT membership_id 
			FROM leaderboard.individual_global_leaderboard 
			WHERE clears_rank <= $1 
			   OR fresh_clears_rank <= $1 
			   OR sherpas_rank <= $1
			   OR total_time_played_rank <= $1
			   OR speed_rank <= $1
			   OR wfr_score_rank <= $1
			UNION
			SELECT membership_id 
			FROM leaderboard.individual_raid_leaderboard 
			WHERE clears_rank <= $1 
			   OR fresh_clears_rank <= $1 
			   OR sherpas_rank <= $1
			   OR total_time_played_rank <= $1
		) AS ids
		JOIN core.player p ON p.membership_id = ids.membership_id
		WHERE p.membership_type <> 0 AND p.membership_type <> 4`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []PlayerTransport
	for rows.Next() {
		var p PlayerTransport
		if err := rows.Scan(&p.membershipId, &p.membershipType); err != nil {
			return nil, err
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func fetchClansForPlayers(ctx context.Context, players []PlayerTransport, concurrency int) (map[int64]bungie.GroupV2, int32) {
	playerQueue := make(chan PlayerTransport, len(players))
	clanMap := sync.Map{}
	processedCount := new(int32)
	totalPlayers := int32(len(players))
	startTime := time.Now()
	done := make(chan bool)

	// Progress logging ticker
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var wg sync.WaitGroup

	// Progress logger goroutine
	go func() {
		for {
			select {
			case <-ticker.C:
				count := atomic.LoadInt32(processedCount)
				percent := float64(count) / float64(totalPlayers) * 100
				elapsed := time.Since(startTime)
				logger.Info("FETCHING_CLANS_PROGRESS", map[string]any{
					"processed": count,
					"total":     totalPlayers,
					"percent":   fmt.Sprintf("%.1f%%", percent),
					"elapsed":   elapsed.String(),
				})
			case <-done:
				return
			}
		}
	}()

	// Worker goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for player := range playerQueue {
				groups, err := fetchPlayerGroups(ctx, player)
				if err != nil {
					logger.Warn("PLAYER_GROUPS_FAILED", map[string]any{
						logging.MEMBERSHIP_ID: player.membershipId,
						logging.ERROR:         err.Error(),
					})
					continue
				}
				if groups == nil {
					continue
				}

				atomic.AddInt32(processedCount, 1)
				for groupId, group := range groups {
					clanMap.Store(groupId, group)
				}
			}
		}()
	}

	for _, player := range players {
		playerQueue <- player
	}
	close(playerQueue)
	wg.Wait()
	ticker.Stop()
	close(done)

	// Final progress log
	finalCount := atomic.LoadInt32(processedCount)
	if finalCount > 0 {
		percent := float64(finalCount) / float64(totalPlayers) * 100
		elapsed := time.Since(startTime)
		logger.Info("FETCHING_CLANS_PROGRESS", map[string]any{
			"processed": finalCount,
			"total":     totalPlayers,
			"percent":   fmt.Sprintf("%.1f%%", percent),
			"elapsed":   elapsed.String(),
		})
	}

	result := make(map[int64]bungie.GroupV2)
	clanMap.Range(func(key, value any) bool {
		result[key.(int64)] = value.(bungie.GroupV2)
		return true
	})

	return result, *processedCount
}

func fetchPlayerGroups(ctx context.Context, player PlayerTransport) (map[int64]bungie.GroupV2, error) {
	for attempt := 0; attempt < 4; attempt++ {
		result, err := bungie.Client.GetGroupsForMember(player.membershipType, player.membershipId)
		if err != nil {
			logger.Warn("GET_GROUPS_ERROR", map[string]any{
				logging.MEMBERSHIP_ID: player.membershipId,
				"membership_type":     player.membershipType,
				logging.ERROR:         err.Error(),
			})
			time.Sleep(time.Second * time.Duration((attempt+1)*2))
			continue
		}
		if !result.Success || result.Data == nil {
			return nil, nil
		}

		groups := make(map[int64]bungie.GroupV2)
		for _, g := range result.Data.Results {
			// Only include active memberships
			if !result.Data.AreAllMembershipsInactive[g.Group.GroupId] {
				groups[g.Group.GroupId] = g.Group
			}
		}
		return groups, nil
	}
	return nil, nil
}

type processingStats struct {
	total          int32
	successful     int32
	failed         int32
	queuedForCrawl int32
}

func processClanMembers(ctx context.Context, clans map[int64]bungie.GroupV2, concurrency int) processingStats {
	tx, err := postgres.DB.BeginTx(ctx, nil)
	if err != nil {
		logger.Fatal("ERROR_BEGINNING_TRANSACTION", map[string]any{logging.ERROR: err.Error()})
	}
	defer tx.Rollback()

	// Truncate clan members table
	if _, err := tx.ExecContext(ctx, `TRUNCATE TABLE clan.clan_members`); err != nil {
		logger.Fatal("ERROR_TRUNCATING_CLAN_MEMBERS", map[string]any{logging.ERROR: err.Error()})
	}

	// Prepare statements
	upsertClan, err := tx.PrepareContext(ctx, `
		INSERT INTO clan.clan (group_id, name, motto, call_sign, clan_banner_data, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (group_id)
		DO UPDATE SET name = $2, motto = $3, call_sign = $4, clan_banner_data = $5, updated_at = $6`)
	if err != nil {
		logger.Fatal("ERROR_PREPARING_UPSERT_CLAN", map[string]any{logging.ERROR: err.Error()})
	}
	defer upsertClan.Close()

	insertMember, err := tx.PrepareContext(ctx, `
		INSERT INTO clan.clan_members (group_id, membership_id) 
		VALUES ($1, $2) 
		ON CONFLICT DO NOTHING`)
	if err != nil {
		logger.Fatal("ERROR_PREPARING_INSERT_MEMBER", map[string]any{logging.ERROR: err.Error()})
	}
	defer insertMember.Close()

	// Process clans concurrently
	clanQueue := make(chan bungie.GroupV2, len(clans))
	stats := processingStats{}
	totalClans := int32(len(clans))
	processedClans := new(int32)
	startTime := time.Now()
	done := make(chan bool)

	// Progress logging ticker
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var wg sync.WaitGroup

	// Progress logger goroutine
	go func() {
		for {
			select {
			case <-ticker.C:
				count := atomic.LoadInt32(processedClans)
				total := atomic.LoadInt32(&stats.total)
				successful := atomic.LoadInt32(&stats.successful)
				percent := float64(count) / float64(totalClans) * 100
				elapsed := time.Since(startTime)
				logger.Info("PROCESSING_CLANS_PROGRESS", map[string]any{
					"processed":     count,
					"total":         totalClans,
					"percent":       fmt.Sprintf("%.1f%%", percent),
					"elapsed":       elapsed.String(),
					"members_total": total,
					"members_ok":    successful,
				})
			case <-done:
				return
			}
		}
	}()

	// Worker goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for group := range clanQueue {
				clanStats := processClan(ctx, tx, upsertClan, insertMember, &group)
				atomic.AddInt32(&stats.total, clanStats.total)
				atomic.AddInt32(&stats.successful, clanStats.successful)
				atomic.AddInt32(&stats.failed, clanStats.failed)
				atomic.AddInt32(&stats.queuedForCrawl, clanStats.queuedForCrawl)
				atomic.AddInt32(processedClans, 1)
			}
		}()
	}

	for _, clan := range clans {
		clanQueue <- clan
	}
	close(clanQueue)
	wg.Wait()
	ticker.Stop()
	close(done)

	// Final progress log
	finalCount := atomic.LoadInt32(processedClans)
	if finalCount > 0 {
		percent := float64(finalCount) / float64(totalClans) * 100
		elapsed := time.Since(startTime)
		total := atomic.LoadInt32(&stats.total)
		successful := atomic.LoadInt32(&stats.successful)
		logger.Info("PROCESSING_CLANS_PROGRESS", map[string]any{
			"processed":     finalCount,
			"total":         totalClans,
			"percent":       fmt.Sprintf("%.1f%%", percent),
			"elapsed":       elapsed.String(),
			"members_total": total,
			"members_ok":    successful,
		})
	}

	if err := tx.Commit(); err != nil {
		logger.Fatal("ERROR_COMMITTING_TRANSACTION", map[string]any{logging.ERROR: err.Error()})
	}

	return stats
}

func processClan(ctx context.Context, tx *sql.Tx, upsertClan, insertMember *sql.Stmt, group *bungie.GroupV2) processingStats {
	stats := processingStats{}

	// Parse and upsert clan details
	clanBannerData, clanName, callSign, motto, err := clan.ParseClanDetails(group)
	if err != nil {
		logger.Warn("ERROR_PARSING_CLAN_DETAILS", map[string]any{
			logging.GROUP_ID: group.GroupId,
			logging.ERROR:    err.Error(),
		})
	}

	if _, err := upsertClan.ExecContext(ctx, group.GroupId, clanName, motto, callSign, clanBannerData, time.Now().UTC()); err != nil {
		logger.Warn("ERROR_UPSERTING_CLAN", map[string]any{
			logging.GROUP_ID: group.GroupId,
			logging.ERROR:    err.Error(),
		})
	}

	// Fetch and insert all clan members (with pagination)
	queuedForCrawl := 0
	for page := 1; ; page++ {
		members, hasMore, err := fetchClanMembersPage(ctx, group.GroupId, page)
		if err != nil {
			logger.Warn("ERROR_FETCHING_CLAN_MEMBERS", map[string]any{
				logging.GROUP_ID: group.GroupId,
				"page":           page,
				logging.ERROR:    err.Error(),
			})
			break
		}
		if members == nil {
			break
		}

		for _, member := range members {
			stats.total++
			membershipId := member.DestinyUserInfo.MembershipId

			// Check if player exists in database
			var exists bool
			err := postgres.DB.QueryRowContext(ctx, `SELECT true FROM core.player WHERE membership_id = $1`, membershipId).Scan(&exists)
			if err != nil {
				stats.failed++
				if err == sql.ErrNoRows {
					// Player doesn't exist, send to crawl queue
					queuedForCrawl++
					publishing.PublishInt64Message(ctx, routing.PlayerCrawl, membershipId)
				} else {
					logger.Warn("ERROR_CHECKING_PLAYER_EXISTS", map[string]any{
						logging.MEMBERSHIP_ID: membershipId,
						logging.ERROR:         err.Error(),
					})
				}
				continue
			}

			// Insert clan member
			if _, err := insertMember.ExecContext(ctx, group.GroupId, membershipId); err != nil {
				logger.Warn("ERROR_INSERTING_MEMBER", map[string]any{
					logging.MEMBERSHIP_ID: membershipId,
					logging.GROUP_ID:      group.GroupId,
					logging.ERROR:         err.Error(),
				})
				stats.failed++
			} else {
				stats.successful++
			}
		}

		if !hasMore {
			break
		}
	}

	if queuedForCrawl > 0 {
		logger.Debug("CLAN_MEMBERS_QUEUED_FOR_CRAWL", map[string]any{
			logging.GROUP_ID: group.GroupId,
			logging.COUNT:    queuedForCrawl,
		})
	}

	stats.queuedForCrawl = int32(queuedForCrawl)
	return stats
}

func fetchClanMembersPage(ctx context.Context, groupId int64, page int) ([]bungie.GroupMember, bool, error) {
	for attempt := 0; attempt < 4; attempt++ {
		result, err := bungie.Client.GetMembersOfGroup(groupId, page)
		if err != nil {
			if attempt < 3 {
				time.Sleep(time.Second * time.Duration((attempt+1)*2))
			}
			continue
		}

		if result.BungieErrorCode == bungie.GroupNotFound {
			return nil, false, nil
		}

		if !result.Success || result.Data == nil {
			return nil, false, nil
		}

		return result.Data.Results, result.Data.HasMore, nil
	}
	return nil, false, nil
}

func refreshClanLeaderboard(ctx context.Context) error {
	logger.Debug("REFRESHING_LEADERBOARDS", map[string]any{})
	_, err := postgres.DB.ExecContext(ctx, `REFRESH MATERIALIZED VIEW leaderboard.clan_leaderboard WITH DATA`)
	return err
}

func main() {
	flag.Parse()
	LeaderboardClanCrawl()
}
