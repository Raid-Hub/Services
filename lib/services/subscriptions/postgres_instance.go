package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/dto"

	"github.com/lib/pq"
)

// LoadDTOInstanceFromPostgres loads one instance from core.instance + core.instance_player (source of truth).
func LoadDTOInstanceFromPostgres(ctx context.Context, instanceID int64) (*dto.Instance, error) {
	if instanceID <= 0 {
		return nil, fmt.Errorf("instance_id must be positive, got %d", instanceID)
	}

	const qInstance = `
		SELECT
			hash,
			completed,
			flawless,
			fresh,
			player_count,
			date_started,
			date_completed,
			duration,
			platform_type,
			score,
			skull_hashes
		FROM core.instance
		WHERE instance_id = $1::bigint`

	var (
		hashRaw       int64
		completed     bool
		flawlessNull  sql.NullBool
		freshNull     sql.NullBool
		playerCount   int
		dateStarted   time.Time
		dateCompleted time.Time
		duration      int
		platformType  int
		score         int
		skullRaw      pq.Int64Array
	)

	err := postgres.DB.QueryRowContext(ctx, qInstance, instanceID).Scan(
		&hashRaw,
		&completed,
		&flawlessNull,
		&freshNull,
		&playerCount,
		&dateStarted,
		&dateCompleted,
		&duration,
		&platformType,
		&score,
		&skullRaw,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("core.instance not found for instance_id=%d", instanceID)
		}
		return nil, fmt.Errorf("load instance row: %w", err)
	}

	skullHashes := make([]uint32, len(skullRaw))
	for i, v := range skullRaw {
		skullHashes[i] = uint32(v)
	}

	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT membership_id, completed
		FROM core.instance_player
		WHERE instance_id = $1::bigint
		ORDER BY membership_id`, instanceID)
	if err != nil {
		return nil, fmt.Errorf("load instance players: %w", err)
	}
	defer rows.Close()

	var membershipIDs []int64
	var playersComplete []bool
	for rows.Next() {
		var mid int64
		var completed bool
		if err := rows.Scan(&mid, &completed); err != nil {
			return nil, err
		}
		membershipIDs = append(membershipIDs, mid)
		playersComplete = append(playersComplete, completed)
	}
	if err := rows.Err(); err != nil {
		return nil, err
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
		if i < len(playersComplete) && playersComplete[i] {
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

	var flawlessPtr *bool
	if flawlessNull.Valid {
		v := flawlessNull.Bool
		flawlessPtr = &v
	}
	var freshPtr *bool
	if freshNull.Valid {
		v := freshNull.Bool
		freshPtr = &v
	}

	return &dto.Instance{
		InstanceId:      instanceID,
		Hash:            uint32(hashRaw),
		Completed:       completed,
		Flawless:        flawlessPtr,
		Fresh:           freshPtr,
		PlayerCount:     playerCount,
		DateStarted:     dateStarted,
		DateCompleted:   dateCompleted,
		DurationSeconds: duration,
		MembershipType:  platformType,
		Score:           score,
		Players:         players,
		SkullHashes:     skullHashes,
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
