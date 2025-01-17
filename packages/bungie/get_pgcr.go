package bungie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	bungieURL = "/Platform/Destiny2/Stats/PostGameCarnageReport"
)

func GetPGCR(client *http.Client, baseURL string, instanceId int64, apiKey string) (*json.Decoder, int, func(), error) {
	instanceUrl := fmt.Sprintf("%s%s/%d/", baseURL, bungieURL, instanceId)
	req, _ := http.NewRequest("GET", instanceUrl, nil)
	req.Header.Set("X-API-KEY", apiKey)

	// Create a context that will cancel the request if it takes longer than 60 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, -1, nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	return decoder, resp.StatusCode, func() {
		resp.Body.Close()
		cancel()
	}, nil

}

// There are more fields here than recorded in this file, but these are the only ones we care about
type DestinyPostGameCarnageReportResponse struct {
	Response        DestinyPostGameCarnageReport `json:"Response"`
	ErrorCode       int                          `json:"ErrorCode"`
	ErrorStatus     string                       `json:"ErrorStatus"`
	ThrottleSeconds int                          `json:"ThrottleSeconds"`
}

type DestinyPostGameCarnageReport struct {
	ActivityDetails                 DestinyHistoricalStatsActivity      `json:"activityDetails"`
	Period                          string                              `json:"period"`
	StartingPhaseIndex              int                                 `json:"startingPhaseIndex"`
	ActivityWasStartedFromBeginning bool                                `json:"activityWasStartedFromBeginning"`
	Entries                         []DestinyPostGameCarnageReportEntry `json:"entries"`
}

type DestinyPostGameCarnageReportEntry struct {
	Player      DestinyPostGameCarnageReportPlayer        `json:"player"`
	CharacterId int64                                     `json:"characterId,string"`
	Values      map[string]DestinyHistoricalStatsValue    `json:"values"`
	Extended    *DestinyPostGameCarnageReportExtendedData `json:"extended"`
	Score       DestinyHistoricalStatsValuePair           `json:"score"`
}

// type DestinyPostGameCarnageReportEntryValues struct {
// 	Assists                 *DestinyHistoricalStatsValue `json:"assists"`
// 	Completed               *DestinyHistoricalStatsValue `json:"completed"`
// 	Deaths                  *DestinyHistoricalStatsValue `json:"deaths"`
// 	Kills                   *DestinyHistoricalStatsValue `json:"kills"`
// 	Score                   *DestinyHistoricalStatsValue `json:"score"`
// 	ActivityDurationSeconds *DestinyHistoricalStatsValue `json:"activityDurationSeconds"`
// 	CompletionReason        *DestinyHistoricalStatsValue `json:"completionReason"`
// 	StartSeconds            *DestinyHistoricalStatsValue `json:"startSeconds"`
// 	TimePlayedSeconds       *DestinyHistoricalStatsValue `json:"timePlayedSeconds"`
// 	PlayerCount             *DestinyHistoricalStatsValue `json:"playerCount"`
// 	TeamScore               *DestinyHistoricalStatsValue `json:"teamScore"`
// }

type DestinyPostGameCarnageReportPlayer struct {
	DestinyUserInfo DestinyUserInfo `json:"destinyUserInfo"`
	ClassHash       uint32          `json:"classHash"`
	CharacterClass  *string         `json:"characterClass"`
	RaceHash        uint32          `json:"raceHash"`
	GenderHash      uint32          `json:"genderHash"`
	CharacterLevel  int             `json:"characterLevel"`
	LightLevel      int             `json:"lightLevel"`
	EmblemHash      uint32          `json:"emblemHash"`
}

type DestinyPostGameCarnageReportExtendedData struct {
	Values  map[string]DestinyHistoricalStatsValue `json:"values"`
	Weapons []DestinyHistoricalWeaponStats         `json:"weapons"`
}

// type DestinyPostGameCarnageReportExtendedDataValues struct {
// 	PrecisionKills     *DestinyHistoricalStatsValue `json:"precisionKills"`
// 	WeaponKillsSuper   *DestinyHistoricalStatsValue `json:"weaponKillsSuper"`
// 	WeaponKillsGrenade *DestinyHistoricalStatsValue `json:"weaponKillsGrenade"`
// 	WeaponKillsMelee   *DestinyHistoricalStatsValue `json:"weaponKillsMelee"`
// }

type DestinyHistoricalWeaponStats struct {
	ReferenceId uint32                                 `json:"referenceId"`
	Values      map[string]DestinyHistoricalStatsValue `json:"values"`
}

// type DestinyHistoricalWeaponStatsValues struct {
// 	UniqueWeaponKills          *DestinyHistoricalStatsValue `json:"uniqueWeaponKills"`
// 	UniqueWeaponPrecisionKills *DestinyHistoricalStatsValue `json:"uniqueWeaponPrecisionKills"`
// }

type DestinyHistoricalStatsValue struct {
	Basic DestinyHistoricalStatsValuePair `json:"basic"`
}

type DestinyHistoricalStatsValuePair struct {
	Value        float32 `json:"value"`
	DisplayValue string  `json:"displayValue"`
}
