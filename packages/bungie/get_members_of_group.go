package bungie

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type GetMembersOfGroupResponse struct {
	Response        SearchResultOfGroupMember `json:"Response"`
	ErrorCode       int                       `json:"ErrorCode"`
	ErrorStatus     string                    `json:"ErrorStatus"`
	ThrottleSeconds int                       `json:"ThrottleSeconds"`
}

type SearchResultOfGroupMember struct {
	HasMore bool          `json:"hasMore"`
	Results []GroupMember `json:"results"`
}

func GetMembersOfGroup(groupId int64, page int) (*SearchResultOfGroupMember, int, error) {
	url := fmt.Sprintf("%s/Platform/GroupV2/%d/Members/?currentpage=%d&memberType=0", getBungieURL(), groupId, page)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, -1, err
	}

	apiKey := os.Getenv("BUNGIE_API_KEY") // Read the API key from the BUNGIE_API_KEY environment variable
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, -1, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var data BungieError
		if err := decoder.Decode(&data); err != nil {
			return nil, data.ErrorCode, err
		}

		defer func() {
			if data.ThrottleSeconds > 0 {
				time.Sleep(time.Duration(data.ThrottleSeconds) * time.Second)
			}
		}()

		return nil, data.ErrorCode, fmt.Errorf("error response: %s (%d)", data.Message, data.ErrorCode)
	}

	var data GetMembersOfGroupResponse
	if err := decoder.Decode(&data); err != nil {
		return nil, data.ErrorCode, err
	}

	return &data.Response, data.ErrorCode, nil
}
