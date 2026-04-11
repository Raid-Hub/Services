package instance_storage

import (
	"context"

	"raidhub/lib/database/clickhouse"
	"raidhub/lib/dto"
)

// StoreToClickHouse stores the instance data to ClickHouse as one row per instance
// with the players Nested column populated. Materialized views (e.g. weapon_meta_by_hour_mv)
// read from instance and populate analytics tables.
//
// With flatten_nested=0 on the connection, clickhouse-go expects one []map per Nested column
// (Array(Tuple(...)) — see lib/column/nested.go and examples/clickhouse_api/nested.go NestedUnFlattened).
// Do not pass separate arrays per Nested field here; that only matches flattened mode and breaks
// batch.Append with "expected N arguments, got M".
func StoreToClickHouse(inst *dto.Instance) error {
	conn := clickhouse.DB
	ctx := context.Background()

	fresh := uint8(2) // 2 = unknown/nil, 1 = true, 0 = false
	if inst.Fresh != nil {
		if *inst.Fresh {
			fresh = 1
		} else {
			fresh = 0
		}
	}
	flawless := uint8(2)
	if inst.Flawless != nil {
		if *inst.Flawless {
			flawless = 1
		} else {
			flawless = 0
		}
	}

	players := buildPlayersMaps(inst.Players)

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO instance")
	if err != nil {
		return err
	}
	defer batch.Abort()

	err = batch.Append(
		inst.InstanceId,
		inst.Hash,
		boolToUInt8(inst.Completed),
		uint32(inst.PlayerCount),
		fresh,
		flawless,
		inst.DateStarted,
		inst.DateCompleted,
		uint16(inst.MembershipType),
		uint32(inst.DurationSeconds),
		int32(inst.Score),
		players,
	)
	if err != nil {
		return err
	}
	return batch.Send()
}

func boolToUInt8(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}

// buildPlayersMaps returns one map per player for the players Nested column (flatten_nested=0).
func buildPlayersMaps(pl []dto.InstancePlayer) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(pl))
	for _, p := range pl {
		characters := make([]map[string]interface{}, 0, len(p.Characters))
		for _, c := range p.Characters {
			classHash := uint32(0)
			if c.ClassHash != nil {
				classHash = *c.ClassHash
			}
			emblemHash := uint32(0)
			if c.EmblemHash != nil {
				emblemHash = *c.EmblemHash
			}
			instanceCharacter := map[string]interface{}{
				"character_id":        c.CharacterId,
				"class_hash":          classHash,
				"emblem_hash":         emblemHash,
				"completed":           boolToUInt8(c.Completed),
				"score":               int32(c.Score),
				"kills":               uint32(c.Kills),
				"assists":             uint32(c.Assists),
				"deaths":              uint32(c.Deaths),
				"precision_kills":     uint32(c.PrecisionKills),
				"super_kills":         uint32(c.SuperKills),
				"grenade_kills":       uint32(c.GrenadeKills),
				"melee_kills":         uint32(c.MeleeKills),
				"time_played_seconds": uint32(c.TimePlayedSeconds),
				"start_seconds":       uint32(c.StartSeconds),
			}
			weapons := make([]map[string]interface{}, 0, len(c.Weapons))
			for _, w := range c.Weapons {
				weapons = append(weapons, map[string]interface{}{
					"weapon_hash":     w.WeaponHash,
					"kills":           uint32(w.Kills),
					"precision_kills": uint32(w.PrecisionKills),
				})
			}
			instanceCharacter["weapons"] = weapons
			characters = append(characters, instanceCharacter)
		}
		out = append(out, map[string]interface{}{
			"membership_id":       p.Player.MembershipId,
			"completed":           boolToUInt8(p.Finished),
			"time_played_seconds": uint32(p.TimePlayedSeconds),
			"sherpas":             uint32(p.Sherpas),
			"is_first_clear":      boolToUInt8(p.IsFirstClear),
			"characters":          characters,
		})
	}
	return out
}
