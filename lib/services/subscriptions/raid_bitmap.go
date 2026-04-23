package subscriptions

import (
	"context"
	"fmt"
	"sync"

	"raidhub/lib/database/postgres"
)

// subscriptionRaidBitForActivityID returns the raid bitmap bit for definitions.activity_definition.id.
// Layout matches lib/services/cheat_detection raid iota (1<<n for n = activity id for raids 1–16; Pantheon id 101 → 1<<33).
func subscriptionRaidBitForActivityID(activityID int64) (uint64, bool) {
	switch {
	case activityID >= 1 && activityID <= 32:
		return uint64(1) << uint(activityID), true
	case activityID == 101:
		return uint64(1) << 33, true
	default:
		return 0, false
	}
}

var (
	raidHashBitMu     sync.Mutex
	raidHashToBit     map[uint32]uint64
	raidHashBitsReady bool
)

func loadRaidHashBits(ctx context.Context) error {
	raidHashBitMu.Lock()
	defer raidHashBitMu.Unlock()
	if raidHashBitsReady {
		return nil
	}
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT av.hash, ad.id
		FROM definitions.activity_version av
		INNER JOIN definitions.activity_definition ad ON ad.id = av.activity_id AND ad.is_raid = true`)
	if err != nil {
		return fmt.Errorf("load raid activity_version hashes: %w", err)
	}
	defer rows.Close()

	m := make(map[uint32]uint64)
	for rows.Next() {
		var hash int64
		var activityID int64
		if err := rows.Scan(&hash, &activityID); err != nil {
			return err
		}
		bit, ok := subscriptionRaidBitForActivityID(activityID)
		if !ok || bit == 0 {
			continue
		}
		h := uint32(hash)
		m[h] = bit
	}
	if err := rows.Err(); err != nil {
		return err
	}
	raidHashToBit = m
	raidHashBitsReady = true
	return nil
}

func raidBitForActivityHash(ctx context.Context, activityHash uint32) (uint64, error) {
	if err := loadRaidHashBits(ctx); err != nil {
		return 0, err
	}
	return raidHashToBit[activityHash], nil
}

// ruleMatchesRaidBitmap enforces activity_raid_bitmap (column is always non-null; 0 means no raid filter).
func ruleMatchesRaidBitmap(ctx context.Context, activityHash uint32, ruleBitmap uint64) (bool, error) {
	if ruleBitmap == 0 {
		return true, nil
	}
	bit, err := raidBitForActivityHash(ctx, activityHash)
	if err != nil {
		return false, err
	}
	if bit == 0 {
		return false, nil
	}
	return (ruleBitmap & bit) != 0, nil
}
