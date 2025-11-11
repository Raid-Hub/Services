package dto

import (
	"time"
)

type Instance struct {
	InstanceId      int64            `json:"instanceId"`
	Hash            uint32           `json:"hash"`
	Completed       bool             `json:"completed"`
	Flawless        *bool            `json:"flawless"`
	Fresh           *bool            `json:"fresh"`
	PlayerCount     int              `json:"playerCount"`
	DateStarted     time.Time        `json:"dateStarted"`
	DateCompleted   time.Time        `json:"dateCompleted"`
	DurationSeconds int              `json:"durationSeconds"`
	MembershipType  int              `json:"membershipType"`
	Score           int              `json:"score"`
	Players         []InstancePlayer `json:"players"`
	SkullHashes     []uint32         `json:"skullHashes"`
}

type InstancePlayer struct {
	Finished          bool                `json:"finished"`
	TimePlayedSeconds int                 `json:"timePlayedSeconds"`
	Player            PlayerInfo          `json:"player"`
	Characters        []InstanceCharacter `json:"characters"`
	IsFirstClear      bool                `json:"isFirstClear"` // Not set by default
	Sherpas           int                 `json:"sherpas"`      // Not set by default
}

type InstanceCharacter struct {
	CharacterId       int64                     `json:"characterId"`
	ClassHash         *uint32                   `json:"classHash"`
	EmblemHash        *uint32                   `json:"emblemHash"`
	Completed         bool                      `json:"completed"`
	Score             int                       `json:"score"`
	Kills             int                       `json:"kills"`
	Deaths            int                       `json:"deaths"`
	Assists           int                       `json:"assists"`
	PrecisionKills    int                       `json:"precisionKills"`
	SuperKills        int                       `json:"superKills"`
	GrenadeKills      int                       `json:"grenadeKills"`
	MeleeKills        int                       `json:"meleeKills"`
	StartSeconds      int                       `json:"startSeconds"`
	TimePlayedSeconds int                       `json:"timePlayedSeconds"`
	Weapons           []InstanceCharacterWeapon `json:"weapons"`
}

type InstanceCharacterWeapon struct {
	WeaponHash     uint32 `json:"weaponHash"`
	Kills          int    `json:"kills"`
	PrecisionKills int    `json:"precisionKills"`
}
