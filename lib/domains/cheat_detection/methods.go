package cheat_detection

import (
	"fmt"
	"math"
	"strings"
	"time"
)

var crafteningStart = time.Date(2023, 9, 15, 13, 54, 00, 0, time.UTC)
var crafteningEnd = time.Date(2023, 9, 18, 4, 00, 9, 0, time.UTC)

// Skip window for the bad flagging period (prevent recurrence for Sept 16-23, 2025)
var quickfangStart = time.Date(2025, 9, 16, 17, 0, 0, 0, time.UTC)
var quickfangEnd = time.Date(2025, 9, 23, 17, 0, 0, 0, time.UTC)

func (h ActivityHeuristic) apply(instance *Instance) (ResultTuple, map[int64]ResultTuple) {
	// Instances overlapping known problematic windows should be skipped
	if (instance.DateCompleted.After(crafteningStart) && instance.DateStarted.Before(crafteningEnd)) ||
		(instance.DateCompleted.After(quickfangStart) && instance.DateStarted.Before(quickfangEnd)) {
		// We cannot apply any heuristics to instances that are part of these windows
		return ResultTuple{}, map[int64]ResultTuple{}
	}

	if !instance.Completed && (instance.Activity == 4 || instance.Activity == 11) {
		// lot's of shuro and golgy false positive flags here potentially
		return ResultTuple{}, map[int64]ResultTuple{}
	}

	var lowmanPrb float64
	var lowmanReasonBit uint64
	var lowmanExplanation string

	if instance.Completed {
		if instance.Fresh != nil && *instance.Fresh {
			lowmanPrb, lowmanReasonBit, lowmanExplanation = h.applyFreshLowman(instance)
		} else {
			lowmanPrb, lowmanReasonBit, lowmanExplanation = h.applyCheckpointLowman(instance)
		}
	}

	a, b := h.applyGeneral(instance, lowmanPrb, lowmanReasonBit, lowmanExplanation)

	return resultsAdjustedIfSolo(instance, a, b)

}

func (h ActivityHeuristic) applyFreshLowman(instance *Instance) (float64, uint64, string) {
	versionH, versionExists := h.FreshLowman[instance.Version]
	if versionExists {
		return h.iterateLowmanData(stringOfVersion(instance.Version), versionH, instance, false)
	}

	anyVersionH, anyVersionExists := h.FreshLowman[Any]
	if anyVersionExists {
		return h.iterateLowmanData(stringOfVersion(Any), anyVersionH, instance, false)
	}

	return nilResult()
}

func (h ActivityHeuristic) applyCheckpointLowman(instance *Instance) (float64, uint64, string) {
	versionH, versionExists := h.CheckpointLowman[instance.Version]
	if versionExists {
		return h.iterateLowmanData(stringOfVersion(instance.Version), versionH, instance, true)
	}

	anyVersionH, anyVersionExists := h.CheckpointLowman[Any]
	if anyVersionExists {
		return h.iterateLowmanData(stringOfVersion(Any), anyVersionH, instance, true)
	}

	return nilResult()
}

func (h ActivityHeuristic) iterateLowmanData(key string, arr []LowmanData, instance *Instance, isCheckpoint bool) (float64, uint64, string) {
	for _, data := range arr {
		var explanation string
		var reasonBit = h.RaidBit
		if instance.PlayerCount < data.MinPlayers {
			if isCheckpoint {
				if instance.DurationSeconds < int(data.MinTime.Seconds()) {
					reasonBit |= FastLowmanCheckpoint
				}
				explanation = fmt.Sprintf("cleared %s (%s) with %d players, expected at least %d",
					h.CheckpointName, key, instance.PlayerCount, data.MinPlayers)
				reasonBit |= TooFewPlayersFresh
			} else {
				explanation = fmt.Sprintf(
					"cleared fresh %s (%s) with %d players, expected at least %d",
					h.RaidName, key, instance.PlayerCount, data.MinPlayers,
				)
				reasonBit |= TooFewPlayersCheckpoint
			}

			return 0.995, reasonBit, explanation
		} else if instance.PlayerCount == data.MinPlayers {
			rawCheatedChance := data.CheatedChance
			if instance.Flawless != nil && *instance.Flawless {
				rawCheatedChance = math.Pow(rawCheatedChance, 0.5)
			}

			if isCheckpoint && instance.DurationSeconds < int(data.MinTime.Seconds()) {
				reasonBit |= FastLowmanCheckpoint
				prbFastCp := varLowmanCheckpointDurationRatioCurve(float64(instance.DurationSeconds) / float64(data.MinTime.Seconds()))
				rawCheatedChance = cumulativeProbability(rawCheatedChance, prbFastCp)
			}

			if rawCheatedChance > Threshold {
				flawlessStr := ""
				if instance.Flawless != nil && *instance.Flawless {
					flawlessStr = "flawless "
				}
				reasonBit |= UnlikelyLowman
				clearName := h.RaidName
				if isCheckpoint {
					clearName = h.CheckpointName
				}
				explanation = fmt.Sprintf(
					"cleared %s%s (%s) with exactly %d players in %.2f minutes (%.3f)",
					flawlessStr, clearName, key, instance.PlayerCount,
					float64(instance.DurationSeconds)/60, rawCheatedChance,
				)
			}

			if len(data.Range) == 0 {
				return rawCheatedChance, reasonBit, explanation
			}

			for _, r := range data.Range {
				if instance.DateCompleted.After(r.Start) && instance.DateCompleted.Before(r.End) {
					return rawCheatedChance, reasonBit, explanation
				}
			}

			if isCheckpoint {
				explanation = fmt.Sprintf("cleared %s with %d players outside of the window",
					h.CheckpointName, instance.PlayerCount)
				reasonBit |= TooFewPlayersCheckpoint
			} else {
				explanation = fmt.Sprintf(
					"cleared fresh %s with %d players outside of the window",
					h.RaidName, instance.PlayerCount,
				)
				reasonBit |= TooFewPlayersFresh
			}
			return 0.995, reasonBit, explanation
		}
	}

	return nilResult()
}

