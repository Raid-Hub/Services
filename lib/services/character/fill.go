package character

import (
	"raidhub/lib/utils"
	"raidhub/lib/web/bungie"
)

var CharacterLogger utils.Logger

func init() {
	CharacterLogger = utils.NewLogger("INSTANCE_CHARACTER_SERVICE")
}

// Fill fetches and fills missing character data
func Fill(membershipId int64, characterId int64) error {
	CharacterLogger.Info("Filling character data", "membershipId", membershipId, "characterId", characterId)

	// Get character from Bungie API
	result, _, err := bungie.Client.GetCharacter(2, membershipId, characterId)
	if err != nil {
		CharacterLogger.Error("Error fetching character", "membershipId", membershipId, "characterId", characterId, "error", err)
		return err
	}
	if result == nil || !result.Success || result.Data == nil {
		CharacterLogger.Warn("Character not found or no data", "membershipId", membershipId, "characterId", characterId)
		return nil
	}
	character := result.Data

	if character == nil || character.Character.Data == nil {
		CharacterLogger.Warn("Character not found or no data", "membershipId", membershipId, "characterId", characterId)
		return nil
	}

	CharacterLogger.Info("Successfully fetched character", "characterId", characterId)

	return nil
}
