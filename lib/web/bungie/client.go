package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type BungieHttpResult[T any] struct {
	Success           bool
	Data              *T
	BungieErrorCode   int
	BungieErrorStatus string
}

func (r *BungieHttpResult[T]) FormatError(keys ...string) error {
	if len(keys) == 0 {
		return fmt.Errorf("%s (%d)", r.BungieErrorStatus, r.BungieErrorCode)
	}
	return fmt.Errorf("%s: %s (%d)", strings.Join(keys, " | "), r.BungieErrorStatus, r.BungieErrorCode)
}

type BungieClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func get[T any](c *BungieClient, url string) (*BungieHttpResult[T], int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, -1, err
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, -1, err
	}

	if resp.StatusCode != http.StatusOK {
		error_response, err := decodeResponse[BungieError](resp)
		if err != nil {
			return nil, resp.StatusCode, err
		}
		return &BungieHttpResult[T]{
			Success:           false,
			BungieErrorCode:   error_response.ErrorCode,
			BungieErrorStatus: error_response.ErrorStatus,
			Data:              nil,
		}, resp.StatusCode, nil
	}
	response, err := decodeResponse[BungieResponse[T]](resp)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return &BungieHttpResult[T]{
		Success:           true,
		BungieErrorCode:   response.ErrorCode,
		BungieErrorStatus: response.ErrorStatus,
		Data:              &response.Response,
	}, resp.StatusCode, nil
}

func decodeResponse[T any](resp *http.Response) (*T, error) {
	var data T
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *BungieClient) GetPGCR(instanceId int64) (*BungieHttpResult[DestinyPostGameCarnageReport], int, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/Stats/PostGameCarnageReport/%d/", c.baseURL, instanceId)
	return get[DestinyPostGameCarnageReport](c, url)
}

func (c *BungieClient) GetProfile(membershipType int, membershipId int64, components []int) (*BungieHttpResult[DestinyProfileResponse], int, error) {
	componentsStr := fmt.Sprintf("%d", components[0])
	for i := 1; i < len(components); i++ {
		componentsStr = fmt.Sprintf("%s,%d", componentsStr, components[i])
	}
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/?components=%s", c.baseURL, membershipType, membershipId, componentsStr)
	return get[DestinyProfileResponse](c, url)
}

func (c *BungieClient) GetCharacter(membershipType int32, membershipId int64, characterId int64) (*BungieHttpResult[DestinyCharacterResponse], int, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/Character/%d/?components=200", c.baseURL, membershipType, membershipId, characterId)
	return get[DestinyCharacterResponse](c, url)
}

func (c *BungieClient) GetDestinyManifest() (*BungieHttpResult[DestinyManifest], int, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/Manifest/", c.baseURL)
	return get[DestinyManifest](c, url)
}

func (c *BungieClient) GetHistoricalStats(membershipType int, membershipId int64) (*BungieHttpResult[DestinyHistoricalStatsAccountResult], int, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Account/%d/Stats/", c.baseURL, membershipType, membershipId)
	return get[DestinyHistoricalStatsAccountResult](c, url)
}

func (c *BungieClient) GetLinkedProfiles(membershipType int, membershipId int64, getAllMemberships bool) (*BungieHttpResult[LinkedProfiles], int, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/LinkedProfiles/?getAllMemberships=%t", c.baseURL, membershipType, membershipId, getAllMemberships)
	return get[LinkedProfiles](c, url)
}

func (c *BungieClient) GetGroup(groupId int64) (*BungieHttpResult[GroupResponse], int, error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/%d/", c.baseURL, groupId)
	return get[GroupResponse](c, url)
}

func (c *BungieClient) GetGroupsForMember(membershipType int, membershipId int64) (*BungieHttpResult[GetGroupsForMemberResponse], int, error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/User/%d/%d/0/1/", c.baseURL, membershipType, membershipId)
	return get[GetGroupsForMemberResponse](c, url)
}

func (c *BungieClient) GetMembersOfGroup(groupId int64, page int) (*BungieHttpResult[SearchResultOfGroupMember], int, error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/%d/Members/?currentpage=%d&memberType=0", c.baseURL, groupId, page)
	return get[SearchResultOfGroupMember](c, url)
}

func (c *BungieClient) GetCommonSettings() (*BungieHttpResult[CoreSettingsConfiguration], int, error) {
	url := fmt.Sprintf("%s/Platform/Settings/", c.baseURL)
	return get[CoreSettingsConfiguration](c, url)
}

func (c *BungieClient) GetActivityHistoryPage(membershipType int, membershipId int64, characterId int64, count int, page int, mode int) (*BungieHttpResult[DestinyActivityHistoryResults], int, error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Account/%d/Character/%d/Stats/Activities/?mode=%d&count=%d&page=%d", c.baseURL, membershipType, membershipId, characterId, mode, count, page)
	return get[DestinyActivityHistoryResults](c, url)
}
