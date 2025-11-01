package bungie

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/network"
)

var clientLogger = logging.NewLogger("BUNGIE_CLIENT")

type BungieHttpResult[T any] struct {
	Success           bool
	Data              *T
	HttpStatusCode    int
	BungieErrorCode   int
	BungieErrorStatus string
}

type BungieClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func makeNonStandardHttpResult[T any](statusCode int) BungieHttpResult[T] {
	return BungieHttpResult[T]{
		Success:           false,
		HttpStatusCode:    statusCode,
		BungieErrorCode:   OtherError,
		BungieErrorStatus: "",
		Data:              nil,
	}
}

func get[T any](c *BungieClient, url string, operation string) (BungieHttpResult[T], error) {
	ctx := context.Background()
	config := network.DefaultRetryConfig()
	config.MaxAttempts = 2 // Quick retries for fast-fail
	config.InitialDelay = 50 * time.Millisecond
	
	return network.WithRetryForResult(ctx, config, func() (BungieHttpResult[T], error) {
		return getInternal[T](c, url, operation)
	})
}

func getInternal[T any](c *BungieClient, url string, operation string) (BungieHttpResult[T], error) {
	startTime := time.Now()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		clientLogger.Error("BUNGIE_REQUEST_CREATE_FAILED", map[string]any{
			logging.OPERATION: operation,
			logging.ENDPOINT:  url,
			logging.ERROR:     err.Error(),
		})
		return makeNonStandardHttpResult[T](0), err
	}
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime).Milliseconds()
	if err != nil {
		clientLogger.Debug("BUNGIE_REQUEST_FAILED", map[string]any{
			logging.OPERATION: operation,
			logging.ENDPOINT:  url,
			logging.ERROR:     err.Error(),
			logging.DURATION:  fmt.Sprintf("%dms", duration),
		})
		return makeNonStandardHttpResult[T](0), err
	}

	// first check if json header is present
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		clientLogger.Warn("BUNGIE_REQUEST_FAILED", map[string]any{
			logging.OPERATION:   operation,
			logging.ENDPOINT:    url,
			logging.ERROR:       "Content-Type is not application/json",
			logging.DURATION:    fmt.Sprintf("%dms", duration),
			logging.STATUS_CODE: resp.StatusCode,
		})
		return makeNonStandardHttpResult[T](resp.StatusCode), fmt.Errorf("content-type is not application/json")
	}

	if resp.StatusCode != http.StatusOK {
		error_response, err := decodeResponse[BungieError](resp)
		if err != nil {
			clientLogger.Error("BUNGIE_ERROR_DECODE_FAILED", map[string]any{
				logging.OPERATION:   operation,
				logging.ENDPOINT:    url,
				logging.STATUS_CODE: resp.StatusCode,
				logging.ERROR:       err.Error(),
				logging.DURATION:    fmt.Sprintf("%dms", duration),
			})
			return makeNonStandardHttpResult[T](resp.StatusCode), err
		}
		result := BungieHttpResult[T]{
			Success:           false,
			HttpStatusCode:    resp.StatusCode,
			BungieErrorCode:   error_response.ErrorCode,
			BungieErrorStatus: error_response.ErrorStatus,
			Data:              nil,
		}
		clientLogger.Debug("BUNGIE_REQUEST_ERROR", map[string]any{
			logging.OPERATION:     operation,
			logging.ENDPOINT:      url,
			logging.STATUS_CODE:   resp.StatusCode,
			"bungie_error_code":   error_response.ErrorCode,
			"bungie_error_status": error_response.ErrorStatus,
			logging.DURATION:      fmt.Sprintf("%dms", duration),
		})
		return result, error_response
	}
	response, err := decodeResponse[BungieResponse[T]](resp)
	if err != nil {
		clientLogger.Error("BUNGIE_RESPONSE_DECODE_FAILED", map[string]any{
			logging.OPERATION:   operation,
			logging.ENDPOINT:    url,
			logging.STATUS_CODE: resp.StatusCode,
			logging.ERROR:       err.Error(),
			logging.DURATION:    fmt.Sprintf("%dms", duration),
		})
		return makeNonStandardHttpResult[T](resp.StatusCode), err
	}

	success := response.ErrorCode == Success
	result := BungieHttpResult[T]{
		Success:           success,
		HttpStatusCode:    resp.StatusCode,
		BungieErrorCode:   response.ErrorCode,
		BungieErrorStatus: response.ErrorStatus,
		Data:              &response.Response,
	}

	clientLogger.Debug("BUNGIE_REQUEST_SUCCESS", map[string]any{
		logging.OPERATION:     operation,
		logging.ENDPOINT:      url,
		logging.STATUS_CODE:   resp.StatusCode,
		"bungie_error_code":   response.ErrorCode,
		"bungie_error_status": response.ErrorStatus,
		logging.DURATION:      fmt.Sprintf("%dms", duration),
	})

	return result, nil
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

