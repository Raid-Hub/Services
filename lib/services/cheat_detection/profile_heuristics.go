package cheat_detection

import (
	"math"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
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

type PlayerAccountData struct {
	MembershipId      int64
	AgeInDays         float64
	Clears            int
	MembershipType    int
	IconPath          string
	BungieName        string
	CurrentCheatLevel int
	IsPrivate         bool
	FlawlessRatio     float64
	LowmanRatio       float64
	SoloRatio         float64
	Profile           *bungie.DestinyProfileComponent
}

// perform some heuristics to determine if the player is a cheater based on history of their account,
// should return the chance of a player being a cheater based on their profile
func GetCheaterAccountChance(membershipId int64) (float64, uint64, PlayerAccountData) {
	data := PlayerAccountData{
		MembershipId: membershipId,
	}
	// get the age of the account and # of clears
	err := postgres.DB.QueryRow(`
		SELECT 
			EXTRACT(EPOCH FROM age(NOW(), first_seen)) / 86400 AS age_in_days,
			clears,
		    membership_type,
			icon_path,
			bungie_name,
			cheat_level,
			is_private
		FROM player
		WHERE membership_id = $1
	`, membershipId).Scan(&data.AgeInDays, &data.Clears, &data.MembershipType, &data.IconPath, &data.BungieName, &data.CurrentCheatLevel, &data.IsPrivate)
	if err != nil {
		logger.Warn(PLAYER_INFO_ERROR, err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return -1, 0, data
	}

	err = postgres.DB.QueryRow(`
		SELECT 
			COUNT(CASE WHEN i.completed AND flawless THEN 1 END) * 1.0 / GREATEST(COUNT(CASE WHEN i.completed = true THEN 1 END), 1) AS flawless_ratio,
			COUNT(CASE WHEN i.completed AND player_count <= 3 THEN 1 END) * 1.0 / GREATEST(COUNT(CASE WHEN i.completed = true THEN 1 END), 1) AS lowman_ratio,
			COUNT(CASE WHEN i.completed AND player_count = 1 THEN 1 END) * 1.0 / GREATEST(COUNT(CASE WHEN i.completed = true THEN 1 END), 1) AS solo_ratio
		FROM instance_player
		JOIN instance i USING (instance_id)
		WHERE i.date_started >= NOW() - INTERVAL '60 days'
			AND membership_id = $1
	`, membershipId).Scan(&data.FlawlessRatio, &data.LowmanRatio, &data.SoloRatio)
	if err != nil {
		logger.Warn(PLAYER_INFO_ERROR, err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.OPERATION:     "get_ratios",
		})
		return -1, 0, data
	}

	result, err := bungie.Client.GetProfile(data.MembershipType, membershipId, []int{100})
	if err != nil {
		logger.Warn(PLAYER_PROFILE_ERROR, err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return -1, 0, data
	}
	if !result.Success || result.Data == nil {
		logger.Info(PLAYER_NO_DATA, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			"reason":              "no_profile_data",
		})
		return -1, 0, data
	}
	res := result.Data
	data.Profile = res.Profile.Data

	flags := uint64(0)
	var ageFactor float64 = 0
	if data.AgeInDays < 7 {
		ageFactor = 0.4
		flags |= AccountFlagAge
	} else if data.AgeInDays < 30 {
		ageFactor = 0.3
		flags |= AccountFlagAge
	} else if data.AgeInDays < 90 {
		ageFactor = 0.2
		flags |= AccountFlagAge
	} else if data.AgeInDays < 365 {
		ageFactor = 0.05
	}

	var clearsFactor float64 = 0
	if data.Clears < 50 {
		clearsFactor = ((50 - float64(data.Clears)) / 250)
		flags |= AccountFlagClears
	}

	var starClearsFactor float64 = min(0.75,
		max(
			data.LowmanRatio*0.5,
			data.FlawlessRatio*5,
			data.SoloRatio*10,
		),
	// for every 70 clears, reduce star factor by 0.5
	) / math.Max(1, float64(data.Clears-70)/70)
	if starClearsFactor > 0.08 {
		flags |= AccountFlagStarClears
	}

	var platformMultiplierFactor float64 = -0.2
	// ensure we have the current membership type in the list
	data.Profile.UserInfo.ApplicableMembershipTypes = append(data.Profile.UserInfo.ApplicableMembershipTypes, data.Profile.UserInfo.MembershipType)
	for _, memType := range res.Profile.Data.UserInfo.ApplicableMembershipTypes {
		if memType == 3 {
			platformMultiplierFactor = max(platformMultiplierFactor, 0.03) // Steam
		} else if memType == 6 {
			platformMultiplierFactor = min(platformMultiplierFactor, 0.06) // Epic
			flags |= AccountFlagPlatform
		}

	}

	var characterFactor float64 = 0
	numChars := len(data.Profile.CharacterIds)
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
		if data.Profile.VersionsOwned&(int64(1)<<i) != 0 {
			dlcsOwned++
		}
	}
	seasonsOwned := len(data.Profile.SeasonHashes)
	var dlcSeasonOwnershipFactor float64 = math.Pow(0.81, float64(dlcsOwned)+(0.5*float64(seasonsOwned))) - 0.15
	if dlcSeasonOwnershipFactor > 0.08 {
		flags |= AccountFlagDLCs
	}

	var guardianRankFactor float64 = 0
	if data.Profile.LifetimeHighestGuardianRank < 5 {
		guardianRankFactor = 0.3 + (float64(4-data.Profile.LifetimeHighestGuardianRank) * 0.05)
		flags |= AccountFlagGuardianRank
	} else if data.Profile.LifetimeHighestGuardianRank == 5 {
		guardianRankFactor = 0.05
		flags |= AccountFlagGuardianRank
	} else if data.Profile.CurrentGuardianRank >= 9 {
		guardianRankFactor = -0.05 - (float64(data.Profile.LifetimeHighestGuardianRank-9) * 0.1)
	}

	var privateProfileFactor float64 = 0
	if data.IsPrivate {
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
	), flags, data
}

