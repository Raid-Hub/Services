package postgres

import (
	"database/sql"
	"time"
)

func GetLatestInstanceId(db *sql.DB, buffer int64) (int64, error) {
	var latestID int64
	err := db.QueryRow(`SELECT instance_id FROM instance ORDER BY instance_id DESC LIMIT 1`).Scan(&latestID)
	if err != nil {
		return 0, err
	} else {
		return latestID - buffer, nil
	}
}

func GetLatestInstance(db *sql.DB) (int64, time.Time, error) {
	var latestID int64
	var dateCompleted time.Time
	err := db.QueryRow(`SELECT instance_id, date_completed FROM instance ORDER BY instance_id DESC LIMIT 1`).Scan(&latestID, &dateCompleted)
	if err != nil {
		return 0, time.Time{}, err
	} else {
		return latestID, dateCompleted, nil
	}
}
