package clan

import (
	"encoding/json"
	"fmt"
	"html"
	"raidhub/shared/bungie"
	"strings"
)

func ParseClanDetails(group *bungie.GroupV2) ([]byte, string, string, string, error) {
	clanBannerData, err := json.Marshal(group.ClanInfo.ClanBannerData)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("error marshalling ClanBannerData: %v", err)
	}

	clanName := html.UnescapeString(strings.TrimSpace(strings.ReplaceAll(group.Name, "ㅤ", "")))
	callSign := html.UnescapeString(group.ClanInfo.ClanCallsign)
	motto := html.UnescapeString(group.Motto)

	return clanBannerData, clanName, callSign, motto, nil
}
