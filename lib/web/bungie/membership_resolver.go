package bungie

import (
	"context"
	"fmt"
)

var membershipTypeAliases = map[string]int{
	"TigerXbox":   1, // TigerXbox = 1 (per Bungie API)
	"TigerPsn":    2, // TigerPsn = 2 (per Bungie API)
	"TigerSteam":  3, // TigerSteam = 3 (per Bungie API)
	"TigerStadia": 5, // TigerStadia = 5 (per Bungie API)
	"TigerEgs":    6, // TigerEgs = 6 (per Bungie API)
}

// ResolveProfile tries a known membership type first (if provided),
// then iterates common types until a valid profile is found.
// Returns the resolved membership type and the successful profile result.
func ResolveProfile(ctx context.Context, membershipId int64, knownType int) (int, BungieHttpResult[DestinyProfileResponse], error) {
	attempted := make(map[int]bool)
	var queue []int

	if knownType != 0 {
		queue = append(queue, knownType)
	}
	queue = append(queue, AllViableMembershipTypes...)

	var lastResult BungieHttpResult[DestinyProfileResponse]
	var lastErr error

	for len(queue) > 0 {
		mt := queue[0]
		queue = queue[1:]

		if attempted[mt] || mt == 0 {
			continue
		}
		attempted[mt] = true

		lastResult, lastErr = Client.GetProfile(ctx, mt, membershipId, []int{100, 200})
		if lastErr == nil && lastResult.Data != nil {
			return mt, lastResult, nil
		} else if lastResult.BungieErrorCode != InvalidParameters {
			break
		}

		// If Bungie tells us the real membership type in the error payload, prioritize it next.
		if hintedType := parseMembershipTypeHint(lastResult.MessageData); hintedType != 0 && !attempted[hintedType] {
			clientLogger.Info("MEMBERSHIP_TYPE_HINT_FOUND", map[string]any{
				"hinted_type":    hintedType,
				"membership_id":  membershipId,
				"attempted_type": mt,
				"message_data":   lastResult.MessageData,
			})
			queue = append([]int{hintedType}, queue...)
		}
	}

	return 0, lastResult, lastErr
}

func parseMembershipTypeHint(messageData map[string]any) int {
	if len(messageData) == 0 {
		return 0
	}

	if value, ok := messageData["membershipInfo.membershipType"]; ok {
		switch v := value.(type) {
		case string:
			if alias, ok := membershipTypeAliases[v]; ok {
				return alias
			}
		}

		clientLogger.Error(
			"IMPROPER_MEMBERSHIP_TYPE_MESSAGE_DATA",
			fmt.Errorf("improper membership type response: %v", messageData),
			map[string]any{
				"message_data": messageData,
			},
		)

	}
	return 0
}

func getMessageDataKeys(m map[string]any) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
