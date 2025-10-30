package player

import (
	"database/sql"
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"strconv"
	"time"
)

var logger = logging.NewLogger("PLAYER_SERVICE")

// GetPlayer retrieves a player by membership ID
func GetPlayer(membershipId int64) (*Player, error) {
	query := `
		SELECT membership_id, membership_type, display_name, history_last_crawled
		FROM player
		WHERE membership_id = $1
	`

	var p Player
	var membershipType *int
	var historyLastCrawled *time.Time
	err := postgres.DB.QueryRow(query, membershipId).Scan(
		&p.MembershipId,
		&membershipType,
		&p.DisplayName,
		&historyLastCrawled,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Player doesn't exist yet
		}
		return nil, err
	}

	if membershipType != nil {
		p.MembershipType = membershipType
	}
	if historyLastCrawled != nil {
		p.HistoryLastCrawled = *historyLastCrawled
	}

	return &p, nil
}

// CreateOrUpdatePlayer creates or updates a player
func CreateOrUpdatePlayer(p *Player) error {
	now := time.Now()

	// Set default values for required fields if not provided
	lastSeen := p.LastSeen
	if lastSeen.IsZero() {
		lastSeen = now
	}
	firstSeen := p.FirstSeen
	if firstSeen.IsZero() {
		firstSeen = now
	}

	query := `
		INSERT INTO player (
			membership_id, 
			membership_type,
			display_name, 
			icon_path,
			bungie_global_display_name,
			bungie_global_display_name_code,
			last_seen,
			first_seen,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (membership_id) 
		DO UPDATE SET 
			membership_type = COALESCE(EXCLUDED.membership_type, player.membership_type),
			display_name = COALESCE(EXCLUDED.display_name, player.display_name),
			icon_path = COALESCE(EXCLUDED.icon_path, player.icon_path),
			bungie_global_display_name = COALESCE(EXCLUDED.bungie_global_display_name, player.bungie_global_display_name),
			bungie_global_display_name_code = COALESCE(EXCLUDED.bungie_global_display_name_code, player.bungie_global_display_name_code),
			last_seen = GREATEST(player.last_seen, EXCLUDED.last_seen),
			first_seen = LEAST(player.first_seen, EXCLUDED.first_seen),
			updated_at = NOW()
	`

	var membershipType *int
	if p.MembershipType != nil {
		membershipType = p.MembershipType
	}

	_, err := postgres.DB.Exec(
		query,
		p.MembershipId,
		membershipType,
		p.DisplayName,
		p.IconPath,
		p.BungieGlobalDisplayName,
		p.BungieGlobalDisplayNameCode,
		lastSeen,
		firstSeen,
	)
	return err
}

// UpdateHistoryLastCrawled updates the timestamp when player history was last crawled
func UpdateHistoryLastCrawled(membershipId int64) error {
	query := `
		UPDATE player
		SET history_last_crawled = NOW(), updated_at = NOW()
		WHERE membership_id = $1
	`

	_, err := postgres.DB.Exec(query, membershipId)
	return err
}

// GetPlayersNeedingHistoryUpdate gets players that need their history updated
func GetPlayersNeedingHistoryUpdate(limit int) ([]int64, error) {
	query := `
		SELECT membership_id 
		FROM player
		WHERE history_last_crawled IS NULL 
		   OR history_last_crawled < NOW() - INTERVAL '25 weeks'
		ORDER BY clears DESC
		LIMIT $1
	`

	rows, err := postgres.DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var membershipIds []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		membershipIds = append(membershipIds, id)
	}

	return membershipIds, rows.Err()
}

// GetPlayerCharacters retrieves all character IDs for a player from their profile
// This function fetches character IDs from the Bungie API profile response
func GetPlayerCharacters(membershipId int64) ([]Character, error) {
	// Get membership type - try common membership types to find the correct one
	membershipType := 0
	membershipTypes := bungie.AllViableMembershipTypes

	for _, mt := range membershipTypes {
		result, err := bungie.Client.GetLinkedProfiles(mt, membershipId, true)
		if err != nil {
			continue
		}
		if !result.Success || result.Data == nil {
			continue
		}

		// Check if any profile matches our membership ID
		for _, profile := range result.Data.Profiles {
			if profile.MembershipId == membershipId {
				membershipType = profile.MembershipType
				break
			}
		}

		if membershipType != 0 {
			break
		}
	}

	if membershipType == 0 {
		return nil, fmt.Errorf("could not determine membership type for membership ID: %d", membershipId)
	}

	// Fetch profile from Bungie API to get character IDs
	result, err := bungie.Client.GetProfile(membershipType, membershipId, []int{100})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	if !result.Success || result.Data == nil {
		return nil, fmt.Errorf("bungie api returned unsuccessful response")
	}

	profile := result.Data
	if profile.Profile.Data == nil {
		return []Character{}, nil
	}

	// Extract character IDs from profile
	var characters []Character
	for _, charIdStr := range profile.Profile.Data.CharacterIds {
		charId, err := strconv.ParseInt(charIdStr, 10, 64)
		if err != nil {
			continue
		}
		characters = append(characters, Character{
			MembershipId: membershipId,
			CharacterID:  charId,
		})
	}

	return characters, nil
}
