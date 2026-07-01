package cheat_detection

import "fmt"

const maxEmblemCheatProbability = 0.10

// Default class emblems (Common, v300.emblems.basic). Overrepresented on cheaters
// but also on legitimate new accounts — only apply when other player signals are already present.
var defaultClassEmblemHashes = map[uint32]struct{}{
	1907674137: {}, // Warlock's Flight
	1907674138: {}, // Hunter's Wit
	1907674139: {}, // Titan's Pride
}

// suspiciousLegendaryEmblemHashList is from prod analysis (cheat_level >= 2, flagged runs,
// last 12 months, 3-month instance chunks × last_seen buckets): emblem_hash with
// >= 3 cheater wearers and >= 80× lift vs clean players on flagged runs.
var suspiciousLegendaryEmblemHashList = []uint32{
	46275857,
	54004488,
	54004492,
	131450128,
	185321778,
	298334059,
	298334060,
	298334061,
	383734232,
	383734233,
	621113310,
	690263480,
	690263482,
	690263483,
	707041058,
	707041059,
	707041064,
	707041066,
	707041068,
	707041069,
	707041070,
	723818646,
	723818648,
	723818649,
	723818651,
	723818652,
	748692000,
	748692001,
	748692002,
	787024996,
	788073488,
	844563491,
	866034298,
	866034301,
	908153537,
	908153540,
	1059304051,
	1063872195,
	1063872196,
	1063872197,
	1063872198,
	1138508276,
	1465090516,
	1465090517,
	1511214613,
	1530147650,
	1611948522,
	1714370698,
	1868330223,
	1885107795,
	1901885382,
	1983519830,
	2010554579,
	2026109717,
	2054118356,
	2069797998,
	2069797999,
	2071635914,
	2071635915,
	2227664598,
	2390666069,
	2420153991,
	2484637936,
	2484637939,
	2510169794,
	2535664168,
	2535664170,
	2565108496,
	2565108497,
	2565108500,
	2565108501,
	2565108502,
	2565108503,
	2565108508,
	2680217521,
	2680217522,
	2680217523,
	2680217527,
	2680217529,
	2790542793,
	2847579026,
	2847579027,
	2962546544,
	2962546549,
	2962546551,
	2967682030,
	3282118732,
	3338748564,
	3373303016,
	3508476924,
	3508476927,
	3564452901,
	3778092977,
	3800278196,
	3800278197,
	3800278198,
	3828080585,
	3888032083,
	3888032090,
	3888032093,
	3903070390,
	3903070392,
	3992231361,
	3992231368,
	3992231369,
	3992231370,
	3992231371,
	3992231372,
	4133455812,
	4150233535,
	4178714180,
	4178714187,
	4178714189,
	4178714190,
	4178714191,
	4183788692,
	4183788693,
	4183788697,
}

var suspiciousLegendaryEmblemHashes map[uint32]struct{}

func init() {
	suspiciousLegendaryEmblemHashes = make(map[uint32]struct{}, len(suspiciousLegendaryEmblemHashList))
	for _, hash := range suspiciousLegendaryEmblemHashList {
		suspiciousLegendaryEmblemHashes[hash] = struct{}{}
	}
}

func playerEmblemHashes(player Player) []uint32 {
	seen := make(map[uint32]struct{})
	var hashes []uint32
	for _, char := range player.Characters {
		if char.EmblemHash == nil || *char.EmblemHash == 0 {
			continue
		}
		hash := *char.EmblemHash
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		hashes = append(hashes, hash)
	}
	return hashes
}

// emblemCheatProbability returns a small additive cheat probability from worn emblems.
// existingPlayerPrb is the player's probability before emblem adjustment (used to gate default-class emblems).
func emblemCheatProbability(player Player, existingPlayerPrb float64) (float64, string) {
	var boost float64
	var matchedHash uint32

	for _, hash := range playerEmblemHashes(player) {
		if _, ok := suspiciousLegendaryEmblemHashes[hash]; ok {
			if boost < 0.06 {
				boost = 0.06
				matchedHash = hash
			}
			continue
		}
		if _, ok := defaultClassEmblemHashes[hash]; ok && existingPlayerPrb > PlayerThreshold {
			if boost < 0.03 {
				boost = 0.03
				matchedHash = hash
			}
		}
	}

	if boost == 0 {
		return 0, ""
	}
	if boost > maxEmblemCheatProbability {
		boost = maxEmblemCheatProbability
	}
	return boost, fmt.Sprintf("suspicious emblem %d", matchedHash)
}
