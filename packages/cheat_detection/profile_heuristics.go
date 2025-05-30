package cheat_detection

import (
	"math"
	"raidhub/packages/bungie"
)

const (
	AccountFlagAge uint64 = 1 << iota
	AccountFlagStarClears
	AccountFlagClears
	AccountFlagPlatform
	AccountFlagCharacters
	AccountFlagDLCs
	AccountFlagGuardianRank
	AccountFlagPrivateProfile
)

func GetCheaterAccountFlagsStrings(flags uint64) []string {
	var flagStrings []string
	if flags&AccountFlagAge != 0 {
		flagStrings = append(flagStrings, "Age")
	}
	if flags&AccountFlagStarClears != 0 {
		flagStrings = append(flagStrings, "StarClears")
	}
	if flags&AccountFlagClears != 0 {
		flagStrings = append(flagStrings, "Clears")
	}
	if flags&AccountFlagPlatform != 0 {
		flagStrings = append(flagStrings, "Platform")
	}
	if flags&AccountFlagCharacters != 0 {
		flagStrings = append(flagStrings, "Characters")
	}
	if flags&AccountFlagDLCs != 0 {
		flagStrings = append(flagStrings, "DLCs")
	}
	if flags&AccountFlagGuardianRank != 0 {
		flagStrings = append(flagStrings, "GuardianRank")
	}
	if flags&AccountFlagPrivateProfile != 0 {
		flagStrings = append(flagStrings, "PrivateProfile")
	}
	return flagStrings
}

// perform some heuristics to determine if the player is a cheater based on history of their account,
// should return the chance of a player being a cheater based on their profile
func GetCheaterAccountChance(profile *bungie.DestinyProfileComponent, clears int, ageInDays float64, flawlessRatio float64, lowmanRatio float64, soloRatio float64, isPrivate bool) (float64, uint64) {
	flags := uint64(0)
	var ageFactor float64 = 0
	if ageInDays < 7 {
		ageFactor = 0.4
		flags |= AccountFlagAge
	} else if ageInDays < 30 {
		ageFactor = 0.3
		flags |= AccountFlagAge
	} else if ageInDays < 90 {
		ageFactor = 0.2
		flags |= AccountFlagAge
	} else if ageInDays < 365 {
		ageFactor = 0.05
	}

	var clearsFactor float64 = 0
	if clears < 50 {
		clearsFactor = ((50 - float64(clears)) / 250)
		flags |= AccountFlagClears
	}

	var starClearsFactor float64 = min(0.75,
		max(
			lowmanRatio*0.5,
			flawlessRatio*5,
			soloRatio*10,
		),
	// for every 70 clears, reduce star factor by 0.5
	) / math.Max(1, float64(clears-70)/70)
	if starClearsFactor > 0.08 {
		flags |= AccountFlagStarClears
	}

	var platformMultiplierFactor float64 = -0.2
	// ensure we have the current membership type in the list
	profile.UserInfo.ApplicableMembershipTypes = append(profile.UserInfo.ApplicableMembershipTypes, profile.UserInfo.MembershipType)
	for _, memType := range profile.UserInfo.ApplicableMembershipTypes {
		if memType == 3 {
			platformMultiplierFactor = max(platformMultiplierFactor, 0.03) // Steam
		} else if memType == 6 {
			platformMultiplierFactor = min(platformMultiplierFactor, 0.06) // Epic
			flags |= AccountFlagPlatform
		}

	}

	var characterFactor float64 = 0
	numChars := len(profile.CharacterIds)
	if numChars == 1 {
		characterFactor = 0.25
		flags |= AccountFlagCharacters
	} else if numChars == 2 {
		characterFactor = 0.15
		flags |= AccountFlagCharacters
	}

	dlcsOwned := 0
	// skip red war, coo, warmind
	for i := 3; i < 64; i++ {
		if profile.VersionsOwned&(int64(1)<<i) != 0 {
			dlcsOwned++
		}
	}
	seasonsOwned := len(profile.SeasonHashes)
	var dlcSeasonOwnershipFactor float64 = math.Pow(0.81, float64(dlcsOwned)+(0.5*float64(seasonsOwned))) - 0.15
	if dlcSeasonOwnershipFactor > 0.08 {
		flags |= AccountFlagDLCs
	}

	var guardianRankFactor float64 = 0
	if profile.LifetimeHighestGuardianRank < 5 {
		guardianRankFactor = 0.3 + (float64(4-profile.LifetimeHighestGuardianRank) * 0.05)
		flags |= AccountFlagGuardianRank
	} else if profile.LifetimeHighestGuardianRank == 5 {
		guardianRankFactor = 0.05
		flags |= AccountFlagGuardianRank
	} else if profile.CurrentGuardianRank >= 9 {
		guardianRankFactor = -0.05 - (float64(profile.LifetimeHighestGuardianRank-9) * 0.1)
	}

	var privateProfileFactor float64 = 0
	if isPrivate {
		privateProfileFactor = 0.125
		flags |= AccountFlagPrivateProfile
	}

	return cumulativeProbability(
		ageFactor,
		clearsFactor,
		starClearsFactor,
		platformMultiplierFactor,
		characterFactor,
		dlcSeasonOwnershipFactor,
		guardianRankFactor,
		privateProfileFactor,
	), flags
}

// Determine the minimum cheat level based on the number of flags
func GetMinimumCheatLevel(flag PlayerInstanceFlagStats, cheaterAccountChance float64) int {
	if (flag.FlagsA >= 5 && cheaterAccountChance >= 0.90) || (flag.FlagsA >= 10 && cheaterAccountChance >= 0.7) || (flag.FlagsA >= 25 && cheaterAccountChance >= 0.5) || (flag.FlagsA >= 50 && cheaterAccountChance >= 0.25) || (flag.FlagsA >= 100) {
		return 4
	} else if flag.FlagsB >= 15 || flag.FlagsA >= 6 || (cheaterAccountChance >= 0.75 && flag.FlagsB >= 10) {
		return 3
	} else if flag.FlagsC >= 30 || flag.FlagsB >= 10 || flag.FlagsA >= 4 || (cheaterAccountChance >= 0.6 && flag.FlagsC >= 15) {
		return 2
	} else if flag.FlagsD >= 20 || flag.FlagsC >= 5 || flag.FlagsB >= 2 || flag.FlagsA >= 1 {
		return 1
	} else {
		return 0
	}
}
