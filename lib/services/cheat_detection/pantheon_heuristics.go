package cheat_detection

import "fmt"

const (
	pantheonEmptyFeatSkullHash     = int64(790421403)
	pantheonBattalionsSkullHash    = int64(2392637702)
	pantheonCutthroatSkullHash     = int64(2673088233)
	pantheonBattalionsBonusSeconds = 60     // 1m
	pantheonCutthroatBonusSeconds  = 2 * 60 // 2m
)

// Pantheon version IDs (version_definition.id).
const (
	pantheonVersionCalusResplendent               = 132
	pantheonVersionMorgethSurpassing              = 133
	pantheonVersionInsurrectionPrimeRevolutionary = 134
)

func pantheonFeatCount(skullHashes []int64) int {
	count := 0
	for _, hash := range skullHashes {
		if hash != pantheonEmptyFeatSkullHash {
			count++
		}
	}
	return count
}

func pantheonTimingFeatBonusSeconds(skullHashes []int64) int {
	bonus := 0
	for _, hash := range skullHashes {
		switch hash {
		case pantheonBattalionsSkullHash:
			bonus += pantheonBattalionsBonusSeconds
		case pantheonCutthroatSkullHash:
			bonus += pantheonCutthroatBonusSeconds
		}
	}
	return bonus
}

// pantheonOutlierFloorSeconds returns the minimum plausible fresh-clear duration.
// Only Battalions and Cutthroat increase the floor; other feats do not add time.
func pantheonOutlierFloorSeconds(version int, skullHashes []int64) int {
	bonus := pantheonTimingFeatBonusSeconds(skullHashes)

	switch version {
	case pantheonVersionCalusResplendent:
		return 7*60 + bonus
	case pantheonVersionInsurrectionPrimeRevolutionary:
		return 15*60 + bonus
	default:
		return 0
	}
}

func pantheonDurationCheck(instance *Instance) (float64, string) {
	if !instance.Completed || instance.Fresh == nil || !*instance.Fresh {
		return 0, ""
	}

	outlierFloor := pantheonOutlierFloorSeconds(instance.Version, instance.SkullHashes)
	if outlierFloor == 0 || instance.DurationSeconds >= outlierFloor {
		return 0, ""
	}

	ratio := float64(instance.DurationSeconds) / float64(outlierFloor)
	prob := pantheonCompletionTimeCurve(ratio)

	featCount := pantheonFeatCount(instance.SkullHashes)
	explanation := fmt.Sprintf(
		"cleared fresh Pantheon (%s, %d feats, %d players) in %02d:%02d, implausible below %02d:%02d for a full run",
		stringOfVersion(instance.Version),
		featCount,
		instance.PlayerCount,
		instance.DurationSeconds/60,
		instance.DurationSeconds%60,
		outlierFloor/60,
		outlierFloor%60,
	)

	return prob, explanation
}

func mergePantheonDurationCheck(h ActivityHeuristic, instance *Instance, cheatedTimePrb float64, cheatedTimeExplanation string) (float64, string) {
	if h.ActivityId != 102 {
		return cheatedTimePrb, cheatedTimeExplanation
	}

	durationPrb, explanation := pantheonDurationCheck(instance)
	if durationPrb <= Threshold {
		return cheatedTimePrb, cheatedTimeExplanation
	}

	if cheatedTimeExplanation == "" {
		return durationPrb, explanation
	}

	return cumulativeProbability(cheatedTimePrb, durationPrb), cheatedTimeExplanation + ", " + explanation
}
