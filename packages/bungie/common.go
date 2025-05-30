package bungie

import (
	"os"
	"sync"
	"time"
)

var (
	bungieUrlBase string
	once          sync.Once
)

type BungieError struct {
	ErrorCode       int    `json:"ErrorCode"`
	Message         string `json:"Message"`
	ErrorStatus     string `json:"ErrorStatus"`
	ThrottleSeconds int    `json:"ThrottleSeconds"`
}

type DestinyUserInfo struct {
	IconPath                    *string `json:"iconPath"`
	MembershipType              int     `json:"membershipType"`
	MembershipId                int64   `json:"membershipId,string"`
	DisplayName                 *string `json:"displayName"`
	BungieGlobalDisplayName     *string `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode *int    `json:"bungieGlobalDisplayNameCode"`
	ApplicableMembershipTypes   []int   `json:"applicableMembershipTypes"`
}

type DestinyHistoricalStatsActivity struct {
	InstanceId           int64  `json:"instanceId,string"`
	Mode                 int    `json:"mode"`
	Modes                []int  `json:"modes"`
	MembershipType       int    `json:"membershipType"`
	DirectorActivityHash uint32 `json:"directorActivityHash"`
}

type DestinyCharacterComponent struct {
	CharacterId    int64     `json:"characterId,string"`
	EmblemPath     string    `json:"emblemPath"`
	EmblemHash     uint32    `json:"emblemHash"`
	ClassHash      uint32    `json:"classHash"`
	DateLastPlayed time.Time `json:"dateLastPlayed,string"`
}

func getBungieURL() string {
	once.Do(func() {
		bungieUrlBase = os.Getenv("BUNGIE_URL_BASE")
		if bungieUrlBase == "" {
			bungieUrlBase = "https://www.bungie.net/"
		}
	})
	return bungieUrlBase
}
