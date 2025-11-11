package character

import (
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

var logger = logging.NewLogger("CHARACTER_SERVICE")
var CHARACTER_NOT_FOUND = "CHARACTER_NOT_FOUND"

// Fill fetches and fills missing character data
func Fill(membershipId int64, characterId int64, instanceId int64) error {
	logger.Info("CHARACTER_FILL_STARTED", map[string]any{
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
		resolvedType, _, err := bungie.ResolveProfile(membershipId, nil)
		if err != nil || resolvedType == 0 {
			// fail here
			logger.Error("COULD_NOT_DETERMINE_MEMBERSHIP_TYPE", err, map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.CHARACTER_ID:  characterId,
			})
			return err
		}
		membershipType = resolvedType
	} else {
		membershipType = known
	}

	// Get character from Bungie API
	result, err := bungie.Client.GetCharacter(membershipType, membershipId, characterId)
	if err != nil {
		if result.BungieErrorCode == bungie.CharacterNotFound || result.HttpStatusCode == 404 {
			logger.Warn(CHARACTER_NOT_FOUND, nil, map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.CHARACTER_ID:  characterId,
				logging.REASON:        "character_not_found",
			})
			return nil
		}
		logger.Error("CHARACTER_FETCH_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.CHARACTER_ID:  characterId,
		})
		return err
	}
	if !result.Success || result.Data == nil {
		if result.BungieErrorCode == bungie.CharacterNotFound || result.HttpStatusCode == 404 {
			logger.Warn(CHARACTER_NOT_FOUND, nil, map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.CHARACTER_ID:  characterId,
				logging.REASON:        "character_not_found_bungie_error",
			})
			return nil
		}
		logger.Warn(CHARACTER_NOT_FOUND, nil, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.CHARACTER_ID:  characterId,
			logging.REASON:        "no_api_data",
		})
		return nil
	}
	character := result.Data

	if character == nil || character.Character.Data == nil {
		logger.Warn(CHARACTER_NOT_FOUND, nil, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.CHARACTER_ID:  characterId,
			logging.REASON:        "no_character_data",
		})
		return nil
	}

	charData := character.Character.Data

	// Update instance_character table with missing data
	_, err = postgres.DB.Exec(`
		UPDATE instance_character
		SET class_hash = COALESCE(class_hash, $1),
		    emblem_hash = COALESCE(emblem_hash, $2)
		WHERE instance_id = $3
		  AND membership_id = $4
		  AND character_id = $5
	`, charData.ClassHash, charData.EmblemHash, instanceId, membershipId, characterId)
	if err != nil {
		logger.Error("CHARACTER_UPDATE_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.CHARACTER_ID:  characterId,
			logging.INSTANCE_ID:   instanceId,
		})
		return err
	}

	logger.Debug("CHARACTER_FILL_COMPLETE", map[string]any{
		logging.CHARACTER_ID: characterId,
		logging.INSTANCE_ID:  instanceId,
		logging.STATUS:       "success",
	})

	return nil
}
