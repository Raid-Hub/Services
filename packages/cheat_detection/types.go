package cheat_detection

import "time"

const (
	Manual uint64 = 1 << iota
	Leviathan
	EaterOfWorlds
	SpireOfStars
	LastWish
	ScourgeOfThePast
	CrownOfSorrow
	GardenOfSalvation
	DeepStoneCrypt
	VaultOfGlass
	VowOfTheDisciple
	KingsFall
	RootOfNightmares
	CrotasEnd
	SalvationsEdge
	Raid15
	Raid16
	Raid17
	Raid18
	Raid19
	Raid20
	Raid21
	Raid22
	Raid23
	Raid24
	Raid25
	Raid26
	Raid27
	Raid28
	Raid29
	Raid30
	Raid31
	Raid32
	Pantheon
	Bit34
	Bit35
	Bit36
	Bit37
	Bit38
	Bit39
	Bit40
	Bit41
	Bit42
	Bit43
	Bit44
	Bit45
	Bit46
	PlayerHeavyAmmoKills
	FastLowmanCheckpoint
	UnlikelyLowman
	PlayerKillsShare
	TimeDilation
	FirstClear
	Solo
	TotalInstanceKills
	TwoPlusCheaters
	PlayerTotalKills
	PlayerWeaponDiversity
	PlayerSuperKills
	PlayerGrenadeKills
	TooFast
	TooFewPlayersFresh
	TooFewPlayersCheckpoint
)

type ResultTuple struct {
	MembershipId int64
	Probability  float64
	Explanation  string
	Reason       uint64
}

type Instance struct {
	InstanceId       int64     `json:"instanceId"`
	Activity         int       `json:"activity"`
	Version          int       `json:"version"`
	RaidPath         string    `json:"raidPath"`
	Completed        bool      `json:"completed"`
	Flawless         *bool     `json:"flawless"`
	Fresh            *bool     `json:"fresh"`
	PlayerCount      int       `json:"playerCount"`
	DateStarted      time.Time `json:"dateStarted"`
	DateCompleted    time.Time `json:"dateCompleted"`
	DaysAfterRelease float64   `json:"daysAfterRelease"`
	Season           int       `json:"season"`
	DurationSeconds  int       `json:"duration"`
	MembershipType   int       `json:"membershipType"`
	Score            int       `json:"score"`
	Players          []Player  `json:"players"`
}

type Player struct {
	MembershipId      int64       `json:"membershipId"`
	Finished          bool        `json:"finished"`
	TimePlayedSeconds int         `json:"timePlayedSeconds"`
	IsFirstClear      bool        `json:"isFirstClear"`
	Sherpas           int         `json:"sherpas"`
	Characters        []Character `json:"characters"`
}

type Character struct {
	CharacterId       int64    `json:"characterId"`
	Class             *uint32  `json:"classHash"`
	EmblemHash        *uint32  `json:"emblemHash"`
	Completed         bool     `json:"completed"`
	Score             int      `json:"score"`
	Kills             int      `json:"kills"`
	Deaths            int      `json:"deaths"`
	Assists           int      `json:"assists"`
	PrecisionKills    int      `json:"precisionKills"`
	SuperKills        int      `json:"superKills"`
	GrenadeKills      int      `json:"grenadeKills"`
	MeleeKills        int      `json:"meleeKills"`
	StartSeconds      int      `json:"startSeconds"`
	TimePlayedSeconds int      `json:"timePlayedSeconds"`
	Weapons           []Weapon `json:"weapons"`
}

type Weapon struct {
	Kills          int    `json:"kills"`
	PrecisionKills int    `json:"precisionKills"`
	Name           string `json:"name"`
	WeaponType     string `json:"weaponType"`
	AmmoType       string `json:"ammoType"`
	Slot           string `json:"slot"`
	Element        string `json:"element"`
}

type PlayerInstanceFlagStats struct {
	MembershipId int64 `db:"membership_id"`
	FlaggedCount int   `db:"flagged_count"`
	FlagsA       int   `db:"flags_type_a"`
	FlagsB       int   `db:"flags_type_b"`
	FlagsC       int   `db:"flags_type_c"`
	FlagsD       int   `db:"flags_type_d"`
}
