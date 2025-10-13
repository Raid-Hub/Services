package player_crawl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"raidhub/packages/async"
	"raidhub/packages/bungie"
	"raidhub/packages/pgcr_types"
	"raidhub/packages/postgres"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func process_player_request(qw *async.QueueWorker, msg amqp.Delivery) {
	qw.Wg.Wait()
	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	var request PlayerRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Printf("Failed to unmarshal message: %s", err)
		return
	}

	membershipType, lastCrawled, err := get_player(request.MembershipId, qw.Db)
	if err != nil {
		log.Printf("Failed to get player: %s", err)
		return
	} else if membershipType == -1 || membershipType == 0 {
		log.Printf("Crawling missing player %d", request.MembershipId)
		crawl_player_profiles(request.MembershipId, qw.Db)
	} else if lastCrawled == nil || time.Since(*lastCrawled) > 6*time.Hour {
		crawl_membership(membershipType, request.MembershipId, qw.Db)
	}
}

func get_player(membershipId int64, db *sql.DB) (int, *time.Time, error) {
	var membershipType int
	var lastCrawled sql.NullTime
	err := db.QueryRow(`SELECT COALESCE(membership_type, 0), last_crawled FROM player WHERE membership_id = $1 LIMIT 1`, membershipId).Scan(&membershipType, &lastCrawled)
	if err == sql.ErrNoRows {
		return -1, nil, nil
	} else if err != nil {
		return -1, nil, err
	} else {
		if lastCrawled.Valid {
			return membershipType, &lastCrawled.Time, nil
		} else {
			return membershipType, nil, nil
		}
	}
}

func crawl_player_profiles(destinyMembershipId int64, db *sql.DB) {
	profiles, err := bungie.GetLinkedProfiles(-1, destinyMembershipId, true)
	if err != nil {
		log.Printf("Failed to get linked profiles: %s", err)
	} else if len(profiles) == 0 {
		log.Println("No profiles found")
		return
	}

	var wg sync.WaitGroup
	for _, profile := range profiles {
		wg.Add(1)
		go func(membershipId int64, membershipType int) {
			defer wg.Done()
			crawl_membership(membershipType, membershipId, db)
		}(profile.MembershipId, profile.MembershipType)
	}

	wg.Wait()
}

func crawl_membership(membershipType int, membershipId int64, db *sql.DB) {
	profile, err := bungie.GetProfile(membershipType, membershipId, []int{100, 200})
	if err != nil {
		log.Printf("Failed to get profile %d/%d: %s", membershipType, membershipId, err)
		return
	}

	if profile.Profile.Data == nil {
		log.Printf("Profile component is nil")
		return
	}

	if profile.Characters.Data == nil {
		log.Printf("Characters component is nil")
		return
	}

	var characterId int64
	for key := range *profile.Characters.Data {
		characterId = key
		break
	}

	activities, activityHistoryErrorCode, activityHistoryErr := bungie.GetActivityHistoryPage(membershipType, membershipId, characterId, 150, 0, 2)
	if activityHistoryErr != nil && activityHistoryErrorCode != 1665 {
		log.Printf("Activity history error: %s", activityHistoryErr)
		return
	}
	if activityHistoryErrorCode != 1 && activityHistoryErrorCode != 1665 {
		log.Fatalf("Unexpected activity history error code: %d", activityHistoryErrorCode)
	}
	isPrivate := activityHistoryErrorCode == 1665

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to initiate transaction: %s", err)
		return
	}
	defer tx.Rollback()

	userInfo := profile.Profile.Data.UserInfo
	var bungieGlobalDisplayNameCodeStr *string = nil
	var bungieGlobalDisplayName *string = nil
	if userInfo.BungieGlobalDisplayName == nil || userInfo.BungieGlobalDisplayNameCode == nil || *userInfo.BungieGlobalDisplayName == "" {
		bungieGlobalDisplayName = nil
		bungieGlobalDisplayNameCodeStr = nil
	} else {
		bungieGlobalDisplayName = userInfo.BungieGlobalDisplayName
		bungieGlobalDisplayNameCodeStr = bungie.FixBungieGlobalDisplayNameCode(userInfo.BungieGlobalDisplayNameCode)
	}

	var iconPath *string = nil
	var mostRecentDate time.Time = time.Time{}
	for _, character := range *profile.Characters.Data {
		if iconPath == nil || character.DateLastPlayed.After(mostRecentDate) {
			icon := character.EmblemPath
			iconPath = &icon
			mostRecentDate = character.DateLastPlayed
		}
	}

	firstSeen := time.Now()
	if len(activities) > 0 {
		firstSeen, err = time.Parse(time.RFC3339, activities[len(activities)-1].Period)
		if err != nil {
			log.Fatalf("Failed to parse first seen date: %s", err)
		}
	}

	_, err = postgres.UpsertPlayer(tx, &pgcr_types.Player{
		MembershipId:                userInfo.MembershipId,
		MembershipType:              &userInfo.MembershipType,
		IconPath:                    iconPath,
		DisplayName:                 userInfo.DisplayName,
		BungieGlobalDisplayName:     bungieGlobalDisplayName,
		BungieGlobalDisplayNameCode: bungieGlobalDisplayNameCodeStr,
		LastSeen:                    mostRecentDate,
		FirstSeen:                   firstSeen,
	})
	if err != nil {
		log.Printf("Failed to upsert full player: %s", err)
		return
	}

	// _, err = stats.UpdatePlayerSumOfBest(membershipId, tx)
	// if err != nil {
	// 	log.Printf("Error updating sum of best for membershipId %d: %s", membershipId, err)
	// 	return
	// }

	_, err = tx.Exec(`UPDATE player SET last_crawled = NOW(), is_private = $1 WHERE membership_id = $2`, isPrivate, membershipId)
	if err != nil {
		log.Fatalf("Failed to update last crawled: %s", err)
		return
	}
	if err = tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %s", err)
	} else {
		log.Printf("Upserted player %d", membershipId)

		// go func() {
		// 	// 5% chance to send a history fetch request
		// 	// not needed all the time, but if I feel like there are missing activities, this will help
		// 	if rand.Intn(100) < 5 {
		// 		send_history_fetch_request(outgoing, membershipId)
		// 	}
		// }()
	}
}

// TODO: hacky, would like to import the function instead but i get a cyclic import error
func send_history_fetch_request(ch *amqp.Channel, membershipId int64) error {
	body := fmt.Appendf(nil, "%d", membershipId)

	return ch.PublishWithContext(
		context.Background(),
		"",                 // exchange
		"activity_history", // routing key (queue name)
		true,               // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		},
	)
}
