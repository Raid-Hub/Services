package pgcr

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"raidhub/shared/bungie"
	"strings"
)

func StoreJSON(report *bungie.DestinyPostGameCarnageReport, db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO pgcr (instance_id, data)
		VALUES ($1, $2)
		ON CONFLICT (instance_id) DO NOTHING;`)
	if err != nil {
		return err
	}

	defer stmt.Close()

	// Marshal the struct to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		return err
	}

	// Compress the JSON data
	compressedData, err := gzipCompress(jsonData)
	if err != nil {
		return err
	}

	result, err := stmt.Exec(report.ActivityDetails.InstanceId, compressedData)
	if err != nil {
		return err
	}

	rowsAdded, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAdded == 0 {
		log.Printf("Duplicate raw PGCR: %d", report.ActivityDetails.InstanceId)
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func gzipCompress(data []byte) ([]byte, error) {
	var b strings.Builder
	w := gzip.NewWriter(&b)
	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func RetrieveJSON(instanceId int64, db *sql.DB) (*bungie.DestinyPostGameCarnageReport, error) {
	var compressedData []byte
	row := db.QueryRow(`SELECT data FROM pgcr WHERE instance_id = $1`, instanceId)

	err := row.Scan(&compressedData)
	if err != nil {
		return nil, err
	}

	// Decompress the JSON data
	decompressedJSON, err := GzipDecompress(compressedData)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON back to a struct
	var data bungie.DestinyPostGameCarnageReport
	err = json.Unmarshal(decompressedJSON, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func GzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	decompressedData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return decompressedData, nil
}
