package cheat_detection

import (
	"database/sql"
	"fmt"
	"log"
)

const (
	CheatCheckVersion = "beta-2.1.11"
	Threshold         = 0.05
	PlayerThreshold   = 0.02
)

// This function should be idempotent such that it can be run multiple times without causing issues.
// Returns
// `instanceResult` which is the result of the instance check
// `flaggedPlayers` which is a list of players that were flagged
func CheckForCheats(instanceId int64, db *sql.DB) (*Instance, ResultTuple, []ResultTuple, bool, error) {
	instance, err := getInstance(instanceId, db)
	if err != nil {
		log.Printf("Error getting instance %d: %s", instanceId, err)
		return nil, ResultTuple{}, nil, false, err
	}

	heuristic, err := getActivityHeuristic(instance)
	if err != nil {
		return instance, ResultTuple{}, nil, false, err
	}

	instanceResult, playerResults := heuristic.apply(instance)
	isSolo := len(playerResults) == 1

	if instanceResult.Probability <= Threshold && len(playerResults) == 0 {
		return instance, instanceResult, nil, isSolo, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return instance, instanceResult, nil, isSolo, err
	}

	defer tx.Rollback()

	if instanceResult.Probability > Threshold {
		err = flagInstance(FlagInstance{
			InstanceId:        instance.InstanceId,
			CheatCheckVersion: CheatCheckVersion,
			CheatCheckBitmask: instanceResult.Reason,
			CheatProbability:  instanceResult.Probability,
			Explanation:       instanceResult.Explanation,
		}, tx)

		if err != nil {
			return instance, instanceResult, nil, isSolo, err
		}
	}

	flaggedPlayers := make([]ResultTuple, 0)
	for membershipId, result := range playerResults {
		if result.Probability > Threshold {
			err = flagPlayerInstance(FlagInstancePlayer{
				InstanceId:        instance.InstanceId,
				MembershipId:      membershipId,
				CheatCheckVersion: CheatCheckVersion,
				CheatCheckBitmask: result.Reason,
				CheatProbability:  result.Probability,
				Explanation:       result.Explanation,
			}, tx)
			if err != nil {
				return instance, instanceResult, flaggedPlayers, isSolo, err
			}
			flaggedPlayers = append(flaggedPlayers, result)
		}
	}

	err = tx.Commit()
	if err != nil {
		return instance, instanceResult, flaggedPlayers, isSolo, err
	}

	return instance, instanceResult, flaggedPlayers, isSolo, nil
}

func getActivityHeuristic(instance *Instance) (*ActivityHeuristic, error) {
	switch instance.Activity {
	case 1:
		return &LeviathanHeuristic, nil
	case 2:
		return &EaterOfWorldsHeuristic, nil
	case 3:
		return &SpireOfStarsHeuristic, nil
	case 4:
		return &LastWishHeuristic, nil
	case 5:
		return &ScourgeOfThePastHeuristic, nil
	case 6:
		return &CrownOfSorrowHeuristic, nil
	case 7:
		return &GardenOfSalvationHeuristic, nil
	case 8:
		return &DeepStoneCryptHeuristic, nil
	case 9:
		return &VaultOfGlassHeuristic, nil
	case 10:
		return &VowOfTheDiscipleHeuristic, nil
	case 11:
		return &KingsFallHeuristic, nil
	case 12:
		return &RootOfNightmaresHeuristic, nil
	case 13:
		return &CrotasEndHeuristic, nil
	case 101:
		return &PantheonHeuristic, nil
	case 14:
		return &SalvationsEdgeHeuristic, nil
	default:
		return nil, fmt.Errorf("no heuristic for activity %d", instance.Activity)
	}
}
