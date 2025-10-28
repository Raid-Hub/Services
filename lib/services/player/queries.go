package player

import (
	"database/sql"
	"raidhub/lib/database/postgres"
	"time"
)

// GetPlayer retrieves a player by membership ID
func GetPlayer(membershipId int64) (*Player, error) {
	query := `
		SELECT membership_id, display_name, history_last_crawled
		FROM player
		WHERE membership_id = $1
	`

	var p Player
	var historyLastCrawled *time.Time
	err := postgres.DB.QueryRow(query, membershipId).Scan(
		&p.MembershipId,
		&p.DisplayName,
		&historyLastCrawled,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Player doesn't exist yet
		}
		return nil, err
	}

	if historyLastCrawled != nil {
		p.HistoryLastCrawled = *historyLastCrawled
	}

	return &p, nil
}

// CreateOrUpdatePlayer creates or updates a player
func CreateOrUpdatePlayer(p *Player) error {
	query := `
		INSERT INTO player (membership_id, display_name, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (membership_id) 
		DO UPDATE SET display_name = EXCLUDED.display_name, updated_at = NOW()
	`

	_, err := postgres.DB.Exec(query, p.MembershipId, p.DisplayName)
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

// GetPlayerCharacters retrieves all characters for a player
func GetPlayerCharacters(membershipId int64) ([]Character, error) {
	query := `
		SELECT character_id, membership_id, character_id
		FROM character
		WHERE membership_id = $1
	`

	rows, err := postgres.DB.Query(query, membershipId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var characters []Character
	for rows.Next() {
		var c Character
		if err := rows.Scan(&c.ID, &c.MembershipId, &c.CharacterID); err != nil {
			return nil, err
		}
		characters = append(characters, c)
	}

	return characters, rows.Err()
}
