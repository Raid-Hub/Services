package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type LinkedProfilesResponse struct {
	Response        LinkedProfiles `json:"Response"`
	ErrorCode       int            `json:"ErrorCode"`
	ErrorStatus     string         `json:"ErrorStatus"`
	ThrottleSeconds int            `json:"ThrottleSeconds"`
}

type LinkedProfiles struct {
	Profiles []DestinyUserInfo `json:"profiles"`
}

func GetLinkedProfiles(membershipType int, membershipId int64, getAllMemberships bool) ([]DestinyUserInfo, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/LinkedProfiles/?getAllMemberships=%t", getBungieURL(), membershipType, membershipId, getAllMemberships)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []DestinyUserInfo{}, err
	}

	apiKey := os.Getenv("BUNGIE_API_KEY") // Read the API key from the BUNGIE_API_KEY environment variable
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []DestinyUserInfo{}, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var data BungieError
		if err := decoder.Decode(&data); err != nil {
			return []DestinyUserInfo{}, err
		}

		return []DestinyUserInfo{}, fmt.Errorf("error response: %s (%d)", data.Message, data.ErrorCode)
	}

	var data LinkedProfilesResponse
	if err := decoder.Decode(&data); err != nil {
		return []DestinyUserInfo{}, err
	}

	return data.Response.Profiles, nil
}
