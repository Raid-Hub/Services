package cheat_detection

import (
	"fmt"
	"math"
	"time"
)

var crafteningStart = time.Date(2023, 9, 15, 13, 54, 00, 0, time.UTC)
var crafteningEnd = time.Date(2023, 9, 18, 4, 00, 9, 0, time.UTC)

func (h ActivityHeuristic) apply(instance *Instance) (ResultTuple, map[int64]ResultTuple) {
	if instance.DateCompleted.After(crafteningStart) && instance.DateStarted.Before(crafteningEnd) {
		// We cannot apply any heuristics to instances that are part of the craftening
		a, b, _ := nilResult()
		return a, b
	}

	if !instance.Completed && (instance.Activity == 4 || instance.Activity == 11) {
		// lot's of shuro and golgy false positive flags here potentially
		a, b, _ := nilResult()
		return a, b
	}

	if instance.Completed {
		if instance.Fresh != nil && *instance.Fresh {
			a, b, isAppplied := h.applyFreshLowman(instance)
			if isAppplied {
				return a, b
			}

			a, b, isAppplied = h.applySpeedrun(instance)
			if isAppplied {
				return a, b
			}
		}

		a, b, isAppplied := h.applyCheckpointLowman(instance)
		if isAppplied {
			return a, b
		}
	}

	return h.applyGeneral(instance)
}

func (h ActivityHeuristic) applyFreshLowman(instance *Instance) (ResultTuple, map[int64]ResultTuple, bool) {
	versionH, versionExists := h.FreshLowman[instance.Version]
	if versionExists {
		a, b, isApplied := h.iterateLowmanData(versionH, instance, false)
		if isApplied {
			return a, b, isApplied
		}
	}

	anyVersionH, anyVersionExists := h.FreshLowman[Any]
	if anyVersionExists {
		a, b, isApplied := h.iterateLowmanData(anyVersionH, instance, false)
		if isApplied {
			return a, b, isApplied
		}
	}

	return nilResult()
}

func (h ActivityHeuristic) applyCheckpointLowman(instance *Instance) (ResultTuple, map[int64]ResultTuple, bool) {
	versionH, versionExists := h.CheckpointLowman[instance.Version]
	if versionExists {
		a, b, isApplied := h.iterateLowmanData(versionH, instance, true)
		if isApplied {
			return a, b, isApplied
		}
	}

	anyVersionH, anyVersionExists := h.CheckpointLowman[Any]
	if anyVersionExists {
		a, b, isApplied := h.iterateLowmanData(anyVersionH, instance, true)
		if isApplied {
			return a, b, isApplied
		}
	}

	return nilResult()
}

func (h ActivityHeuristic) iterateLowmanData(arr []LowmanData, instance *Instance, isCheckpoint bool) (ResultTuple, map[int64]ResultTuple, bool) {
	for _, data := range arr {
		var explanation string
		var reasonBit uint64
		if instance.PlayerCount < data.MinPlayers {
			if isCheckpoint {
				explanation = fmt.Sprintf("Cleared %s with %d players, expected at least %d",
					h.CheckpointName, instance.PlayerCount, data.MinPlayers)
				reasonBit = TooFewPlayersFresh
			} else {
				explanation = fmt.Sprintf(
					"Cleared fresh %s with %d players, expected at least %d",
					h.RaidName, instance.PlayerCount, data.MinPlayers,
				)
				reasonBit = TooFewPlayersCheckpoint
			}
			return resultForSharedFate(1.0, h.RaidBit|reasonBit, explanation, instance.Players)
		} else if instance.PlayerCount == data.MinPlayers {
			if len(data.Range) == 0 {
				return nilResult()
			}

			for _, r := range data.Range {
				if instance.DateCompleted.After(r.Start) && instance.DateCompleted.Before(r.End) {
					return nilResult()
				}
			}

			if isCheckpoint {
				explanation = fmt.Sprintf("Cleared %s with %d players outside of the window",
					h.CheckpointName, instance.PlayerCount)
				reasonBit = TooFewPlayersCheckpoint
			} else {
				explanation = fmt.Sprintf(
					"Cleared fresh %s with %d players outside of the window",
					h.RaidName, instance.PlayerCount,
				)
				reasonBit = TooFewPlayersFresh
			}
			return resultForSharedFate(1.0, h.RaidBit|reasonBit, explanation, instance.Players)
		}
	}

	return nilResult()
}

func buildLogisticCurve(k, x0 float64) func(x float64) float64 {
	return func(x float64) float64 {
		return 1 / (1 + math.Exp(k*(x-x0)))
	}
}

