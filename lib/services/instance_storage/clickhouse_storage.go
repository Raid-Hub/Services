package instance_storage

import (
	"context"
	"raidhub/lib/database/clickhouse"
	"raidhub/lib/dto"
)

// StoreToClickHouse stores the instance data to ClickHouse
func StoreToClickHouse(inst *dto.Instance) error {
	// Use the ClickHouse connection from singleton
	conn := clickhouse.DB

	// Insert the instance data into ClickHouse
	ctx := context.Background()
	err := conn.Exec(ctx, "INSERT INTO instance (instance_id, hash, completed, date_started, date_completed, duration, player_count) VALUES (?, ?, ?, ?, ?, ?, ?)",
		inst.InstanceId, inst.Hash, inst.Completed, inst.DateStarted, inst.DateCompleted, inst.DurationSeconds, inst.PlayerCount)
	return err
}
