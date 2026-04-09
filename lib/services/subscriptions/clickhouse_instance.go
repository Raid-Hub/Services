package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"raidhub/lib/database/clickhouse"
	"raidhub/lib/database/postgres"
	"raidhub/lib/dto"

	"github.com/lib/pq"
)

// LoadDTOInstanceFromClickHouse loads one instance row from ClickHouse by id, or when filterInstanceID is 0
// selects the most recent instance by date_completed.
func LoadDTOInstanceFromClickHouse(ctx context.Context, filterInstanceID int64) (*dto.Instance, error) {
	const pickLatest = `
SELECT instance_id
FROM instance FINAL
WHERE (? = toInt64(0) OR instance_id = ?)
ORDER BY date_completed DESC
LIMIT 1`

	var instanceID int64
	err := clickhouse.DB.QueryRow(ctx, pickLatest, filterInstanceID, filterInstanceID).Scan(&instanceID)
	if err != nil {
		return nil, fmt.Errorf("pick instance: %w", err)
	}

	const q = `
SELECT
	instance_id,
	hash,
	completed,
	player_count,
	date_started,
	date_completed,
	duration,
	platform_type,
	score,
	players.membership_id,
	players.completed
FROM instance FINAL
WHERE instance_id = ?`

	var (
		hash            uint32
		completedU8     uint8
		playerCount     uint32
		dateStarted     time.Time
		dateCompleted   time.Time
		duration        uint32
		platformType    uint16
		score           int32
		membershipIDs   []int64
		playersComplete []uint8
	)

	err = clickhouse.DB.QueryRow(ctx, q, instanceID).Scan(
		&instanceID,
		&hash,
		&completedU8,
		&playerCount,
		&dateStarted,
		&dateCompleted,
		&duration,
		&platformType,
		&score,
		&membershipIDs,
		&playersComplete,
	)
	if err != nil {
		return nil, fmt.Errorf("load instance row: %w", err)
	}

	types, err := loadMembershipTypesForInstance(ctx, membershipIDs)
	if err != nil {
		return nil, err
	}

	defaultMT := 1
	players := make([]dto.InstancePlayer, len(membershipIDs))
	for i, mid := range membershipIDs {
		mt := types[mid]
		if mt == nil {
			mt = &defaultMT
		}
		fin := false
		if i < len(playersComplete) && playersComplete[i] != 0 {
			fin = true
		}
		players[i] = dto.InstancePlayer{
			Finished: fin,
			Player: dto.PlayerInfo{
				MembershipId:   mid,
				MembershipType: mt,
			},
		}
	}

	return &dto.Instance{
		InstanceId:      instanceID,
		Hash:            hash,
		Completed:       completedU8 != 0,
		PlayerCount:     int(playerCount),
		DateStarted:     dateStarted,
		DateCompleted:   dateCompleted,
		DurationSeconds: int(duration),
		MembershipType:  int(platformType),
		Score:           int(score),
		Players:         players,
	}, nil
}

func loadMembershipTypesForInstance(ctx context.Context, ids []int64) (map[int64]*int, error) {
	out := make(map[int64]*int)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := postgres.DB.QueryContext(ctx,
		`SELECT membership_id, membership_type FROM core.player WHERE membership_id = ANY($1)`,
		pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int64
		var mt sql.NullInt32
		if err := rows.Scan(&mid, &mt); err != nil {
			return nil, err
		}
		if mt.Valid {
			v := int(mt.Int32)
			out[mid] = &v
		}
	}
	return out, rows.Err()
}
