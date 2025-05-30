package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type GetProfilesResponse struct {
	Response        DestinyProfileResponse `json:"Response"`
	ErrorCode       int                    `json:"ErrorCode"`
	ErrorStatus     string                 `json:"ErrorStatus"`
	ThrottleSeconds int                    `json:"ThrottleSeconds"`
}

type DestinyProfileResponse struct {
	Profile    SingleComponentResponseOfDestinyProfileComponent               `json:"profile"`
	Characters DictionaryComponentResponseOfint64AndDestinyCharacterComponent `json:"characters"`
}

type SingleComponentResponseOfDestinyProfileComponent struct {
	Data *DestinyProfileComponent `json:"data"`
}

type DestinyProfileComponent struct {
	UserInfo                    DestinyUserInfo `json:"userInfo"`
	DateLastPlayed              time.Time       `json:"dateLastPlayed"`
	VersionsOwned               int64           `json:"versionsOwned"`
	CharacterIds                []string        `json:"characterIds"`
	SeasonHashes                []uint32        `json:"seasonHashes"`
	EventCardHashesOwned        []uint32        `json:"eventCardHashesOwned"`
	CurrentGuardianRank         int             `json:"currentGuardianRank"`
	LifetimeHighestGuardianRank int             `json:"lifetimeHighestGuardianRank"`
	RenewedGuardianRank         int             `json:"renewedGuardianRank"`
}

type DictionaryComponentResponseOfint64AndDestinyCharacterComponent struct {
	Data *map[int64]DestinyCharacterComponent `json:"data"`
}

func GetProfile(membershipType int, membershipId int64, components []int) (*DestinyProfileResponse, error) {
	// turn componets into a comma separated string
	componentsStr := ""
	for i, component := range components {
		if i == 0 {
			componentsStr = fmt.Sprintf("%d", component)
		} else {
			componentsStr = fmt.Sprintf("%s,%d", componentsStr, component)
		}
	}
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/?components=%s", getBungieURL(), membershipType, membershipId, componentsStr)
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

	var data GetProfilesResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data.Response, nil
}
