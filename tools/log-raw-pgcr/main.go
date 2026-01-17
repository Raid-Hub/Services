package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"strconv"
	"time"
)

var logger = logging.NewLogger("log-raw-pgcr")

func main() {
	logging.ParseFlags()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	// Parse the instance ID from command line args
	// Since logging.ParseFlags() is used, the actual arguments start from flag.Arg(0)
	if flag.NArg() < 1 {
		logger.Error("USAGE_ERROR", nil, map[string]any{"message": "Usage: scripts log-raw-pgcr <instance_id>"})
		return
	}

	instanceId, err := strconv.ParseInt(flag.Arg(0), 10, 64)
	if err != nil {
		logger.Error("INVALID_INSTANCE_ID", err, map[string]any{})
		return
	}

	logger.Info("FETCHING_RAW_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// Wait for PostgreSQL connection
	postgres.Wait()

	// Query raw.pgcr table
	var pgcrData []byte
	var dateCrawled time.Time
	err = postgres.DB.QueryRow(`SELECT data, date_crawled FROM raw.pgcr WHERE instance_id = $1`, instanceId).Scan(&pgcrData, &dateCrawled)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Warn("RAW_PGCR_NOT_FOUND", err, map[string]any{logging.INSTANCE_ID: instanceId})
		} else {
			logger.Error("RAW_PGCR_QUERY_ERROR", err, map[string]any{logging.INSTANCE_ID: instanceId})
		}
		return
	}

	// Decompress the gzip data
	var decompressedData []byte
	if len(pgcrData) >= 2 && pgcrData[0] == 0x1f && pgcrData[1] == 0x8b {
		// Data is gzip compressed
		reader, err := gzip.NewReader(bytes.NewReader(pgcrData))
		if err != nil {
			logger.Error("RAW_PGCR_DECOMPRESS_ERROR", err, map[string]any{logging.INSTANCE_ID: instanceId})
			return
		}
		defer reader.Close()
		decompressedData, err = io.ReadAll(reader)
		if err != nil {
			logger.Error("RAW_PGCR_READ_DECOMPRESSED_ERROR", err, map[string]any{logging.INSTANCE_ID: instanceId})
			return
		}
	} else {
		// Data is not compressed
		decompressedData = pgcrData
	}

	// Pretty print the JSON
	var pgcrJSON map[string]any
	if err := json.Unmarshal(decompressedData, &pgcrJSON); err != nil {
		logger.Error("RAW_PGCR_UNMARSHAL_ERROR", err, map[string]any{
			logging.INSTANCE_ID: instanceId,
			"raw_data":          string(decompressedData),
		})
		return
	}

	// Create output structure with timestamp and PGCR data
	output := map[string]any{
		"fetched_at":      dateCrawled.Format(time.RFC3339),
		"fetched_at_unix": dateCrawled.Unix(),
		"instance_id":     instanceId,
		"pgcr_data":       pgcrJSON,
	}

	// Marshal the output structure
	outputJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		logger.Error("RAW_PGCR_MARSHAL_ERROR", err, map[string]any{logging.INSTANCE_ID: instanceId})
		return
	}

	// Write to file
	filename := fmt.Sprintf("pgcr_%d.json", instanceId)
	err = os.WriteFile(filename, outputJSON, 0644)
	if err != nil {
		logger.Error("RAW_PGCR_WRITE_ERROR", err, map[string]any{
			logging.INSTANCE_ID: instanceId,
			"filename":          filename,
		})
		return
	}

	logger.Info("RAW_PGCR_SAVED", map[string]any{
		logging.INSTANCE_ID: instanceId,
		"filename":          filename,
		"fetched_at":        dateCrawled.Format(time.RFC3339),
	})

}
