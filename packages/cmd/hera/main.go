package main

import (
	"context"
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

	"github.com/lib/pq"
)

type PlayerTransport struct {
	membershipId   int64
	membershipType int
}

var (
	topPlayers = flag.Int("top", 1500, "number of top players to get")
	reqs       = flag.Int("reqs", 15, "number of requests to make to bungie concurrently")
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
	SELECT membership_id, membership_type FROM (
		SELECT membership_id FROM individual_global_leaderboard WHERE (
			clears_rank <= $1 
			OR fresh_clears_rank <= $1 
			OR sherpas_rank <= $1
			OR total_time_played_rank <= $1
			OR speed_rank <= $1
		)
		UNION
		SELECT membership_id FROM individual_raid_leaderboard WHERE (
			clears_rank <= $1 
			OR fresh_clears_rank <= $1 
			OR sherpas_rank <= $1
			OR total_time_played_rank <= $1
		)
		UNION
		SELECT membership_id FROM world_first_player_rankings WHERE rank <= $1
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
	groups := make(chan bungie.GroupV2)

	wg := sync.WaitGroup{}
	for i := 0; i < *reqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for player := range queue {
				res, err := bungie.GetGroupsForMember(player.membershipType, player.membershipId)
				if err != nil {
					// retry
					res, err = bungie.GetGroupsForMember(player.membershipType, player.membershipId)
					if err != nil {
						log.Fatalf("Error getting groups for player %d: %s", player.membershipId, err)
					}
				}
				atomic.AddInt32(playerCountPointer, 1)

				for _, group := range res.Results {
					if !res.AreAllMembershipsInactive[group.Group.GroupId] {
						groups <- group.Group
					}
				}
			}
		}()
	}

	groupSet := make(map[int64]bungie.GroupV2)
	wg2 := sync.WaitGroup{}
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for group := range groups {
			groupSet[group.GroupId] = group
		}
	}()

	defer rows.Close()
	for rows.Next() {
		player := PlayerTransport{}
		rows.Scan(&player.membershipId, &player.membershipType)
		queue <- player
	}

	close(queue)
	wg.Wait()
	close(groups)
	wg2.Wait()

	log.Printf("Got all %d clans from %d players.", len(groupSet), *playerCountPointer)

	// Begin processing the clans
	groupChannel := make(chan bungie.GroupV2, len(groupSet))
	for _, group := range groupSet {
		groupChannel <- group
	}
	close(groupChannel)

	_, err = db.ExecContext(ctx, `TRUNCATE TABLE clan_members`)
	if err != nil {
		log.Fatalf("Error truncating the clan_members table: %s", err)
	}

	log.Println("Truncated the clan_members table.")

	upsertClan, err := db.PrepareContext(ctx, `INSERT INTO clan (group_id, name, motto, call_sign, clan_banner_data, updated_at) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (group_id)
		DO UPDATE SET name = $2, motto = $3, call_sign = $4, clan_banner_data = $5, updated_at = $6`)
	if err != nil {
		log.Fatalf("Error preparing the upsert clan statement: %s", err)
	}

	insertMember, err := db.PrepareContext(ctx, `INSERT INTO clan_members (group_id, membership_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`)
	if err != nil {
		log.Fatalf("Error preparing the insert member statement: %s", err)
	}

	log.Println("Prepared the insert statements.")

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
					results, _, err := bungie.GetMembersOfGroup(group.GroupId, page)

					if err != nil {
						time.Sleep(5 * time.Second)
						results2, errCode, err := bungie.GetMembersOfGroup(group.GroupId, page)
						if errCode == 686 { // Group not found
							break
						}
						if err != nil {
							log.Fatalf("Error getting members of group %d: %s", group.GroupId, err)
						}
						results = results2
					}

					atomic.AddInt32(clanMemberCountPointer, int32(len(results.Results)))
					for _, member := range results.Results {
						_, err := insertMember.ExecContext(ctx, group.GroupId, member.DestinyUserInfo.MembershipId)
						if err != nil {
							atomic.AddInt32(memberFailurePointer, 1)
							if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
								player_crawl.SendMessage(ch, member.DestinyUserInfo.MembershipId)
							} else {
								log.Fatalf("Error inserting into the clan_members table: %s", err)
							}
						}
					}

					if !results.HasMore {
						break
					}
				}
			}
		}()
	}
	wg.Wait()
	log.Printf("Inserted %d/%d clan members, failed on %d", *clanMemberCountPointer-*memberFailurePointer, *clanMemberCountPointer, *memberFailurePointer)

	_, err = db.ExecContext(ctx, `REFRESH MATERIALIZED VIEW clan_leaderboard WITH DATA`)
	if err != nil {
		log.Fatalf("Error refreshing the clan_leaderboard materialized view: %s", err)
	}

	log.Println("Refreshed Materialized View.")
	log.Println("Done.")
}
