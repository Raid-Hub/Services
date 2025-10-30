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

// Hera logging constants
const (
	SERVICE_STARTED       = "SERVICE_STARTED"
	PLAYERS_SELECTED      = "PLAYERS_SELECTED"
	PROCESSING_COMPLETE   = "PROCESSING_COMPLETE"
	DB_OPERATION_ERROR    = "DB_OPERATION_ERROR"
	LEADERBOARD_REFRESHED = "LEADERBOARD_REFRESHED"
	PLAYER_GROUPS_FAILED  = "PLAYER_GROUPS_FAILED"
)

type PlayerTransport struct {
	membershipId   int64
	membershipType int
}

var (
	topPlayers = flag.Int("top", 1500, "number of top players to get")
	reqs       = flag.Int("reqs", 14, "number of requests to make to bungie concurrently")
	logger     = logging.NewLogger("HERA")
)

func main() {
	flag.Parse()

	logger.Info(SERVICE_STARTED, map[string]any{
		logging.SERVICE: "hera",
		"purpose":       "leaderboard_maintenance",
	})
	logger.Info(PLAYERS_SELECTED, map[string]any{
		"top_players_count": *topPlayers,
		logging.OPERATION:   "select_leaderboard_players",
	})

	// postgres.DB and rabbit.Conn are initialized in init()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := rabbit.Conn.Channel()
	if err != nil {
		logger.Fatal("Error creating a channel", map[string]any{logging.ERROR: err.Error()})
	}
	defer ch.Close()

	// Get all players who are in the top 1000 of individual leaderboard
	rows, err := postgres.DB.QueryContext(ctx, `
	SELECT DISTINCT ON (membership_id) membership_id, membership_type FROM (
		SELECT membership_id FROM individual_global_leaderboard WHERE (
			clears_rank <= $1 
			OR fresh_clears_rank <= $1 
			OR sherpas_rank <= $1
			OR total_time_played_rank <= $1
			OR speed_rank <= $1
			OR wfr_score_rank <= $1
		)
		UNION
		SELECT membership_id FROM individual_raid_leaderboard WHERE (
			clears_rank <= $1 
			OR fresh_clears_rank <= $1 
			OR sherpas_rank <= $1
			OR total_time_played_rank <= $1
		)
	) as ids
	JOIN player USING (membership_id)
	WHERE membership_type <> 0 AND membership_type <> 4`, *topPlayers)

	if err != nil {
		logger.Fatal("Error querying the database", map[string]any{logging.ERROR: err.Error()})
	}

	logger.Info("Selected all top players.", map[string]any{})

	// Get all groups for each player
	playerCountPointer := new(int32)
	queue := make(chan PlayerTransport, 100)

	groupSet := sync.Map{}

	wg := sync.WaitGroup{}
	for i := 0; i < *reqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for player := range queue {
				attempts := 0
				for attempts < 4 {
					result, err := bungie.Client.GetGroupsForMember(player.membershipType, player.membershipId)
					if err != nil {
						// retry
						attempts++
						time.Sleep(time.Second * time.Duration(attempts*2))
						continue
					}
					if !result.Success || result.Data == nil {
						break
					}
					res := result.Data
					atomic.AddInt32(playerCountPointer, 1)

					for _, group := range res.Results {
						if !res.AreAllMembershipsInactive[group.Group.GroupId] {
							groupSet.Store(group.Group.GroupId, group.Group)
						}
					}
					break
				}
				if attempts >= 4 {
					logger.Warn(PLAYER_GROUPS_FAILED, map[string]any{
						logging.MEMBERSHIP_ID: player.membershipId,
						logging.ATTEMPT:       4,
					})
				}

			}
		}()
	}
	defer rows.Close()

	logger.Info("Grabbing groups for each player...", map[string]any{})
	for rows.Next() {
		player := PlayerTransport{}
		rows.Scan(&player.membershipId, &player.membershipType)
		queue <- player
	}

	close(queue)
	wg.Wait()

	count := 0
	groupSet.Range(func(_, _ any) bool {
		count++
		return true
	})

	logger.Info("Found clans from players", map[string]any{logging.COUNT: count, logging.PLAYERS: *playerCountPointer})

	groupChannel := make(chan bungie.GroupV2)

	tx, err := postgres.DB.BeginTx(ctx, nil)
	if err != nil {
		logger.Warn("Error beginning transaction", map[string]any{logging.ERROR: err.Error()})
	}
	defer tx.Rollback()
	logger.Info("Beginning transaction...", map[string]any{})

	_, err = tx.ExecContext(ctx, `TRUNCATE TABLE clan_members`)
	if err != nil {
		logger.Warn("Error truncating the clan_members table", map[string]any{logging.ERROR: err.Error()})
	}

	logger.Info("Truncated the clan_members table.", map[string]any{})

	upsertClan, err := tx.PrepareContext(ctx, `INSERT INTO clan (group_id, name, motto, call_sign, clan_banner_data, updated_at) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (group_id)
		DO UPDATE SET name = $2, motto = $3, call_sign = $4, clan_banner_data = $5, updated_at = $6`)
	if err != nil {
		logger.Warn("Error preparing the upsert clan statement", map[string]any{logging.ERROR: err.Error()})
	}
	defer upsertClan.Close()

	insertMember, err := tx.PrepareContext(ctx, `INSERT INTO clan_members (group_id, membership_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`)
	if err != nil {
		logger.Warn("Error preparing the insert member statement", map[string]any{logging.ERROR: err.Error()})
	}
	defer insertMember.Close()

	logger.Info("Prepared statements for upserting clans and inserting members.", map[string]any{})

	memberFailurePointer := new(int32)
	clanMemberCountPointer := new(int32)
	for i := 0; i < *reqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for group := range groupChannel {
				clanBannerData, clanName, callSign, motto, err := clan.ParseClanDetails(&group)
				if err != nil {
					logger.Warn("Error parsing clan details", map[string]any{logging.ERROR: err.Error()})
				}

				_, err = upsertClan.ExecContext(ctx, group.GroupId, clanName, motto, callSign, clanBannerData, time.Now().UTC())
				if err != nil {
					logger.Warn("Error upserting clan", map[string]any{logging.GROUP_ID: group.GroupId, logging.ERROR: err.Error()})
				}

				for page := 1; ; page++ {
					var err error
					var results *bungie.SearchResultOfGroupMember
					attempts := 0
					for attempts < 4 {
						result, err := bungie.Client.GetMembersOfGroup(group.GroupId, page)
						if result.BungieErrorCode == bungie.GroupNotFound {
							break
						} else if err != nil {
							logger.Info("Error getting members of group", map[string]any{logging.GROUP_ID: group.GroupId, logging.ERROR: err.Error()})
							attempts++
							continue
						}
						if !result.Success || result.Data == nil {
							break
						}
						results = result.Data

						atomic.AddInt32(clanMemberCountPointer, int32(len(results.Results)))
						for _, member := range results.Results {
							var exists bool
							err := postgres.DB.QueryRowContext(ctx, `SELECT true FROM player WHERE membership_id = $1`, member.DestinyUserInfo.MembershipId).Scan(&exists)
							if err != nil {
								atomic.AddInt32(memberFailurePointer, 1)
								if err == sql.ErrNoRows {
									publishing.PublishTextMessage(ctx, routing.PlayerCrawl, fmt.Sprintf("%d", member.DestinyUserInfo.MembershipId))
									// if member.LastOnlineStatusChange != 0 {
									// 	logger.InfoF("Player %d, last seen %s, is not in the database, sending to player_crawl", member.DestinyUserInfo.MembershipId, time.Unix(member.LastOnlineStatusChange, 0))
									// }
								} else {
									logger.Warn("Error checking if player exists", map[string]any{logging.MEMBERSHIP_ID: member.DestinyUserInfo.MembershipId, logging.ERROR: err.Error()})
								}
							} else {
								_, err := insertMember.ExecContext(ctx, group.GroupId, member.DestinyUserInfo.MembershipId)
								if err != nil {
									logger.Warn("Error inserting member into clan", map[string]any{logging.MEMBERSHIP_ID: member.DestinyUserInfo.MembershipId, logging.GROUP_ID: group.GroupId, logging.ERROR: err.Error()})

								}
							}

						}
						break
					}

					if err != nil || !results.HasMore {
						break
					}
				}
			}
		}()
	}

	// Begin processing the clans
	logger.Info("Processing clans...", map[string]any{})
	groupSet.Range(func(_, group any) bool {
		groupChannel <- group.(bungie.GroupV2)
		return true
	})
	close(groupChannel)
	wg.Wait()

	if err := tx.Commit(); err != nil {
		logger.Warn("Error committing transaction", map[string]any{logging.ERROR: err.Error()})
	}

	logger.Info("Inserted clan members", map[string]any{
		logging.SUCCESSFUL: *clanMemberCountPointer - *memberFailurePointer,
		logging.TOTAL:      *clanMemberCountPointer,
		logging.FAILED:     *memberFailurePointer,
	})

	logger.Info("Refreshing leaderboards...", map[string]any{})
	_, err = postgres.DB.ExecContext(ctx, `REFRESH MATERIALIZED VIEW clan_leaderboard WITH DATA`)
	if err != nil {
		logger.Warn("Error refreshing the clan_leaderboard materialized view", map[string]any{logging.ERROR: err.Error()})
	}

	logger.Info("Done.", map[string]any{})
}