func (c *BungieClient) GetPGCR(instanceId int64) (BungieHttpResult[DestinyPostGameCarnageReport], error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/Stats/PostGameCarnageReport/%d/", c.baseURL, instanceId)
	return get[DestinyPostGameCarnageReport](c, url, "get_pgcr")
}

func (c *BungieClient) GetProfile(membershipType int, membershipId int64, components []int) (BungieHttpResult[DestinyProfileResponse], error) {
	componentsStr := strings.Trim(strings.ReplaceAll(fmt.Sprintf("%v", components), " ", ","), "[]")
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/?components=%s", c.baseURL, membershipType, membershipId, componentsStr)
	return get[DestinyProfileResponse](c, url, "get_profile")
}

func (c *BungieClient) GetCharacter(membershipType int, membershipId int64, characterId int64) (BungieHttpResult[DestinyCharacterResponse], error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/Character/%d/?components=200", c.baseURL, membershipType, membershipId, characterId)
	return get[DestinyCharacterResponse](c, url, "get_character")
}

func (c *BungieClient) GetDestinyManifest() (BungieHttpResult[DestinyManifest], error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/Manifest/", c.baseURL)
	return get[DestinyManifest](c, url, "get_destiny_manifest")
}

func (c *BungieClient) GetHistoricalStats(membershipType int, membershipId int64) (BungieHttpResult[DestinyHistoricalStatsAccountResult], error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Account/%d/Stats/", c.baseURL, membershipType, membershipId)
	return get[DestinyHistoricalStatsAccountResult](c, url, "get_historical_stats")
}

func (c *BungieClient) GetLinkedProfiles(membershipType int, membershipId int64, getAllMemberships bool) (BungieHttpResult[LinkedProfiles], error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Profile/%d/LinkedProfiles/?getAllMemberships=%t", c.baseURL, membershipType, membershipId, getAllMemberships)
	return get[LinkedProfiles](c, url, "get_linked_profiles")
}

func (c *BungieClient) GetGroup(groupId int64) (BungieHttpResult[GroupResponse], error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/%d/", c.baseURL, groupId)
	return get[GroupResponse](c, url, "get_group")
}

func (c *BungieClient) GetGroupsForMember(membershipType int, membershipId int64) (BungieHttpResult[GetGroupsForMemberResponse], error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/User/%d/%d/0/1/", c.baseURL, membershipType, membershipId)
	return get[GetGroupsForMemberResponse](c, url, "get_groups_for_member")
}

func (c *BungieClient) GetMembersOfGroup(groupId int64, page int) (BungieHttpResult[SearchResultOfGroupMember], error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/%d/Members/?currentpage=%d&memberType=0", c.baseURL, groupId, page)
	return get[SearchResultOfGroupMember](c, url, "get_members_of_group")
}

func (c *BungieClient) GetCommonSettings() (BungieHttpResult[CoreSettingsConfiguration], error) {
	url := fmt.Sprintf("%s/Platform/Settings/", c.baseURL)
	return get[CoreSettingsConfiguration](c, url, "get_common_settings")
}

func (c *BungieClient) GetActivityHistoryPage(membershipType int, membershipId int64, characterId int64, count int, page int, mode int) (BungieHttpResult[DestinyActivityHistoryResults], error) {
	url := fmt.Sprintf("%s/Platform/Destiny2/%d/Account/%d/Character/%d/Stats/Activities/?mode=%d&count=%d&page=%d", c.baseURL, membershipType, membershipId, characterId, mode, count, page)
	return get[DestinyActivityHistoryResults](c, url, "get_activity_history_page")
}