// Determine the minimum cheat level based on the number of flags
func GetMinimumCheatLevel(flag PlayerInstanceFlagStats, cheaterAccountChance float64) int {
	flagsA := flag.FlagsA
	flagsB := flag.FlagsB + flagsA
	flagsC := flag.FlagsC + flagsB
	flagsD := flag.FlagsD + flagsC

	if (flagsA >= 5 && cheaterAccountChance >= 0.90) || (flagsA >= 10 && cheaterAccountChance >= 0.7) || (flagsA >= 25 && cheaterAccountChance >= 0.5) || (flagsA >= 50 && cheaterAccountChance >= 0.25) || (flagsA >= 100) {
		return 4
	} else if flagsB >= 30 || flagsA >= 6 || (cheaterAccountChance >= 0.75 && flagsB >= 10) {
		return 3
	} else if flagsC >= 30 || flagsB >= 10 || flagsA >= 4 || (cheaterAccountChance >= 0.6 && flagsC >= 15) {
		return 2
	} else if flagsD >= 30 || flagsC >= 5 || flagsB >= 2 || flagsA >= 1 {
		return 1
	} else {
		return 0
	}
}

func UpdatePlayerCheatLevel(flag PlayerInstanceFlagStats) (int, float64, uint64) {
	cheaterAccountChance, bitFlags, data := GetCheaterAccountChance(flag.MembershipId)

	minCheatLevel := GetMinimumCheatLevel(flag, cheaterAccountChance)

	if minCheatLevel > data.CurrentCheatLevel {
		logger.Info(CHEAT_LEVEL_UPDATED, map[string]any{
			logging.MEMBERSHIP_ID: flag.MembershipId,
			"bungie_name":         data.BungieName,
			"previous_level":      data.CurrentCheatLevel,
			"new_level":           minCheatLevel,
			"last_played":         data.Profile.DateLastPlayed.UTC().Format("15:04:05"),
		})
		// Update the player's cheat level in the database
		_, err := postgres.DB.Exec(`
			UPDATE player
			SET cheat_level = GREATEST(cheat_level, $1)
			WHERE membership_id = $2;
		`, minCheatLevel, flag.MembershipId)

		if err != nil {
			logger.Warn(CHEAT_LEVEL_UPDATE_ERROR, err, map[string]any{
				logging.MEMBERSHIP_ID: flag.MembershipId,
			})
		}

		if minCheatLevel == 4 {
			flag.SendBlacklistedPlayerWebhook(data.Profile, data.Clears, data.AgeInDays, data.BungieName, data.IconPath, cheaterAccountChance, bitFlags)
		}
	}

	return minCheatLevel, cheaterAccountChance, bitFlags
}
