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
// The players shape is []map[string]interface{} (nested: players[i].characters[j].weapons[k])
// so that clickhouse-go can serialize it to the Nested columns, matching the old pgcr_clickhouse
// format that used batch.Append(..., instance["players"]).
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

	players := buildPlayersNested(inst.Players)

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO instance")
	if err != nil {
		return err
	}
	defer batch.Abort()

	err = batch.Append(
		inst.InstanceId,
		inst.Hash,
		inst.Completed,
		inst.PlayerCount,
		fresh,
		flawless,
		inst.DateStarted,
		inst.DateCompleted,
		uint16(inst.MembershipType),
		inst.DurationSeconds,
		inst.Score,
		players,
	)
	if err != nil {
		return err
	}
	return batch.Send()
}

// buildPlayersNested builds the nested []map[string]interface{} shape that
// clickhouse-go serializes to the instance.players Nested columns.
func buildPlayersNested(pl []dto.InstancePlayer) []map[string]interface{} {
	players := make([]map[string]interface{}, 0, len(pl))
	for _, p := range pl {
		instancePlayer := map[string]interface{}{
			"membership_id":       p.Player.MembershipId,
			"completed":           p.Finished,
			"time_played_seconds": p.TimePlayedSeconds,
			"sherpas":             p.Sherpas,
			"is_first_clear":      p.IsFirstClear,
		}
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
				"completed":           c.Completed,
				"score":               c.Score,
				"kills":               c.Kills,
				"assists":             c.Assists,
				"deaths":              c.Deaths,
				"precision_kills":     c.PrecisionKills,
				"super_kills":         c.SuperKills,
				"grenade_kills":       c.GrenadeKills,
				"melee_kills":         c.MeleeKills,
				"time_played_seconds": c.TimePlayedSeconds,
				"start_seconds":       c.StartSeconds,
			}
			weapons := make([]map[string]interface{}, 0, len(c.Weapons))
			for _, w := range c.Weapons {
				weapons = append(weapons, map[string]interface{}{
					"weapon_hash":     w.WeaponHash,
					"kills":           w.Kills,
					"precision_kills": w.PrecisionKills,
				})
			}
			instanceCharacter["weapons"] = weapons
			characters = append(characters, instanceCharacter)
		}
		instancePlayer["characters"] = characters
		players = append(players, instancePlayer)
	}
	return players
}
