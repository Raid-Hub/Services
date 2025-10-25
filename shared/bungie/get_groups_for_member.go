package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type GetGroupsResponse struct {
	Response        GetGroupsForMemberResponse `json:"Response"`
	ErrorCode       int                        `json:"ErrorCode"`
	ErrorStatus     string                     `json:"ErrorStatus"`
	ThrottleSeconds int                        `json:"ThrottleSeconds"`
}

type GetGroupsForMemberResponse struct {
	AreAllMembershipsInactive map[int64]bool    `json:"areAllMembershipsInactive"`
	Results                   []GroupMembership `json:"results"`
}

type GroupMembership struct {
	Member GroupMember `json:"member"`
	Group  GroupV2     `json:"group"`
}

type GroupMember struct {
	DestinyUserInfo        DestinyUserInfo `json:"destinyUserInfo"`
	LastOnlineStatusChange int64           `json:"lastOnlineStatusChange,string"`
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

func GetGroupsForMember(membershipType int, membershipId int64) (*GetGroupsForMemberResponse, error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/User/%d/%d/0/1/", getBungieURL(), membershipType, membershipId)
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

	var data GetGroupsResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data.Response, nil
}
