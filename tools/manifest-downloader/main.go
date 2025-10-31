package manifestdownloader

import (
	"archive/zip"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var logger = logging.NewLogger("MANIFEST_DOWNLOADER")

// sanitizeVersionForFilename replaces filesystem-unsafe characters in version strings
func sanitizeVersionForFilename(version string) string {
	// Replace dots, dashes, and other special chars with underscores
	sanitized := strings.ReplaceAll(version, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, "-", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	return sanitized
}

// DownloadManifest is the command function for downloading and processing Destiny 2 manifest
// Usage: ./bin/tools manifest-downloader [--out=<directory>] [--force] [--disk]
func DownloadManifest() {
	fs := flag.NewFlagSet("manifest-downloader", flag.ExitOnError)
	out := fs.String("out", "", "where to store the sqlite (required)")
	force := fs.Bool("f", false, "force the defs to be updated")
	fromDisk := fs.Bool("disk", false, "read from disk, not bnet")

	// Parse flags after the command name
	fs.Parse(flag.Args())

	if *out == "" {
		logger.Fatal("MISSING_OUTPUT_DIRECTORY", map[string]any{"message": "must specify an artifacts output directory with the -out flag"})
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*out, os.ModePerm); err != nil {
		logger.Fatal("ERROR_CREATING_OUTPUT_DIRECTORY", map[string]any{logging.ERROR: err.Error()})
	}

	var sqlitePath string
	if !*fromDisk {
		result, err := bungie.Client.GetDestinyManifest()
		if err != nil {
			logger.Fatal("ERROR_GETTING_MANIFEST", map[string]any{logging.ERROR: err.Error()})
		}
		if !result.Success || result.Data == nil {
			logger.Fatal("MANIFEST_FETCH_FAILED", map[string]any{})
		}
		manifest := result.Data

		dbURL := fmt.Sprintf("https://www.bungie.net%s", manifest.MobileWorldContentPaths["en"])
		baseFileName := filepath.Base(dbURL)
		// Remove existing extension if present
		baseName := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
		// Remove "world_sql_content" prefix if present
		baseName = strings.TrimPrefix(baseName, "world_sql_content_")
		// Remove ".content" suffix if present
		baseName = strings.TrimSuffix(baseName, ".content")
		// Insert version at the start: Version_baseName
		versionSanitized := sanitizeVersionForFilename(manifest.Version)
		versionedFileName := fmt.Sprintf("%s_%s", versionSanitized, baseName)
		dbFileName := filepath.Join(*out, versionedFileName)
		sqlitePath = dbFileName + ".sqlite3" // name for the cached file

		if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
			logger.Info("LOADING_NEW_MANIFEST_DEFINITIONS", map[string]any{logging.VERSION: manifest.Version})
		} else if err != nil {
			logger.Fatal("ERROR_CHECKING_MANIFEST_FILE", map[string]any{logging.ERROR: err.Error()})
		} else {
			if !*force {
				logger.Info("NO_NEW_MANIFEST_DEFINITIONS", map[string]any{})
				return
			}
			logger.Info("RELOADING_EXISTING_MANIFEST_DEFINITIONS", map[string]any{logging.VERSION: manifest.Version})
		}

		// Download the ZIP file (use same base name as SQLite file)
		zipFileName := dbFileName + ".zip"
		resp, err := http.Get(fmt.Sprintf("%s?c=%d", dbURL, rand.Int()))
		if err != nil {
			logger.Fatal("ERROR_DOWNLOADING_MANIFEST", map[string]any{logging.ERROR: err.Error()})
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			logger.Fatal("INVALID_STATUS_CODE", map[string]any{logging.STATUS_CODE: resp.StatusCode})
		}
		logger.Debug("DOWNLOADED_FILES", map[string]any{})

		// Create the file to save the ZIP
		zipFile, err := os.Create(zipFileName)
		if err != nil {
			logger.Fatal("ERROR_CREATING_ZIP_FILE", map[string]any{
				logging.ERROR: err.Error(),
			})
		}
		logger.Debug("CREATED_ZIP_FILE", map[string]any{})
		defer func() {
			zipFile.Close()
			if err := os.Remove(zipFileName); err != nil {
				logger.Warn("ERROR_REMOVING_ZIP_FILE", map[string]any{logging.ERROR: err.Error()})
			} else {
				logger.Debug("REMOVED_ZIP_FILE", map[string]any{})
			}
		}()

		// Write the downloaded content to the ZIP file
		_, err = io.Copy(zipFile, resp.Body)
		if err != nil {
			logger.Fatal("ERROR_COPYING_RESPONSE_BODY", map[string]any{logging.ERROR: err.Error()})
		}
		zipFile.Close()

		// Extract the ZIP file
		zipReader, err := zip.OpenReader(zipFileName)
		if err != nil {
			logger.Fatal("ERROR_OPENING_ZIP_FILE", map[string]any{logging.ERROR: err.Error()})
		}
		logger.Debug("OPENED_ZIP_FILE", map[string]any{})
		defer zipReader.Close()

		// Extract each file from the ZIP archive
		var originalExtractedFile string
		for _, file := range zipReader.File {
			filePath := filepath.Join(*out, file.Name)
			if file.FileInfo().IsDir() {
				// Create directories
				err = os.MkdirAll(filePath, os.ModePerm)
				if err != nil {
					logger.Warn("ERROR_CREATING_DIRECTORY", map[string]any{logging.ERROR: err.Error()})
				}
				continue
			}

			// Track the original extracted file path
			originalExtractedFile = filePath

			// Create the file
			extractedFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				logger.Warn("ERROR_OPENING_EXTRACTED_FILE", map[string]any{logging.ERROR: err.Error()})
			}
			defer extractedFile.Close()

			// Extract the file
			zipFile, err := file.Open()
			if err != nil {
				logger.Warn("ERROR_OPENING_ZIP_FILE_ENTRY", map[string]any{logging.ERROR: err.Error()})
			}
			defer zipFile.Close()

			_, err = io.Copy(extractedFile, zipFile)
			if err != nil {
				logger.Warn("ERROR_EXTRACTING_FILE", map[string]any{logging.ERROR: err.Error()})
			}
		}
		logger.Debug("EXTRACTED_ZIP_FILE", map[string]any{})

		logger.Info("MANIFEST_DOWNLOADED", map[string]any{
			logging.FORMAT: "sqlite3",
			logging.STATUS: "success",
			logging.SOURCE: "bungie_api",
		})

		// Rename the extracted file to the versioned filename
		if originalExtractedFile != "" && originalExtractedFile != dbFileName {
			err = os.Rename(originalExtractedFile, dbFileName)
			if err != nil {
				logger.Warn("ERROR_RENAMING_TO_VERSIONED_FILE", map[string]any{logging.ERROR: err.Error()})
			}
		}

		// Rename the SQLite database file to have a recognizable extension
		err = os.Rename(dbFileName, sqlitePath)
		if err != nil {
			logger.Warn("ERROR_RENAMING_SQLITE_FILE", map[string]any{logging.ERROR: err.Error()})
		}
		logger.Debug("RENAMED_SQLITE3_FILE", map[string]any{})

		// Clean up the original extracted file if it still exists (shouldn't happen, but just in case)
		if originalExtractedFile != "" && originalExtractedFile != dbFileName && originalExtractedFile != sqlitePath {
			if err := os.Remove(originalExtractedFile); err != nil && !os.IsNotExist(err) {
				logger.Warn("ERROR_REMOVING_ORIGINAL_EXTRACTED_FILE", map[string]any{logging.ERROR: err.Error()})
			}
		}
	} else {
		var newestModTime time.Time

		err := filepath.Walk(*out, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil // skip directories
			}

			if info.ModTime().After(newestModTime) {
				newestModTime = info.ModTime()
				sqlitePath = path
			}

			return nil
		})

		if err != nil {
			logger.Warn("ERROR_WALKING_DIRECTORY", map[string]any{logging.ERROR: err.Error()})
		}

		if sqlitePath == "" {
			logger.Warn("DIRECTORY_IS_EMPTY", map[string]any{logging.DIRECTORY: *out})
		}
	}

	definitions, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		logger.Warn("ERROR_OPENING_SQLITE_DATABASE", map[string]any{logging.ERROR: err.Error()})
	}
	logger.Debug("CONNECTED_TO_SQLITE3", map[string]any{})
	defer definitions.Close()

	// Wait for PostgreSQL connection to be ready
	postgres.Wait()

	// postgres.DB is initialized in init()
	logger.Debug("CONNECTED_TO_POSTGRES", map[string]any{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(2)
	go saveWeaponDefinitions(ctx, &wg, postgres.DB, definitions)
	go saveFeatDefinitions(ctx, &wg, postgres.DB, definitions)

	wg.Wait()
}

func saveWeaponDefinitions(ctx context.Context, wg *sync.WaitGroup, db *sql.DB, definitions *sql.DB) {
	defer wg.Done()

	rows, err := definitions.QueryContext(ctx, `SELECT 
			json_extract(json, '$.hash'), 
			json_extract(json, '$.displayProperties.name'), 
			json_extract(json, '$.displayProperties.icon'), 
			json_extract(json, '$.defaultDamageType'), 
			json_extract(json, '$.equippingBlock.ammoType'), 
			json_extract(json, '$.equippingBlock.equipmentSlotTypeHash'), 
			json_extract(json, '$.itemSubType'),
			json_extract(json, '$.inventory.tierTypeName')
		FROM DestinyInventoryItemDefinition 
		WHERE json_extract(json, '$.itemType') = 3`)
	if err != nil {
		logger.Warn("ERROR_QUERYING_WEAPON_DEFINITIONS", map[string]any{logging.ERROR: err.Error()})
		return
	}
	defer rows.Close()
	logger.Debug("SCANNING_DEFINITIONS", map[string]any{})

	tx, err := postgres.DB.BeginTx(ctx, nil)
	if err != nil {
		logger.Warn("ERROR_BEGINNING_TRANSACTION", map[string]any{logging.ERROR: err.Error()})
		return
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO weapon_definition 
		(hash, name, icon_path, element, ammo_type, slot, weapon_type, rarity) 
		VALUES ($1::bigint, $2, $3, get_element($4), get_ammo_type($5), get_slot($6), get_weapon_type($7), $8)`)
	if err != nil {
		logger.Warn("ERROR_PREPARING_STATEMENT", map[string]any{logging.ERROR: err.Error()})
		return
	}
	defer stmt.Close()
	logger.Debug("PREPARED_POSTGRES_STATEMENT", map[string]any{})

	logger.Info("DATABASE_PREPARED", map[string]any{logging.OPERATION: "weapon_definition_upsert"})

	_, err = tx.ExecContext(ctx, "TRUNCATE TABLE weapon_definition")
	if err != nil {
		logger.Warn("ERROR_TRUNCATING_WEAPONS_TABLE", map[string]any{logging.ERROR: err.Error()})
	}
	logger.Debug("TRUNCATED_WEAPONS_TABLE", map[string]any{})

	// Iterate over the rows and process the data
	count := 0
	for rows.Next() {
		var hash uint32
		var name string
		var icon string
		var element uint8
		var ammoType uint8
		var slot uint32
		var weaponType string
		var rarity string
		if err := rows.Scan(&hash, &name, &icon, &element, &ammoType, &slot, &weaponType, &rarity); err != nil {
			logger.Warn("ERROR_SCANNING_WEAPON_ROW", map[string]any{logging.ERROR: err.Error()})
		}

		_, err := stmt.ExecContext(ctx, hash, name, icon, element, ammoType, slot, weaponType, rarity)
		if err != nil {
			logger.Warn("ERROR_INSERTING_WEAPON", map[string]any{logging.ERROR: err.Error()})
		}

		logger.Debug("INSERTED_WEAPON", map[string]any{logging.HASH: hash, logging.NAME: name})
		count++
	}

	err = tx.Commit()
	if err != nil {
		logger.Warn("ERROR_COMMITTING_TRANSACTION", map[string]any{logging.ERROR: err.Error()})
	}

	logger.Info("DEFINITIONS_SAVED", map[string]any{
		logging.TABLE:  "weapon_definition",
		logging.STATUS: "complete",
		logging.COUNT:  count,
	})
}

type FeatData struct {
	Hash                      uint32
	SkullIdentifierHash       uint32
	Name                      string
	Icon                      string
	Description               string
	DescriptionShort          string
	ModifierPowerContribution int
}

func saveFeatDefinitions(ctx context.Context, wg *sync.WaitGroup, db *sql.DB, definitions *sql.DB) {
	defer wg.Done()

	// first, get all raid hashes
	raidHashRows, err := postgres.DB.QueryContext(ctx, `SELECT hash FROM activity_version`)
	if err != nil {
		logger.Warn("ERROR_QUERYING_ACTIVITY_VERSIONS", map[string]any{logging.ERROR: err.Error()})
		return
	}
	defer raidHashRows.Close()
	var raidHashes []any
	for raidHashRows.Next() {
		var hash uint32
		if err := raidHashRows.Scan(&hash); err != nil {
			logger.Warn("ERROR_SCANNING_RAID_HASH", map[string]any{logging.ERROR: err.Error()})
			continue
		}
		raidHashes = append(raidHashes, hash)
	}

	if len(raidHashes) == 0 {
		logger.Warn("NO_ACTIVITY_VERSIONS_FOUND", map[string]any{})
		return
	}

	// Create a placeholder string for the IN clause
	placeholders := make([]string, len(raidHashes))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	// find the difficultyTierCollectionHash from the activity definition (or zero if not found)
	query := fmt.Sprintf(`SELECT 
			json_extract(selectable_skulls.value, '$.activitySkull.hash') AS hash,
			json_extract(selectable_skulls.value, '$.activitySkull.skullIdentifierHash') AS skull_identifier_hash,
			json_extract(selectable_skulls.value, '$.activitySkull.displayProperties.name') AS name,
			json_extract(selectable_skulls.value, '$.activitySkull.displayProperties.icon') AS icon,
			json_extract(selectable_skulls.value, '$.activitySkull.displayProperties.description') AS description,
			json_extract(selectable_skulls.value, '$.activitySkull.displayDescriptionOverrideForNavMode') AS description_short,
			json_extract(selectable_skulls.value, '$.activitySkull.modifierPowerContribution')
		FROM DestinyActivityDefinition AS a
		JOIN DestinyActivityDifficultyTierCollectionDefinition AS tier_collection
			ON json_extract(a.json, '$.difficultyTierCollectionHash') = json_extract(tier_collection.json, '$.hash')
		JOIN json_each(json_extract(tier_collection.json, '$.difficultyTiers')) AS tier
		JOIN json_each(tier.value, '$.selectableSkullCollectionHashes') AS skull_hashes
		JOIN DestinyActivitySelectableSkullCollectionDefinition AS skull
			ON json_extract(skull.json, '$.hash') = skull_hashes.value
		JOIN json_each(json_extract(skull.json, '$.selectableActivitySkulls')) AS selectable_skulls
		WHERE json_extract(a.json, '$.hash') IN (%s)
		 	AND json_extract(selectable_skulls.value, '$.activitySkull.dynamicUse') > 0
		GROUP BY hash`,
		strings.Join(placeholders, ","))

	rows, err := definitions.QueryContext(ctx, query, raidHashes...)
	if err != nil {
		logger.Warn("ERROR_QUERYING_FEAT_DEFINITIONS", map[string]any{logging.ERROR: err.Error()})
		return
	}
	defer rows.Close()

	// for each row, see if the difficultyTierCollectionHash is not null
	var feat FeatData
	for rows.Next() {
		// for now, just log the hash
		if err := rows.Scan(&feat.Hash, &feat.SkullIdentifierHash, &feat.Name, &feat.Icon, &feat.Description, &feat.DescriptionShort, &feat.ModifierPowerContribution); err != nil {
			logger.Warn("ERROR_SCANNING_FEAT_ROW", map[string]any{logging.ERROR: err.Error()})
		}

		logger.Debug("FOUND_SELECTABLE_SKULL", map[string]any{logging.NAME: feat.Name})

		// Insert the feat into the database
		_, err := postgres.DB.ExecContext(ctx, `INSERT INTO activity_feat_definition 
			(hash, skull_hash, name, icon_path, description, description_short, modifier_power_contribution) 
			VALUES ($1::bigint, $2::bigint, $3, $4, $5, $6, $7) 
			ON CONFLICT (hash) DO UPDATE SET
			name = EXCLUDED.name,
			icon_path = EXCLUDED.icon_path,
			description = EXCLUDED.description,
			description_short = EXCLUDED.description_short,
			modifier_power_contribution = EXCLUDED.modifier_power_contribution`,
			feat.Hash, feat.SkullIdentifierHash, feat.Name, feat.Icon, feat.Description, feat.DescriptionShort, feat.ModifierPowerContribution)
		if err != nil {
			logger.Warn("ERROR_INSERTING_FEAT_DEFINITION", map[string]any{logging.ERROR: err.Error()})
		}
	}
}
