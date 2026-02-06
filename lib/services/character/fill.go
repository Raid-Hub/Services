package character

import (
	"context"
	"errors"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

var logger = logging.NewLogger("CHARACTER_SERVICE")
var NO_PLAYER_CHARACTER_DATA = "NO_PLAYER_CHARACTER_DATA"

// Fill fetches and fills missing character data, returns true if the character was found and filled, false if the character was not found
func Fill(ctx context.Context, membershipId int64, characterId int64, instanceId int64) (bool, error) {
	logger.Debug("CHARACTER_FILL_STARTED", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.CHARACTER_ID:  characterId,
		logging.INSTANCE_ID:   instanceId,
	})

	// Get membership type - try common membership types to find the correct one
	// Try to resolve membership type from DB first
	var known int
	var membershipType int
	_ = postgres.DB.QueryRow("SELECT membership_type FROM player WHERE membership_id = $1", membershipId).Scan(&known)
	if known == 0 {
		// Resolve membership type using shared helper (tries known then all viable types)
		resolvedType, _, err := bungie.ResolveProfile(ctx, membershipId, 0)
		if err != nil || resolvedType == 0 {
			logger.Error("COULD_NOT_DETERMINE_CHARACTER_MEMBERSHIP_TYPE", err, map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.CHARACTER_ID:  characterId,
			})
			return false, err
		}
		membershipType = resolvedType
	} else {
		membershipType = known
	}

	// Get character from Bungie API
	result, err := bungie.Client.GetCharacter(ctx, membershipType, membershipId, characterId)
	if err != nil {
		fields := map[string]any{
			logging.MEMBERSHIP_ID:     membershipId,
			logging.CHARACTER_ID:      characterId,
			logging.BUNGIE_ERROR_CODE: result.BungieErrorCode,
			logging.STATUS_CODE:       result.HttpStatusCode,
		}

		if bungie.IsTransientError(result.BungieErrorCode, result.HttpStatusCode) {
			return false, err
		}

		var finalErr error
		switch result.BungieErrorCode {
		case bungie.CharacterNotFound:
			fields[logging.REASON] = "character_not_found"
			finalErr = nil
		case bungie.DestinyAccountNotFound:
			fields[logging.REASON] = "account_not_found"
			finalErr = nil
		default:
			if result.BungieErrorStatus == "" {
				fields[logging.REASON] = "unknown_error"
			} else {
				fields[logging.REASON] = result.BungieErrorStatus
			}
			logger.Warn("CHARACTER_FETCH_FAILED", err, fields)
			finalErr = processing.NewUnretryableError(err)
		}

		return false, finalErr

	}
	data := result.Data
	if data == nil {
		logger.Warn(NO_PLAYER_CHARACTER_DATA, errors.New("no data found in response"), map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.REASON:        "no_data",
		})
		return false, nil
	} else if data.Character == nil || data.Character.Data == nil {
		logger.Warn(NO_PLAYER_CHARACTER_DATA, errors.New("no character data found in response"), map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.REASON:        "no_character_data",
		})
		return false, nil
	}

	charData := data.Character.Data

	// Update instance_character table with missing data
	if err := updateInstanceCharacter(instanceId, membershipId, characterId, charData.ClassHash, charData.EmblemHash); err != nil {
		logger.Error("CHARACTER_UPDATE_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.CHARACTER_ID:  characterId,
			logging.INSTANCE_ID:   instanceId,
			"class_hash":          charData.ClassHash,
			"emblem_hash":         charData.EmblemHash,
		})
		return false, err
	}

	logger.Debug("CHARACTER_FILL_COMPLETE", map[string]any{
		logging.CHARACTER_ID: characterId,
		logging.INSTANCE_ID:  instanceId,
		logging.STATUS:       "success",
	})

	return true, nil
}

func updateInstanceCharacter(instanceId int64, membershipId int64, characterId int64, classHash uint32, emblemHash uint32) error {
	_, err := postgres.DB.Exec(`
		UPDATE instance_character
		SET class_hash = COALESCE(class_hash, $1),
		    emblem_hash = COALESCE(emblem_hash, $2)
		WHERE instance_id = $3
		  AND membership_id = $4
		  AND character_id = $5
	`, classHash, emblemHash, instanceId, membershipId, characterId)
	return err
}
