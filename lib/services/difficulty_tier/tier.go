package difficultytier

const (
	TierAdventure = "adventure"
	TierStandard  = "standard"
	TierCustom    = "custom"

	skullAdventureTimeOnYourSide = uint32(845104503)
	skullAdventureFixed2         = uint32(2008962334)
	skullEmptyFeat               = uint32(790421403)
)

// ClassifyWithFeatSkulls classifies a PGCR from its skull hashes and known feat skulls.
// Returns nil when the instance does not appear to use a difficulty tier collection.
func ClassifyWithFeatSkulls(skulls []uint32, featSkulls map[uint32]struct{}) *string {
	hasAdventure := false
	hasEmptyFeat := false
	hasFeat := false

	for _, skull := range skulls {
		switch skull {
		case skullAdventureTimeOnYourSide, skullAdventureFixed2:
			hasAdventure = true
		case skullEmptyFeat:
			hasEmptyFeat = true
		default:
			if _, ok := featSkulls[skull]; ok {
				hasFeat = true
			}
		}
	}

	if !hasAdventure && !hasEmptyFeat && !hasFeat {
		return nil
	}

	var tier string
	switch {
	case hasAdventure:
		tier = TierAdventure
	case hasFeat:
		tier = TierCustom
	default:
		tier = TierStandard
	}
	return &tier
}