var (
	killsCurve                       = buildLogisticCurve(-0.15, 40)
	grenadeCurve                     = buildLogisticCurve(-0.35, 20)
	superCurve                       = buildLogisticCurve(-0.75, 6)
	completionTimeCurve              = buildLogisticCurve(40, 0.93)
	totalInstanceKillsCurve          = buildLogisticCurve(20, 0.83)
	totalInstanceKillsSecondaryCurve = buildLogisticCurve(-0.04, 60)
)

func (h ActivityHeuristic) applySpeedrun(instance *Instance) (ResultTuple, map[int64]ResultTuple, bool) {
	estimatedWorldRecordAtClearTime := h.SpeedrunCurve(instance.DaysAfterRelease)
	if float64(instance.DurationSeconds)-estimatedWorldRecordAtClearTime <= -45 {
		explanation := fmt.Sprintf(
			"Cleared %s significantly faster than %02d:%02d",
			h.RaidName, int(estimatedWorldRecordAtClearTime/60), int(estimatedWorldRecordAtClearTime)%60,
		)
		return resultForSharedFate(1.0, h.RaidBit|TooFast, explanation, instance.Players)
	}

	return nilResult()
}

func (h ActivityHeuristic) applyGeneral(instance *Instance) (ResultTuple, map[int64]ResultTuple) {
	var cheatedTimePrb float64 = 0.0
	var cheatedTimeExplanation string

	isFresh := instance.Fresh != nil && *instance.Fresh

	if instance.Completed && isFresh {
		estimatedWorldRecordAtClearTime := h.SpeedrunCurve(instance.DaysAfterRelease)
		totalTimeForAllPlayers := 0
		for _, player := range instance.Players {
			totalTimeForAllPlayers += player.TimePlayedSeconds
		}

		bodies := float64(totalTimeForAllPlayers) / float64(instance.DurationSeconds)
		if bodies > 6 {
			bodies = 6
		}
		adjustedExpectedRecordTime := estimatedWorldRecordAtClearTime * math.Pow(6/bodies, 0.2)

		// timeRatio < 1 means the instance was cleared faster than expected
		completionTimeRatio := float64(instance.DurationSeconds-25) / adjustedExpectedRecordTime

		cheatedTimePrb = completionTimeCurve(completionTimeRatio)

		cheatedTimeExplanation = fmt.Sprintf(
			"Cleared %s in %02d:%02d, expected %02d:%02d (adjusted for player count: %d:%d)",
			h.RaidName, instance.DurationSeconds/60, instance.DurationSeconds%60,
			int(estimatedWorldRecordAtClearTime/60), int(estimatedWorldRecordAtClearTime)%60,
			int(adjustedExpectedRecordTime/60), int(adjustedExpectedRecordTime)%60,
		)
	}

	results := make(map[int64]ResultTuple)
	for _, player := range instance.Players {
		if player.TimePlayedSeconds < 70 {
			continue
		}

		player_prb, reason, explanation := player.killsCheatProbability()

		prb := (1 - (1-player_prb)*(1-cheatedTimePrb))

		if !player.Finished {
			prb /= 2
		} else if player.IsFirstClear {
			prb = math.Pow(prb, 3)
		}

		if prb > PlayerThreshold {
			results[player.MembershipId] = ResultTuple{
				MembershipId: player.MembershipId,
				Probability:  prb,
				Explanation:  explanation,
				Reason:       reason,
			}
		}
	}

	totalInstanceKills := 0
	for _, player := range instance.Players {
		for _, char := range player.Characters {
			totalInstanceKills += char.Kills
		}
	}

	if isFresh && instance.Completed {
		ratio := float64(totalInstanceKills+1) / float64(h.MinFreshKills+1)
		// Instances with a lower min kill count are harder to flag
		killsCheatPrb := totalInstanceKillsCurve(ratio) * totalInstanceKillsSecondaryCurve(float64(h.MinFreshKills+1))

		reason := h.RaidBit
		explanations := (make([]string, 0))

		if killsCheatPrb > Threshold {
			reason |= TotalInstanceKills
			explanations = append(explanations, fmt.Sprintf("Cleared fresh %s with %d kills, expected at least %d",
				h.RaidName, totalInstanceKills, h.MinFreshKills))
		}

		if cheatedTimePrb > Threshold {
			reason |= TooFast
			explanations = append(explanations, cheatedTimeExplanation)
		}

		combinedPrb := 1 - (1-killsCheatPrb)*(1-cheatedTimePrb)
		if combinedPrb > Threshold {
			expString := "No explanation"
			if len(explanations) == 2 {
				expString = fmt.Sprintf("%s and %s", explanations[0], explanations[1])
			} else if len(explanations) == 1 {
				expString = explanations[0]
			}

			return resultsAdjustedIfSolo(instance.PlayerCount, combinedPrb, reason, expString, results)
		}
	} else if instance.Completed {
		ratio := float64(totalInstanceKills+1) / float64(h.MinCheckpointKills+1)
		// Instances with a lower min kill count are harder to flag
		killsCheatPrb := totalInstanceKillsCurve(ratio) * totalInstanceKillsSecondaryCurve(float64(h.MinCheckpointKills+1))

		if killsCheatPrb > Threshold {
			explanation := fmt.Sprintf("Cleared %s with %d kills, expected at least %d",
				h.CheckpointName, totalInstanceKills, h.MinCheckpointKills)
			return resultsAdjustedIfSolo(instance.PlayerCount, killsCheatPrb, h.RaidBit|TotalInstanceKills, explanation, results)
		}
	}

	allProbs := []float64{}
	for _, player := range results {
		allProbs = append(allProbs, (player.Probability * (1 - cheatedTimePrb)))
	}

	// Probability of 2 or more players cheating
	p2orMore := prbAtLeastTwo(allProbs)

	if p2orMore > Threshold {
		return resultsAdjustedIfSolo(instance.PlayerCount, p2orMore, TwoPlusCheaters|h.RaidBit, fmt.Sprintf("Multiple players cheating in %s", h.RaidName), results)
	}

	return ResultTuple{}, results
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
	heavyAmmoKills := 0

	for _, char := range i.Characters {
		for _, weapon := range char.Weapons {
			totalWeapons++
			totalWeaponKills += weapon.Kills
			if weapon.AmmoType == "Heavy" {
				heavyAmmoKills += weapon.Kills
			}
		}
	}

	// score for too many heavy ammo kills
	heavyScore := max(0, heavyAmmoKills - totalWeaponKills)

	if totalWeapons <= 3 {
		baseScore := (float64(totalWeapons+1) * 1200 / float64(i.TimePlayedSeconds)) + float64(heavyScore)
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

	adjStdDev := standardDeviation
	if standardDeviation < 1 {
		adjStdDev = 1
	}

	baseScore := math.Sqrt(avgKillsPerWeapon/adjStdDev) + math.Pow(float64(totalWeapons-outliers), 1.5) + float64(heavyScore)
	if !i.Finished {
		return math.Pow(baseScore, 2)
	} else {
		return baseScore
	}
}

func (p Player) killsCheatProbability() (float64, uint64, string) {
	meleeOffset := p.grenadeKillsPerMinute() - p.meleeKillsPerMinute()
	// Most cheaters don't melee while legit players do (sunbracers, etc)
	if meleeOffset < 0 {
		meleeOffset = 0
	}
	probGrenadeCheat := grenadeCurve(meleeOffset)
	probKillsCheat := killsCurve(p.killsPerMinute())
	probSuperCheat := superCurve(p.superKillsPerMinute())
	probWeaponCheat := math.Exp(-0.2 * p.weaponDiversity())

	var bits uint64 = 0
	explanation := "Failed kill-based heuristics: "
	if probKillsCheat > PlayerThreshold {
		bits |= PlayerTotalKills
		explanation += fmt.Sprintf("total kills: %.4f, ", probKillsCheat)
	}

	if probGrenadeCheat > PlayerThreshold {
		bits |= PlayerGrenadeKills
		explanation += fmt.Sprintf("grenade kills: %.4f, ", probGrenadeCheat)
	}

	if probSuperCheat > PlayerThreshold {
		bits |= PlayerSuperKills
		explanation += fmt.Sprintf("super kills: %.4f, ", probSuperCheat)
	}

	if probWeaponCheat > PlayerThreshold {
		bits |= PlayerWeaponDiversity
		explanation += fmt.Sprintf("weapon diversity: %.4f, ", probWeaponCheat)
	}
	explanation = explanation[:len(explanation)-2]

	allProbs := []float64{probKillsCheat, probGrenadeCheat, probSuperCheat, probWeaponCheat}

	// P(2 or more) = 1 - P(0) - P(1)
	return prbAtLeastTwo(allProbs), bits, explanation
}

func prbAtLeastTwo(allProbs []float64) float64 {
	// get the probability that 2 events happened
	p0 := 1.0
	for _, prob := range allProbs {
		p0 *= (1 - prob)
	}

	p1 := 0.0
	for i := 0; i < len(allProbs); i++ {
		pi := allProbs[i]
		rest := 1.0
		for j := 0; j < len(allProbs); j++ {
			if j != i {
				rest *= (1 - allProbs[j])
			}
		}
		p1 += pi * rest
	}

	return 1 - p0 - p1
}