func (h ActivityHeuristic) applyGeneral(instance *Instance, lowmanPrb float64, lowmanReasonBit uint64, lowmanExplanation string) (ResultTuple, map[int64]ResultTuple) {

	isFresh := instance.Fresh != nil && *instance.Fresh

	totalTimeForAllPlayers := 0
	for _, player := range instance.Players {
		totalTimeForAllPlayers += player.TimePlayedSeconds
	}

	totalInstanceKills := 0
	for _, player := range instance.Players {
		for _, char := range player.Characters {
			totalInstanceKills += char.Kills
		}
	}

	actualVersusMeasuredTime := float64(totalTimeForAllPlayers) / float64(instance.DurationSeconds)

	isMaxIntTimePlayed := false
	for _, player := range instance.Players {
		for _, char := range player.Characters {
			if char.TimePlayedSeconds == 32767 {
				isMaxIntTimePlayed = true
				break
			}
		}
	}

	var finalResult ResultTuple
	finalResult.Reason = h.RaidBit
	finalExplanations := make([]string, 0)
	var cheatedTimePrb float64 = 0.0
	var timeDilationPrb float64 = 0.0
	var totalKillsCheatPrb float64 = 0.0
	var cheatedTimeExplanation string
	var timeDilationExplanation string
	var totalKillsCheatExplanation string

	// instance level
	if actualVersusMeasuredTime < 1 && instance.DurationSeconds > 360 && instance.PlayerCount < 50 && !isMaxIntTimePlayed {
		timeDilationPrb = timeDilationCurve(actualVersusMeasuredTime)
		timeDilationExplanation = fmt.Sprintf(
			"measured time (%02d:%02d) is %.2f%% shorter than maximum actual time (%02d:%02d)",
			instance.DurationSeconds/60, instance.DurationSeconds%60,
			(1-actualVersusMeasuredTime)*100,
			int(float64(totalTimeForAllPlayers)/60), totalTimeForAllPlayers%60,
		)
	}

	if instance.Completed && isFresh {
		estimatedWorldRecordAtClearTime := h.SpeedrunCurve(instance.DaysAfterRelease)

		bodies := min(float64(totalTimeForAllPlayers)/float64(instance.DurationSeconds), 6)
		adjustedExpectedRecordTime := estimatedWorldRecordAtClearTime * math.Pow(6/bodies, 0.2)

		// timeRatio < 1 means the instance was cleared faster than expected
		completionTimeRatio := float64(instance.DurationSeconds-25) / adjustedExpectedRecordTime

		cheatedTimePrb = completionTimeCurve(completionTimeRatio)
		cheatedTimeExplanation = fmt.Sprintf(
			"cleared %s in %02d:%02d, expected %02d:%02d (adjusted for player count: %d:%d)",
			h.RaidName, instance.DurationSeconds/60, instance.DurationSeconds%60,
			int(estimatedWorldRecordAtClearTime/60), int(estimatedWorldRecordAtClearTime)%60,
			int(adjustedExpectedRecordTime/60), int(adjustedExpectedRecordTime)%60,
		)

		finalExplanations = append(finalExplanations, "fresh completion")
		ratio := float64(totalInstanceKills+1) / float64(h.MinFreshKills+1)
		// Instances with a lower min kill count are harder to flag
		totalKillsCheatPrb = totalInstanceKillsCurve(ratio) * totalInstanceKillsSecondaryCurve(float64(h.MinFreshKills+1))
		totalKillsCheatExplanation = fmt.Sprintf("cleared fresh %s with %d kills, expected at least %d",
			h.RaidName, totalInstanceKills, h.MinFreshKills)

		// adjust really low kill counts to be more likely to be flagged
		localMin := (min(h.MinFreshKills, 30))
		if totalInstanceKills < localMin {
			totalKillsCheatPrb = math.Pow(totalKillsCheatPrb, (0.5 + float64(totalInstanceKills)/float64(2*localMin)))
		}

		if totalKillsCheatPrb > Threshold {
			finalResult.Reason |= TotalInstanceKills
			finalExplanations = append(finalExplanations, totalKillsCheatExplanation)
		}

		if cheatedTimePrb > Threshold {
			finalResult.Reason |= TooFast
			finalExplanations = append(finalExplanations, cheatedTimeExplanation)
		}

		if timeDilationPrb > Threshold {
			finalResult.Reason |= TimeDilation
			finalExplanations = append(finalExplanations, timeDilationExplanation)
		}

		finalResult.Probability = cumulativeProbability(totalKillsCheatPrb, cheatedTimePrb, timeDilationPrb)
	} else if instance.Completed {
		finalExplanations = append(finalExplanations, "checkpoint completion")
		ratio := float64(totalInstanceKills+1) / float64(h.MinCheckpointKills+1)
		// Instances with a lower min kill count are harder to flag
		totalKillsCheatPrb = totalInstanceKillsCurve(ratio) * totalInstanceKillsSecondaryCurve(float64(h.MinCheckpointKills+1))
		totalKillsCheatExplanation = fmt.Sprintf("cleared %s with %d kills, expected at least %d",
			h.CheckpointName, totalInstanceKills, h.MinCheckpointKills)

		// adjust really low kill counts to be more likely to be flagged
		localMin := (min(h.MinFreshKills, 12))
		if totalInstanceKills < localMin {
			totalKillsCheatPrb = math.Pow(totalKillsCheatPrb, (0.5 + float64(totalInstanceKills)/float64(2*localMin)))
		}

		if totalKillsCheatPrb > Threshold {
			finalExplanations = append(finalExplanations, totalKillsCheatExplanation)
			finalResult.Reason |= TotalInstanceKills
		}

		if timeDilationPrb > Threshold {
			finalExplanations = append(finalExplanations, timeDilationExplanation)
			finalResult.Reason |= TimeDilation
		}

		finalResult.Probability = cumulativeProbability(totalKillsCheatPrb, timeDilationPrb)
	} else {
		finalExplanations = append(finalExplanations, "incomplete")
		// If the instance is not completed, we can still check for time dilation
		if timeDilationPrb > Threshold {
			finalResult.Reason |= TimeDilation
			finalExplanations = append(finalExplanations, timeDilationExplanation)
		}
		finalResult.Probability = cumulativeProbability(timeDilationPrb)

	}

	// lowman stuff
	if lowmanPrb > Threshold {
		finalResult.Reason |= lowmanReasonBit
		finalExplanations = append(finalExplanations, lowmanExplanation)
	}
	finalResult.Probability = cumulativeProbability(finalResult.Probability, lowmanPrb)

	// at the player level
	playerResults := make(map[int64]ResultTuple)
	for _, player := range instance.Players {
		individualPlayerKillsCheatPrb, reason, explanations := player.killsCheatProbability(totalInstanceKills, totalTimeForAllPlayers)

		// applies instance level probabilities to player level
		participationRatioPercentage := participationCurve(instance.PlayerCount, player.totalKills(), totalInstanceKills, player.TimePlayedSeconds, totalTimeForAllPlayers)

		cheatedSpeedrunParticipationPrb := participationRatioPercentage * cheatedTimePrb
		if cheatedSpeedrunParticipationPrb > PlayerThreshold {
			reason |= TooFast
			explanations = append(explanations, fmt.Sprintf("cheated speedrun participation: %.4f", cheatedSpeedrunParticipationPrb))
		}

		cheatedInstanceKillsPrb := participationRatioPercentage * totalKillsCheatPrb
		if cheatedInstanceKillsPrb > PlayerThreshold {
			reason |= TotalInstanceKills
			explanations = append(explanations, fmt.Sprintf("total instance kills participation: %.4f", cheatedInstanceKillsPrb))
		}

		playerTimeDilationPrb := participationRatioPercentage * timeDilationPrb
		if playerTimeDilationPrb > PlayerThreshold {
			reason |= TimeDilation
			explanations = append(explanations, fmt.Sprintf("player time dilation: %.4f", playerTimeDilationPrb))
		}

		playerLowmanPrb := participationRatioPercentage * lowmanPrb
		if playerLowmanPrb > PlayerThreshold {
			reason |= lowmanReasonBit
			explanations = append(explanations, fmt.Sprintf("improbable lowman participation: %.4f", playerLowmanPrb))
		}

		prb := cumulativeProbability(
			cheatedInstanceKillsPrb,
			individualPlayerKillsCheatPrb,
			cheatedSpeedrunParticipationPrb,
			playerTimeDilationPrb,
			playerLowmanPrb,
		)

		if !player.Finished {
			prb /= (notCompletedAdjustmentCurve(float64(player.TimePlayedSeconds)) + 1)
			explanations = append(explanations, "dnf")
		} else if player.IsFirstClear {
			reason |= FirstClear
			explanations = append(explanations, "first clear")
			prb = math.Pow(prb, 0.8)
		}

		playerResults[player.MembershipId] = ResultTuple{
			MembershipId: player.MembershipId,
			Probability:  prb,
			Explanation:  strings.Join(explanations, ", "),
			Reason:       reason,
		}
	}

	// Probability of 2 or more players cheating
	allPlayerProbs := []float64{}
	for _, player := range playerResults {
		allPlayerProbs = append(allPlayerProbs, player.Probability)
	}
	p2orMore := prbAtLeastTwo(allPlayerProbs)

	if p2orMore > Threshold {
		finalResult.Reason |= TwoPlusCheaters
		finalExplanations = append(finalExplanations, fmt.Sprintf("multiple players cheating in %s", h.RaidName))
	}
	finalResult.Probability = cumulativeProbability(finalResult.Probability, p2orMore)

	finalResult.Explanation = strings.Join(finalExplanations, ", ")
	return finalResult, playerResults
}

