package player

import "time"

type Player struct {
	MembershipId                int64     `json:"membershipId"`
	MembershipType              *int      `json:"membershipType"`
	LastSeen                    time.Time `json:"lastSeen"`
	IconPath                    *string   `json:"iconPath"`
	DisplayName                 *string   `json:"displayName"`
	BungieGlobalDisplayName     *string   `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode *string   `json:"bungieGlobalDisplayNameCode"`
	FirstSeen                   time.Time `json:"firstSeen"` // Not set by default
	HistoryLastCrawled          time.Time `json:"historyLastCrawled"`
}

// Subset of Player, the fields that are used for most queries
type PlayerInfo struct {
	MembershipId                int64     `json:"membershipId"`
	MembershipType              *int      `json:"membershipType"`
	LastSeen                    time.Time `json:"lastSeen"`
	IconPath                    *string   `json:"iconPath"`
	DisplayName                 *string   `json:"displayName"`
	BungieGlobalDisplayName     *string   `json:"bungieGlobalDisplayName"`
	BungieGlobalDisplayNameCode *string   `json:"bungieGlobalDisplayNameCode"`
	FirstSeen                   time.Time `json:"firstSeen"` // Not set by default
}

// Character represents a character in the database
type Character struct {
	ID           int64
	MembershipId int64
	CharacterID  int64
}
