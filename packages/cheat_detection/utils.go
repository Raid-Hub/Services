package cheat_detection

import (
	"fmt"
	"math"
)

// assumes solo if called
func resultsAdjustedIfSolo(instance *Instance, instanceResult ResultTuple, playerResults map[int64]ResultTuple) (ResultTuple, map[int64]ResultTuple) {
	if instance.PlayerCount == 1 && len(playerResults) == 1 {
		var player ResultTuple
		for _, p := range playerResults {
			player = p
			break
		}
		maxPrb := cumulativeProbability(instanceResult.Probability, player.Probability)

		combinedExplanation := ""
		if instanceResult.Explanation == "" {
			combinedExplanation = fmt.Sprintf("Solo, [%s]", player.Explanation)
		} else if player.Explanation == "" {
			combinedExplanation = instanceResult.Explanation
		} else {
			combinedExplanation = fmt.Sprintf("Solo, %s, [%s]", instanceResult.Explanation, player.Explanation)
		}

		return ResultTuple{
				Probability: maxPrb,
				Explanation: combinedExplanation,
				Reason:      player.Reason | instanceResult.Reason | Solo,
			}, map[int64]ResultTuple{player.MembershipId: {
				MembershipId: player.MembershipId,
				Probability:  maxPrb,
				Explanation:  combinedExplanation,
				Reason:       player.Reason | instanceResult.Reason | Solo,
			}}
	} else {
		return instanceResult, playerResults
	}
}

func nilResult() (float64, uint64, string) {
	return 0, 0, ""
}

func cumulativeProbability(probabilities ...float64) float64 {
	prbNo := 1.0
	for _, p := range probabilities {
		prbNo *= (1 - min(1, p))
	}
	return max(0, 1-prbNo)
}

func prbAtLeastTwo(allProbs []float64) float64 {
	// get the probability that 2 events happened
	// formula: P(at least 2) = 1 - P(none) - P(exactly 1)

	// P(none) = product(1 - P(i)) for all i
	p0 := 1.0
	for _, prob := range allProbs {
		p0 *= (1 - prob)
	}

	// P(exactly 1) = sum(P(i) * P(none of others))
	p1 := 0.0
	for i, pi := range allProbs {
		pNoneOfOthers := 1.0
		for j, pj := range allProbs {
			if j != i {
				pNoneOfOthers *= (1 - pj)
			}
		}
		p1 += pi * pNoneOfOthers
	}

	return 1 - p0 - p1
}

func buildLogisticCurve(k, x0 float64) func(x float64) float64 {
	return func(x float64) float64 {
		return 1 / (1 + math.Exp(k*(x-x0)))
	}
}

var (
	killsCurve                            = buildLogisticCurve(-0.15, 40)
	grenadeCurve                          = buildLogisticCurve(-0.35, 20)
	superCurve                            = buildLogisticCurve(-0.75, 6)
	heavyCurve                            = buildLogisticCurve(-13, 0.8)
	completionTimeCurve                   = buildLogisticCurve(40, 0.93)
	killsShareCurve                       = buildLogisticCurve(-7, 1.48)
	totalInstanceKillsCurve               = buildLogisticCurve(20, 0.83)
	totalInstanceKillsSecondaryCurve      = buildLogisticCurve(-0.04, 60)
	notCompletedAdjustmentCurve           = buildLogisticCurve(0.0052, 220)
	timeDilationCurve                     = buildLogisticCurve(12, 0.6)
	varLowmanCheckpointDurationRatioCurve = buildLogisticCurve(15, 0.82)
)

func participationCurve(playerCount int, playerTotalKills int, totalInstanceKills int, playerTimePlayedSeconds int, totalTimeForAllPlayers int) float64 {
	if playerCount == 1 {
		return 0
	}

	participationComparedToExpectedRatio := math.Pow((float64(playerTotalKills)/float64(totalInstanceKills+1)), 1.25) / (float64(playerTimePlayedSeconds) / float64(totalTimeForAllPlayers+1))
	participationPercentage := (float64(playerTotalKills+1) / float64(totalInstanceKills+1)) * (float64(min(playerTotalKills, 50.0)) / 50.0)

	adjPcount := min(max(1, playerCount), 8)
	k := -1.6 - (0.35 * float64(6-adjPcount))
	x0 := 3.0 + (-0.45 * float64(6-adjPcount))
	return buildLogisticCurve(k, x0)(max(0, participationComparedToExpectedRatio) * (2.5 * participationPercentage))
}