func (p *Player) totalKills() int {
	totalKills := 0
	for _, char := range p.Characters {
		totalKills += char.Kills
	}
	return totalKills
}

func (p *Player) killsPerMinute() float64 {
	totalKills := 0
	for _, char := range p.Characters {
		totalKills += char.Kills
	}

	return 60 * float64(totalKills) / float64(p.TimePlayedSeconds)
}

func (p *Player) grenadeKillsPerMinute() float64 {
	grenadeKills := 0
	for _, char := range p.Characters {
		grenadeKills += char.GrenadeKills
	}

	return 60 * float64(grenadeKills) / float64(p.TimePlayedSeconds)
}

func (p *Player) meleeKillsPerMinute() float64 {
	meleeKills := 0
	for _, char := range p.Characters {
		meleeKills += char.MeleeKills
	}

	return 60 * float64(meleeKills) / float64(p.TimePlayedSeconds)
}

func (p *Player) superKillsPerMinute() float64 {
	superKills := 0
	for _, char := range p.Characters {
		superKills += char.SuperKills
	}

	return 60 * float64(superKills) / float64(p.TimePlayedSeconds)
}

func (i *Player) weaponDiversity() float64 {
	totalWeapons := 0
	totalWeaponKills := 0

	for _, char := range i.Characters {
		for _, weapon := range char.Weapons {
			totalWeapons++
			totalWeaponKills += weapon.Kills
		}
	}

	if totalWeaponKills < 150 {
		// If there are less than 150 kills, we cannot calculate a meaningful score
		return 100
	}

	if totalWeapons <= 3 {
		baseScore := (float64(totalWeapons+1) * 1200 / float64(i.TimePlayedSeconds))
		baseScore /= (float64(max(totalWeaponKills, 800)) / 800)
		if !i.Finished {
			return math.Pow(baseScore, 2)
		} else {
			return baseScore
		}
	}

	avgKillsPerWeapon := float64(totalWeaponKills) / float64(totalWeapons)
	// calculate how many weapons are above 2 standard deviations from the mean
	varianceSum := 0.0
	for _, char := range i.Characters {
		for _, weapon := range char.Weapons {
			varianceSum += math.Pow(float64(weapon.Kills)-avgKillsPerWeapon, 2)
		}
	}

	standardDeviation := math.Sqrt(varianceSum / (float64(totalWeapons) - 1))

	// calculate how many weapons are above/below 2 standard deviations from the mean
	outliers := 0
	for _, char := range i.Characters {
		for _, weapon := range char.Weapons {
			if float64(weapon.Kills) > avgKillsPerWeapon+2*standardDeviation || float64(weapon.Kills) < avgKillsPerWeapon-2*standardDeviation {
				outliers++
			}
		}
	}

	adjStdDev := max(standardDeviation, 1)

	baseScore := math.Sqrt(avgKillsPerWeapon/adjStdDev) + math.Pow(float64(totalWeapons-outliers), 1.5)
	baseScore /= (float64(max(totalWeaponKills, 600)) / 600)

	if !i.Finished {
		return math.Pow(baseScore, 2)
	} else {
		return baseScore
	}
}

