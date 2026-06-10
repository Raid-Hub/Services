package difficultytierloader

import (
	"context"
	"sync"

	"raidhub/lib/database/postgres"
	difficultytier "raidhub/lib/services/difficulty_tier"
)

var (
	featSkullsOnce sync.Once
	featSkulls     map[uint32]struct{}
	featSkullsErr  error
)

// Classify derives the MotT difficulty tier from PGCR skull identifier hashes.
func Classify(skulls []uint32) *string {
	return difficultytier.ClassifyWithFeatSkulls(skulls, featSkullSet())
}

func featSkullSet() map[uint32]struct{} {
	featSkullsOnce.Do(func() {
		rows, err := postgres.DB.QueryContext(context.Background(), `SELECT skull_hash FROM activity_feat_definition`)
		if err != nil {
			featSkullsErr = err
			return
		}
		defer rows.Close()

		featSkulls = make(map[uint32]struct{})
		for rows.Next() {
			var skullHash int64
			if err := rows.Scan(&skullHash); err != nil {
				featSkullsErr = err
				return
			}
			featSkulls[uint32(skullHash)] = struct{}{}
		}
		featSkullsErr = rows.Err()
	})

	if featSkullsErr != nil || featSkulls == nil {
		return map[uint32]struct{}{}
	}
	return featSkulls
}
