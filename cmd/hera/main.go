package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"raidhub/packages/async/player_crawl"
	"raidhub/packages/bungie"
	"raidhub/packages/clan"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"
	"sync"
	"sync/atomic"
	"time"
)

type PlayerTransport struct {
	membershipId   int64
	membershipType int
}

var (
	topPlayers = flag.Int("top", 1500, "number of top players to get")
	reqs       = flag.Int("reqs", 14, "number of requests to make to bungie concurrently")
)

func main() {
	flag.Parse()

	log.Println("Starting...")
	log.Printf("Selecting the top %d players from each leaderboard...", *topPlayers)

	db, err := postgres.Connect()
	if err != nil {
		log.Fatalf("Error connecting to the database: %s", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := rabbit.Init()
	if err != nil {
		log.Fatalf("Error connecting to RabbitMQ: %s", err)
	}
	defer rabbit.Cleanup()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Error creating a channel: %s", err)
	}
	defer ch.Close()

	// Get all players who are in the top 1000 of individual leaderboard
	rows, err := db.QueryContext(ctx, `
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
		log.Fatalf("Error querying the database: %s", err)
	}

	log.Println("Selected all top players.")

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
					res, err := bungie.GetGroupsForMember(player.membershipType, player.membershipId)
					if err != nil {
						// retry
						attempts++
						time.Sleep(time.Second * time.Duration(attempts*2))
						continue
					}
					atomic.AddInt32(playerCountPointer, 1)

					for _, group := range res.Results {
						if !res.AreAllMembershipsInactive[group.Group.GroupId] {
							groupSet.Store(group.Group.GroupId, group.Group)
						}
					}
					break
				}
				if attempts >= 4 {
					log.Printf("Failed to get groups for player %d after 4 attempts", player.membershipId)
				}

			}
		}()
	}
	defer rows.Close()

	log.Println("Grabbing groups for each player...")
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

	log.Printf("Found %d clans from %d players.", count, *playerCountPointer)

	groupChannel := make(chan bungie.GroupV2)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("Error beginning transaction: %s", err)
	}
	defer tx.Rollback()
	log.Println("Beginning transaction...")

	_, err = tx.ExecContext(ctx, `TRUNCATE TABLE clan_members`)
	if err != nil {
		log.Fatalf("Error truncating the clan_members table: %s", err)
	}

	log.Println("Truncated the clan_members table.")

	upsertClan, err := tx.PrepareContext(ctx, `INSERT INTO clan (group_id, name, motto, call_sign, clan_banner_data, updated_at) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (group_id)
		DO UPDATE SET name = $2, motto = $3, call_sign = $4, clan_banner_data = $5, updated_at = $6`)
	if err != nil {
		log.Fatalf("Error preparing the upsert clan statement: %s", err)
	}
	defer upsertClan.Close()

	insertMember, err := tx.PrepareContext(ctx, `INSERT INTO clan_members (group_id, membership_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`)
	if err != nil {
		log.Fatalf("Error preparing the insert member statement: %s", err)
	}
	defer insertMember.Close()

	log.Println("Prepared statements for upserting clans and inserting members.")

	memberFailurePointer := new(int32)
	clanMemberCountPointer := new(int32)
	for i := 0; i < *reqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for group := range groupChannel {
				clanBannerData, clanName, callSign, motto, err := clan.ParseClanDetails(&group)
				if err != nil {
					log.Fatalf("Error parsing clan details: %s", err)
				}

				_, err = upsertClan.ExecContext(ctx, group.GroupId, clanName, motto, callSign, clanBannerData, time.Now().UTC())
				if err != nil {
					log.Fatalf("Error upserting clan %d: %s", group.GroupId, err)
				}

				for page := 1; ; page++ {
					var err error
					var results *bungie.SearchResultOfGroupMember
					var errCode int
					attempts := 0
					for attempts < 4 {
						results, errCode, err = bungie.GetMembersOfGroup(group.GroupId, page)
						if errCode == 686 { // Group not found
							break
						} else if err != nil {
							log.Printf("Error getting members of group %d: %s", group.GroupId, err)
							attempts++
							continue
						}

						atomic.AddInt32(clanMemberCountPointer, int32(len(results.Results)))
						for _, member := range results.Results {
							var exists bool
							err := db.QueryRowContext(ctx, `SELECT true FROM player WHERE membership_id = $1`, member.DestinyUserInfo.MembershipId).Scan(&exists)
							if err != nil {
								atomic.AddInt32(memberFailurePointer, 1)
								if err == sql.ErrNoRows {
									player_crawl.SendMessage(ch, member.DestinyUserInfo.MembershipId)
									// if member.LastOnlineStatusChange != 0 {
									// 	log.Printf("Player %d, last seen %s, is not in the database, sending to player_crawl", member.DestinyUserInfo.MembershipId, time.Unix(member.LastOnlineStatusChange, 0))
									// }
								} else {
									log.Fatalf("Error checking if player %d exists: %s", member.DestinyUserInfo.MembershipId, err)
								}
							} else {
								_, err := insertMember.ExecContext(ctx, group.GroupId, member.DestinyUserInfo.MembershipId)
								if err != nil {
									log.Fatalf("Error inserting member %d into clan %d: %s", member.DestinyUserInfo.MembershipId, group.GroupId, err)

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
	log.Println("Processing clans...")
	groupSet.Range(func(_, group any) bool {
		groupChannel <- group.(bungie.GroupV2)
		return true
	})
	close(groupChannel)
	wg.Wait()

	if err := tx.Commit(); err != nil {
		log.Fatalf("Error committing transaction: %s", err)
	}

	log.Printf("Inserted %d/%d clan members, failed on %d", *clanMemberCountPointer-*memberFailurePointer, *clanMemberCountPointer, *memberFailurePointer)

	log.Println("Refreshing leaderboards...")
	_, err = db.ExecContext(ctx, `REFRESH MATERIALIZED VIEW clan_leaderboard WITH DATA`)
	if err != nil {
		log.Fatalf("Error refreshing the clan_leaderboard materialized view: %s", err)
	}

	log.Println("Done.")
}
