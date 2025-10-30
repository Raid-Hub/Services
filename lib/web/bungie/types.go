package bungie

import (
	"fmt"
	"time"
)

type BungieResponse[T any] struct {
	ErrorCode       int    `json:"ErrorCode"`
	Message         string `json:"Message"`
	ErrorStatus     string `json:"ErrorStatus"`
	ThrottleSeconds int    `json:"ThrottleSeconds"`
	Response        T      `json:"Response"`
}

type BungieError struct {
	ErrorCode       int    `json:"ErrorCode"`
	Message         string `json:"Message"`
	ErrorStatus     string `json:"ErrorStatus"`
	ThrottleSeconds int    `json:"ThrottleSeconds"`
}

func (b *BungieError) Error() string {
	return fmt.Sprintf("%s [%d]: %s", b.ErrorStatus, b.ErrorCode, b.Message)
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

type DestinyPostGameCarnageReport struct {
	ActivityDetails                 DestinyHistoricalStatsActivity      `json:"activityDetails"`
	ActivityDifficultyTier          *int                                `json:"activityDifficultyTier"`
	SelectedSkullHashes             *[]uint32                           `json:"selectedSkullHashes"`
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

type DestinyHistoricalWeaponStats struct {
	ReferenceId uint32                                 `json:"referenceId"`
	Values      map[string]DestinyHistoricalStatsValue `json:"values"`
}

type DestinyHistoricalStatsValue struct {
	Basic DestinyHistoricalStatsValuePair `json:"basic"`
}

type DestinyHistoricalStatsValuePair struct {
	Value        float32 `json:"value"`
	DisplayValue string  `json:"displayValue"`
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

type DestinyCharacterResponse struct {
	Character *SingleComponentResponseOfDestinyCharacterComponent `json:"character"`
}

type SingleComponentResponseOfDestinyCharacterComponent struct {
	Data *DestinyCharacterComponent `json:"data"`
}

type DestinyManifest struct {
	JsonWorldComponentContentPaths map[string]map[string]string `json:"jsonWorldComponentContentPaths"`
	JsonWorldContentPaths          map[string]string            `json:"jsonWorldContentPaths"`
	MobileWorldContentPaths        map[string]string            `json:"mobileWorldContentPaths"`
	Version                        string                       `json:"version"`
}

type DestinyHistoricalStatsAccountResult struct {
	Characters []DestinyHistoricalStatsPerCharacter `json:"characters"`
}

type DestinyHistoricalStatsPerCharacter struct {
	CharacterId int64 `json:"characterId,string"`
}

type DestinyActivityHistoryResults struct {
	Activities []DestinyHistoricalStatsPeriodGroup `json:"activities"`
}

type DestinyHistoricalStatsPeriodGroup struct {
	Period          string                         `json:"period"`
	ActivityDetails DestinyHistoricalStatsActivity `json:"activityDetails"`
}

type LinkedProfiles struct {
	Profiles []DestinyUserInfo `json:"profiles"`
}

type GroupResponse struct {
	Detail GroupV2 `json:"detail"`
}

type GroupV2 struct {
	GroupId     int64                        `json:"groupId,string"`
	Name        string                       `json:"name"`
	Motto       string                       `json:"motto"`
	MemberCount int                          `json:"memberCount"`
	ClanInfo    GroupV2ClanInfoAndInvestment `json:"clanInfo"`
	GroupType   int                          `json:"groupType"`
}

type GroupV2ClanInfoAndInvestment struct {
	ClanCallsign   string     `json:"clanCallsign"`
	ClanBannerData ClanBanner `json:"clanBannerData"`
}

type ClanBanner struct {
	DecalId                uint32 `json:"decalId"`
	DecalColorId           uint32 `json:"decalColorId"`
	DecalBackgroundColorId uint32 `json:"decalBackgroundColorId"`
	GonfalonId             uint32 `json:"gonfalonId"`
	GonfalonColorId        uint32 `json:"gonfalonColorId"`
	GonfalonDetailId       uint32 `json:"gonfalonDetailId"`
	GonfalonDetailColorId  uint32 `json:"gonfalonDetailColorId"`
}

type GroupMembership struct {
	Member GroupMember `json:"member"`
	Group  GroupV2     `json:"group"`
}

type GroupMember struct {
	DestinyUserInfo        DestinyUserInfo `json:"destinyUserInfo"`
	LastOnlineStatusChange int64           `json:"lastOnlineStatusChange,string"`
}

type GetGroupsForMemberResponse struct {
	AreAllMembershipsInactive map[int64]bool    `json:"areAllMembershipsInactive"`
	Results                   []GroupMembership `json:"results"`
}

type SearchResultOfGroupMember struct {
	HasMore bool          `json:"hasMore"`
	Results []GroupMember `json:"results"`
}

type CoreSettingsConfiguration struct {
	Systems map[string]CoreSystem `json:"systems"`
}

type CoreSystem struct {
	Enabled bool `json:"enabled"`
}
