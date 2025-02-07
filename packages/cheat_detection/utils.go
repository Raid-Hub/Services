package cheat_detection

import (
	"math"
)

func distributeProbabilities(players []Player, maxProb float64, reason uint64, explanation string) map[int64]ResultTuple {
	var totalTPSScore float64 = 0
	for _, player := range players {
		score := float64(player.TimePlayedSeconds)
		if player.Finished {
			score *= 2
		}
		totalTPSScore += score
	}

	results := make(map[int64]ResultTuple)
	for _, player := range players {
		score := float64(player.TimePlayedSeconds) / totalTPSScore
		if player.Finished {
			score *= 2
		}
		if score > 1 {
			score = 1
		}
		results[player.MembershipId] = ResultTuple{
			MembershipId: player.MembershipId,
			Probability:  maxProb * math.Sqrt(score),
			Explanation:  explanation,
			Reason:       reason,
		}
	}

	return results
}

func resultForSharedFate(probability float64, reason uint64, explanation string, players []Player) (ResultTuple, map[int64]ResultTuple, bool) {
	return ResultTuple{
		Probability: probability,
		Explanation: explanation,
		Reason:      reason,
	}, distributeProbabilities(players, probability, reason, explanation), true
}

func resultsAdjustedIfSolo(playerCount int, probability float64, reason uint64, explanation string, playerResults map[int64]ResultTuple) (ResultTuple, map[int64]ResultTuple) {
	if playerCount == 1 && len(playerResults) == 1 {
		maxPrb := probability
		var player ResultTuple
		for _, p := range playerResults {
			player = p
			break
		}
		maxPrb = max(maxPrb, player.Probability)
		player.Probability = maxPrb

		return ResultTuple{
				Probability: maxPrb,
				Explanation: player.Explanation + " & " + explanation,
				Reason:      reason,
			}, map[int64]ResultTuple{player.MembershipId: {
				MembershipId: player.MembershipId,
				Probability:  maxPrb,
				Explanation:  player.Explanation + " & " + explanation,
				Reason:       player.Reason,
			}}
	} else {
		return ResultTuple{
			Probability: probability,
			Explanation: explanation,
			Reason:      reason,
		}, playerResults
	}
}

func nilResult() (ResultTuple, map[int64]ResultTuple, bool) {
	return ResultTuple{}, map[int64]ResultTuple{}, false
}
