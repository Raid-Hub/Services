package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type GetCharacterResponse struct {
	Response        DestinyCharacterResponse `json:"Response"`
	ErrorCode       int                      `json:"ErrorCode"`
	ErrorStatus     string                   `json:"ErrorStatus"`
	ThrottleSeconds int                      `json:"ThrottleSeconds"`
}

type DestinyCharacterResponse struct {
	Character *SingleComponentResponseOfDestinyCharacterComponent `json:"character"`
}

type SingleComponentResponseOfDestinyCharacterComponent struct {
	Data *DestinyCharacterComponent `json:"data"`
}

func GetCharacter(membershipType int32, membershipId int64, characterId int64) (*DestinyCharacterResponse, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/Character/%d/?components=200", getBungieURL(), membershipType, membershipId, characterId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	apiKey := os.Getenv("BUNGIE_API_KEY") // Read the API key from the BUNGIE_API_KEY environment variable
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var data BungieError
		if err := decoder.Decode(&data); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("error response: %s (%d)", data.Message, data.ErrorCode)
	}

	var data GetCharacterResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data.Response, nil
}
