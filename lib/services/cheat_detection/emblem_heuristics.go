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

// Legendary emblems with high lift on flagged runs in prod analysis (cheat_level >= 2).
var suspiciousLegendaryEmblemHashes = map[uint32]struct{}{
	298334059:  {}, // Inherent Truth
	4178714191: {}, // Timeline's Blade
	2565108501: {}, // After the Unknown
	4178714190: {}, // Third Unknown
	1530147650: {}, // Blade's Blast
	3992231371: {}, // External Sights
	46275857:   {}, // Walker's Warp
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