func (p Player) heavyAmmoCheat() float64 {
	heavyAmmoKills := 0
	totalKills := 0
	for _, char := range p.Characters {
		for _, weapon := range char.Weapons {
			if weapon.AmmoType == "Heavy" {
				heavyAmmoKills += weapon.Kills
			}
			totalKills += weapon.Kills
		}
	}

	totalKillsMultiplier := float64(min(100, totalKills)) / 100.0

	killsPerSecondMultiplier := min(1.25, 5*float64(heavyAmmoKills)/float64(p.TimePlayedSeconds))

	return heavyCurve(killsPerSecondMultiplier*float64(heavyAmmoKills)/float64(totalKills+1)) * totalKillsMultiplier
}

func (p Player) killsCheatProbability(totalInstanceKills int, totalPlayerSeconds int) (float64, uint64, []string) {
	if p.TimePlayedSeconds < 60 {
		return 0.0, 0, []string{}
	}
	meleeOffset := max(p.grenadeKillsPerMinute()-p.meleeKillsPerMinute(), 0)
	adjustForLowTimePlayed := (min(300, float64(p.TimePlayedSeconds)) / 300)
	probGrenadeCheat := grenadeCurve(meleeOffset) * adjustForLowTimePlayed
	probHeavyAmmoCheat := p.heavyAmmoCheat() * adjustForLowTimePlayed
	probKillsPerSecondCheat := killsCurve(p.killsPerMinute()) * adjustForLowTimePlayed
	probSuperCheat := superCurve(p.superKillsPerMinute()) * adjustForLowTimePlayed
	probWeaponCheat := math.Exp(-0.3*p.weaponDiversity()) * adjustForLowTimePlayed * 0.5

	killsRatio := (float64(p.totalKills()) / float64(totalInstanceKills))
	maxExpectedKillsRatio := (math.Log((float64(p.TimePlayedSeconds)/float64(totalPlayerSeconds))+0.05) / math.Log(30)) + 0.99
	killsRatioVsMaxExpectedKillsRatioRatio := killsRatio / maxExpectedKillsRatio
	probKillsShareCheat := 0.0
	if killsRatioVsMaxExpectedKillsRatioRatio > 1 && totalInstanceKills > 20 {
		probKillsShareCheat = killsShareCurve(killsRatioVsMaxExpectedKillsRatioRatio) * adjustForLowTimePlayed
	}

	var bits uint64 = 0
	explanations := make([]string, 0)
	if probKillsPerSecondCheat > PlayerThreshold {
		bits |= PlayerTotalKills
		explanations = append(explanations, fmt.Sprintf("total kills rate: %.4f", probKillsPerSecondCheat))
	}

	if probGrenadeCheat > PlayerThreshold {
		bits |= PlayerGrenadeKills
		explanations = append(explanations, fmt.Sprintf("grenade kills rate: %.4f", probGrenadeCheat))
	}

	if probHeavyAmmoCheat > PlayerThreshold {
		bits |= PlayerHeavyAmmoKills
		explanations = append(explanations, fmt.Sprintf("heavy ammo cheat: %.4f", probHeavyAmmoCheat))
	}

	if probSuperCheat > PlayerThreshold {
		bits |= PlayerSuperKills
		explanations = append(explanations, fmt.Sprintf("super kills rate: %.4f", probSuperCheat))
	}

	if probWeaponCheat > PlayerThreshold {
		bits |= PlayerWeaponDiversity
		explanations = append(explanations, fmt.Sprintf("weapon diversity: %.4f", probWeaponCheat))
	}

	if probKillsShareCheat > PlayerThreshold {
		bits |= PlayerKillsShare
		explanations = append(explanations, fmt.Sprintf("kills share: %.4f", probKillsShareCheat))
	}

	allProbs := []float64{probKillsPerSecondCheat, probGrenadeCheat, probHeavyAmmoCheat, probSuperCheat, probWeaponCheat, probKillsShareCheat}

	return prbAtLeastTwo(allProbs), bits, explanations
}
