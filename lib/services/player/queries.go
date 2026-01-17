package player

import (
	"context"
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
		SELECT membership_id, membership_type, display_name, last_crawled, history_last_crawled
		FROM player
		WHERE membership_id = $1
	`

	var p Player
	var membershipType *int
	var lastCrawled *time.Time
	var historyLastCrawled *time.Time
	err := postgres.DB.QueryRow(query, membershipId).Scan(
		&p.MembershipId,
		&membershipType,
		&p.DisplayName,
		&lastCrawled,
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
	if lastCrawled != nil {
		p.LastCrawled = *lastCrawled
	}
	if historyLastCrawled != nil {
		p.HistoryLastCrawled = *historyLastCrawled
	}

	return &p, nil
}

// CreateOrUpdatePlayer creates or updates a player
// Returns the saved player DTO, whether it was updated (not created), and any error
func CreateOrUpdatePlayer(p *Player) (*Player, bool, error) {
	now := time.Now()

	// Set default values for required fields if not provided
	firstSeen := p.FirstSeen
	if firstSeen.IsZero() {
		firstSeen = now
	}
	lastSeen := p.LastSeen

	// Check if player exists
	var exists bool
	err := postgres.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM player WHERE membership_id = $1)", p.MembershipId).Scan(&exists)
	if err != nil {
		return nil, false, err
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
		RETURNING 
			membership_id,
			membership_type,
			display_name,
			icon_path,
			bungie_global_display_name,
			bungie_global_display_name_code,
			last_seen,
			first_seen,
			updated_at
	`

	var membershipType *int
	if p.MembershipType != nil {
		membershipType = p.MembershipType
	}

	var savedPlayer Player
	var savedMembershipType *int
	var updatedAt time.Time
	err = postgres.DB.QueryRow(
		query,
		p.MembershipId,
		membershipType,
		p.DisplayName,
		p.IconPath,
		p.BungieGlobalDisplayName,
		p.BungieGlobalDisplayNameCode,
		lastSeen,
		firstSeen,
	).Scan(
		&savedPlayer.MembershipId,
		&savedMembershipType,
		&savedPlayer.DisplayName,
		&savedPlayer.IconPath,
		&savedPlayer.BungieGlobalDisplayName,
		&savedPlayer.BungieGlobalDisplayNameCode,
		&savedPlayer.LastSeen,
		&savedPlayer.FirstSeen,
		&updatedAt,
	)
	if err != nil {
		return nil, false, err
	}

	if savedMembershipType != nil {
		savedPlayer.MembershipType = savedMembershipType
	}

	// Return the saved player, whether it was updated (not created), and nil error
	return &savedPlayer, exists, nil
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

// checkAndUpdatePrivacy checks the current privacy status and updates it if it changed
// Returns true if the privacy status was updated
func checkAndUpdatePrivacy(membershipId int64, isPrivate bool) (bool, error) {
	// Get current privacy status
	var currentIsPrivate bool
	err := postgres.DB.QueryRow("SELECT is_private FROM player WHERE membership_id = $1", membershipId).Scan(&currentIsPrivate)
	if err != nil {
		logger.Warn("ERROR_GETTING_PRIVACY_STATUS", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return false, err
	}

	// Update privacy status if it changed
	if isPrivate != currentIsPrivate {
		_, err = postgres.DB.Exec("UPDATE player SET is_private = $1 WHERE membership_id = $2", isPrivate, membershipId)
		if err != nil {
			logger.Warn("ERROR_UPDATING_PRIVACY_STATUS", err, map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
			})
			return false, err
		}
		logger.Info("HISTORY_PRIVACY_UPDATED", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			"is_private":          isPrivate,
			"was_private":         currentIsPrivate,
		})
		return true, nil
	}

	return false, nil
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
func GetPlayerCharacters(ctx context.Context, membershipId int64) ([]Character, error) {
	// Try to get membership type from database first
	var knownType int
	var membershipType *int
	err := postgres.DB.QueryRow("SELECT membership_type FROM player WHERE membership_id = $1", membershipId).Scan(&membershipType)
	if err == nil && membershipType != nil {
		knownType = *membershipType
	}

	// Use ResolveProfile to get the profile (handles membership type resolution efficiently)
	_, result, err := bungie.ResolveProfile(ctx, membershipId, knownType)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile: %w", err)
	}
	if result.Data == nil {
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
