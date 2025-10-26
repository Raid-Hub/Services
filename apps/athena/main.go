package main

import (
	"archive/zip"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"raidhub/lib/database/postgres"
	"raidhub/lib/web/bungie"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var (
	out      = flag.String("dir", "./", "where to store the sqlite")
	force    = flag.Bool("f", false, "force the defs to be updated")
	verbose  = flag.Bool("verbose", false, "log more")
	fromDisk = flag.Bool("disk", false, "read from disk, not bnet")
)

func main() {
	flag.Parse()
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	var sqlitePath string
	if !*fromDisk {
		result, _, err := bungie.Client.GetDestinyManifest()
		if err != nil {
			log.Fatal("get manifest: ", err)
		}
		if result == nil || !result.Success || result.Data == nil {
			log.Fatal("manifest fetch failed")
		}
		manifest := result.Data

		dbURL := fmt.Sprintf("https://www.bungie.net%s", manifest.MobileWorldContentPaths["en"])
		dbFileName := filepath.Join(*out, filepath.Base(dbURL))
		sqlitePath = dbFileName + ".sqlite3" // name for the cached file

		if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
			log.Printf("Loading new manifest definitions: %s", manifest.Version)
		} else if err != nil {
			log.Fatal(err)
		} else {
			log.Printf("No new manifest definitions")
			if !*force {
				return
			}
		}

		// Download the ZIP file
		zipFileName := dbFileName + ".zip"
		resp, err := http.Get(fmt.Sprintf("%s?c=%d", dbURL, rand.Int()))
		if resp.StatusCode != 200 {
			log.Fatal(fmt.Errorf("invalid status code: %d", resp.StatusCode))
		}
		if err != nil {
			log.Fatal(err)
		}
		if *verbose {
			log.Println("Downloaded files")
		}
		defer resp.Body.Close()

		// Create the file to save the ZIP
		zipFile, err := os.Create(zipFileName)
		if err != nil {
			log.Fatal(err)
		}
		if *verbose {
			log.Println("Created zip file")
		}
		defer func() {
			zipFile.Close()
			err = os.Remove(zipFileName)
			if err != nil {
				log.Fatal(err)
			}
			if *verbose {
				log.Println("Removed zip file")
			}
		}()

		// Write the downloaded content to the ZIP file
		_, err = io.Copy(zipFile, resp.Body)
		if err != nil {
			log.Fatal(err)
		}

		// Extract the ZIP file
		zipReader, err := zip.OpenReader(zipFileName)
		if err != nil {
			log.Fatal(err)
		}
		if *verbose {
			log.Println("Opened zip file")
		}
		defer zipReader.Close()

		// Extract each file from the ZIP archive
		for _, file := range zipReader.File {
			filePath := filepath.Join(*out, file.Name)
			if file.FileInfo().IsDir() {
				// Create directories
				err = os.MkdirAll(filePath, os.ModePerm)
				if err != nil {
					log.Fatal(err)
				}
				continue
			}

			// Create the file
			extractedFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				log.Fatal(err)
			}
			defer extractedFile.Close()

			// Extract the file
			zipFile, err := file.Open()
			if err != nil {
				log.Fatal(err)
			}
			defer zipFile.Close()

			_, err = io.Copy(extractedFile, zipFile)
			if err != nil {
				log.Fatal(err)
			}
		}
		if *verbose {
			log.Println("Extracted zip file")
		}

		log.Println("Downloaded sqlite3 successfully")

		// Rename the SQLite database file to have a recognizable extension
		err = os.Rename(dbFileName, sqlitePath)
		if err != nil {
			log.Fatal(err)
		}
		if *verbose {
			log.Println("Remame sqlite3 file")
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
			log.Fatal(err)
		}

		if sqlitePath == "" {
			log.Fatalf("directory %s is empty", *out)
		}
	}

	definitions, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		log.Fatal(err)
	}
	if *verbose {
		log.Println("Connected to sqlite3")
	}
	defer definitions.Close()

	// postgres.DB is initialized in init()
	if *verbose {
		log.Println("Connected to postgres")
	}

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
		log.Fatal(err)
	}
	if *verbose {
		log.Println("Scanning definitions")
	}
	defer rows.Close()

	tx, err := postgres.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO weapon_definition 
		(hash, name, icon_path, element, ammo_type, slot, weapon_type, rarity) 
		VALUES ($1::bigint, $2, $3, get_element($4), get_ammo_type($5), get_slot($6), get_weapon_type($7), $8)`)
	if err != nil {
		log.Fatal(err)
	}
	if *verbose {
		log.Println("Prepared postgres statement")
	}
	defer stmt.Close()

	log.Println("Statement prepared")

	_, err = tx.ExecContext(ctx, "TRUNCATE TABLE weapon_definition")
	if err != nil {
		log.Fatal(err)
	}
	if *verbose {
		log.Println("Truncated weapons table")
	}

	// Iterate over the rows and process the data
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
			log.Fatal(err)
		}

		_, err := stmt.ExecContext(ctx, hash, name, icon, element, ammoType, slot, weaponType, rarity)
		if err != nil {
			log.Fatal(err)
		}

		if *verbose {
			log.Printf("Inserted %d: %s", hash, name)
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Saved weapon definitions")
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
		log.Fatal(err)
	}
	defer raidHashRows.Close()
	var raidHashes []any
	for raidHashRows.Next() {
		var hash uint32
		if err := raidHashRows.Scan(&hash); err != nil {
			log.Fatal(err)
		}
		raidHashes = append(raidHashes, hash)
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
		log.Fatal(err)
	}
	defer rows.Close()

	// for each row, see if the difficultyTierCollectionHash is not null
	var feat FeatData
	for rows.Next() {
		// for now, just log the hash
		if err := rows.Scan(&feat.Hash, &feat.SkullIdentifierHash, &feat.Name, &feat.Icon, &feat.Description, &feat.DescriptionShort, &feat.ModifierPowerContribution); err != nil {
			log.Fatal(err)
		}

		if *verbose {
			log.Printf("Found selectable skull: %s", feat.Name)
		}

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
			log.Fatal(err)
		}
	}
}
