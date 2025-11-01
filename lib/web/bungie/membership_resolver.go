package bungie

// ResolveProfile tries a known membership type first (if provided),
// then iterates common types until a valid profile is found.
// Returns the resolved membership type and the successful profile result.
func ResolveProfile(membershipId int64, knownType *int) (int, BungieHttpResult[DestinyProfileResponse], error) {
	var result BungieHttpResult[DestinyProfileResponse]
	var err error

	// If we have a known membership type, try it first
	if knownType != nil {
		result, err = Client.GetProfile(*knownType, membershipId, []int{100, 200})
		if err == nil && result.Success && result.Data != nil {
			return *knownType, result, nil
		}
	}

	// Try common membership types
	for _, mt := range AllViableMembershipTypes {
		if knownType != nil && mt == *knownType {
			continue
		}
		result, err = Client.GetProfile(mt, membershipId, []int{100, 200})
		if err == nil && result.Success && result.Data != nil {
			return mt, result, nil
		}
		if err == nil && result.BungieErrorCode != InvalidParameters {
			// Stop trying other types if the error isn't InvalidParameters
			break
		}
	}

	return 0, result, err
}
