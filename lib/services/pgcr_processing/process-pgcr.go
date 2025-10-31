package pgcr_processing

import (
	"errors"
	"fmt"
	"raidhub/lib/dto"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"time"
)

var logger = logging.NewLogger("PGCR_PROCESSING_SERVICE")

type PGCRResult int

const RAID_ACTIVITY_MODE = 4

// PGCRResult is the result of the PGCR processing
const (
	Success                PGCRResult = 1
	NonRaid                PGCRResult = 2
	NotFound               PGCRResult = 3
	SystemDisabled         PGCRResult = 4
	InsufficientPrivileges PGCRResult = 5
	BadFormat              PGCRResult = 6
	// InternalError          PGCRResult = 7  DEPRECATED
	// DecodingError          PGCRResult = 8  DEPRECATED
	ExternalError PGCRResult = 9
	RateLimited   PGCRResult = 10
)

// From an id, fetch the PGCR and process it into an Instance, returning the result, the instance, and the raw PGCR
func FetchAndProcessPGCR(instanceID int64) (PGCRResult, *dto.Instance, *bungie.DestinyPostGameCarnageReport) {
	result, rawPGCR := FetchPGCR(instanceID)
	if result != Success {
		return result, nil, nil
	}

	// Check if this is a raid activity
	if rawPGCR.ActivityDetails.Mode != RAID_ACTIVITY_MODE {
		return NonRaid, nil, rawPGCR
	}

	pgcr, isExpectedError, err := parsePGCRToInstance(rawPGCR)

	if err != nil {
		fields := map[string]any{
			logging.ERROR:       err.Error(),
			logging.INSTANCE_ID: instanceID,
		}
		if !isExpectedError {
			logger.Error("PGCR_PARSING_EXCEPTION", fields)
		} else {
			logger.Warn("PGCR_PARSING_ERROR", fields)
		}
		return BadFormat, nil, nil
	}

	return Success, pgcr, rawPGCR
}

func parsePGCRToInstance(report *bungie.DestinyPostGameCarnageReport) (*dto.Instance, bool, error) {
	startDate, err := time.Parse(time.RFC3339, report.Period)
	if err != nil {
		return nil, false, err
	}

	expectedEntryCount := getStat(report.Entries[0].Values, "playerCount")
	actualEntryCount := len(report.Entries)
	if expectedEntryCount >= 0 && actualEntryCount != expectedEntryCount {
		return nil, true, fmt.Errorf("malformed pgcr: invalid entry length: %d != %d", actualEntryCount, expectedEntryCount)
	}

	noOnePlayed := true
	for _, e := range report.Entries {
		if getStat(e.Values, "activityDurationSeconds") != 0 {
			noOnePlayed = false
			break
		}
	}
	if noOnePlayed {
		return nil, false, errors.New("malformed pgcr: no one had any duration_seconds")
	}

	completionReason := getStat(report.Entries[0].Values, "completionReason")

	result := dto.Instance{
		InstanceId: report.ActivityDetails.InstanceId,
		Hash:       report.ActivityDetails.DirectorActivityHash,
		// assigned later
		Fresh:           nil,
		DateStarted:     startDate,
		DateCompleted:   CalculateDateCompleted(startDate, report.Entries[0]),
		DurationSeconds: CalculateDurationSeconds(startDate, report.Entries[0]),
		MembershipType:  report.ActivityDetails.MembershipType,
		Score:           getStat(report.Entries[0].Values, "teamScore"),
		SkullHashes:     []uint32{},
	}

	if report.SelectedSkullHashes != nil {
		setOfSkullHashes := make(map[uint32]bool)
		for _, hash := range *report.SelectedSkullHashes {
			setOfSkullHashes[hash] = true
		}
		for hash := range setOfSkullHashes {
			result.SkullHashes = append(result.SkullHashes, hash)
		}
	}

	players := make(map[int64][]bungie.DestinyPostGameCarnageReportEntry)

	for _, e := range report.Entries {
		if val, ok := players[e.Player.DestinyUserInfo.MembershipId]; ok {
			players[e.Player.DestinyUserInfo.MembershipId] = append(val, e)
		} else {
			players[e.Player.DestinyUserInfo.MembershipId] = []bungie.DestinyPostGameCarnageReportEntry{e}
		}
	}

	var processedPlayerActivities []dto.InstancePlayer
	for _, entries := range players {
		processedPlayerActivity := dto.InstancePlayer{
			Characters: []dto.InstanceCharacter{},
			Player: dto.PlayerInfo{
				FirstSeen: startDate,
			},
		}

		for _, entry := range entries {
			character := dto.InstanceCharacter{
				CharacterId: entry.CharacterId,
				Completed:   getStat(entry.Values, "completed") == 1,
				Weapons:     []dto.InstanceCharacterWeapon{},
			}
			if entry.Player.ClassHash != 0 {
				character.ClassHash = new(uint32)
				*character.ClassHash = entry.Player.ClassHash
			}
			if entry.Player.EmblemHash != 0 {
				character.EmblemHash = new(uint32)
				*character.EmblemHash = entry.Player.EmblemHash
			}

			character.Score = getStat(entry.Values, "score")
			character.Kills = getStat(entry.Values, "kills")
			character.Deaths = getStat(entry.Values, "deaths")
			character.Assists = getStat(entry.Values, "assists")
			character.TimePlayedSeconds = getStat(entry.Values, "timePlayedSeconds")
			character.StartSeconds = getStat(entry.Values, "startSeconds")
			if entry.Extended != nil {
				character.PrecisionKills = getStat(entry.Extended.Values, "precisionKills")
				character.SuperKills = getStat(entry.Extended.Values, "weaponKillsSuper")
				character.GrenadeKills = getStat(entry.Extended.Values, "weaponKillsGrenade")
				character.MeleeKills = getStat(entry.Extended.Values, "weaponKillsMelee")

				for _, weapon := range entry.Extended.Weapons {
					processedWeapon := dto.InstanceCharacterWeapon{
						WeaponHash: weapon.ReferenceId,
					}
					processedWeapon.Kills = getStat(weapon.Values, "uniqueWeaponKills")
					processedWeapon.PrecisionKills = getStat(weapon.Values, "uniqueWeaponPrecisionKills")
					character.Weapons = append(character.Weapons, processedWeapon)
				}
			}

			processedPlayerActivity.Characters = append(processedPlayerActivity.Characters, character)

			processedPlayerActivity.Finished = processedPlayerActivity.Finished || (character.Completed && completionReason == 0)
		}

		processedPlayerActivity.TimePlayedSeconds = calculatePlayerTimePlayedSeconds(entries)

		destinyUserInfo := entries[0].Player.DestinyUserInfo

		processedPlayerActivity.Player.LastSeen = startDate.Add(time.Duration(
			processedPlayerActivity.Characters[0].StartSeconds+
				processedPlayerActivity.Characters[0].TimePlayedSeconds,
		) * time.Second)
		processedPlayerActivity.Player.MembershipId = destinyUserInfo.MembershipId
		if destinyUserInfo.MembershipType != 0 {
			processedPlayerActivity.Player.MembershipType = &destinyUserInfo.MembershipType
			processedPlayerActivity.Player.IconPath = destinyUserInfo.IconPath
			processedPlayerActivity.Player.DisplayName = destinyUserInfo.DisplayName

			if destinyUserInfo.BungieGlobalDisplayNameCode != nil {
				processedPlayerActivity.Player.BungieGlobalDisplayNameCode = bungie.FixBungieGlobalDisplayNameCode(destinyUserInfo.BungieGlobalDisplayNameCode)

				if destinyUserInfo.BungieGlobalDisplayName != nil && *destinyUserInfo.BungieGlobalDisplayName != "" {
					processedPlayerActivity.Player.BungieGlobalDisplayName = destinyUserInfo.BungieGlobalDisplayName
				}
			}
		}

		processedPlayerActivities = append(processedPlayerActivities, processedPlayerActivity)
	}

	result.Players = processedPlayerActivities
	result.PlayerCount = len(players)

	result.Completed = false
	for _, e := range processedPlayerActivities {
		if e.Finished {
			result.Completed = true
			break
		}
	}

	deathless := true
	for _, e := range processedPlayerActivities {
		for _, c := range e.Characters {
			if c.Deaths > 0 {
				deathless = false
				break
			}
		}
		if !deathless {
			break
		}
	}

	fresh, err := isFresh(report, deathless)
	if err != nil {
		return nil, false, err
	}
	result.Fresh = fresh

	if result.Completed && deathless {
		result.Flawless = fresh
	} else {
		result.Flawless = new(bool) // false
	}

	return &result, false, nil
}

func getStat(values map[string]bungie.DestinyHistoricalStatsValue, key string) int {
	if stat, ok := values[key]; ok {
		return int(stat.Basic.Value)
	} else {
		return 0
	}
}

func calculatePlayerTimePlayedSeconds(characters []bungie.DestinyPostGameCarnageReportEntry) int {
	activityDurationSeconds := getStat(characters[0].Values, "activityDurationSeconds")
	timeline := make([]int, activityDurationSeconds+1)
	for _, character := range characters {
		startSecond := getStat(character.Values, "startSeconds")
		timePlayedSeconds := getStat(character.Values, "timePlayedSeconds")
		endSecond := startSecond + timePlayedSeconds

		if startSecond <= activityDurationSeconds {
			timeline[startSecond]++
		}

		if endSecond <= activityDurationSeconds {
			timeline[endSecond]--
		}
	}

	durationSeconds := 0
	currentCharacters := 0
	for _, val := range timeline {
		currentCharacters += val
		if currentCharacters > 0 {
			durationSeconds++
		}
	}

	return durationSeconds
}

func CalculateDurationSeconds(startDate time.Time, entry bungie.DestinyPostGameCarnageReportEntry) int {
	return getStat(entry.Values, "activityDurationSeconds")
}

func CalculateDateCompleted(startDate time.Time, entry bungie.DestinyPostGameCarnageReportEntry) time.Time {
	seconds := getStat(entry.Values, "activityDurationSeconds")
	return startDate.Add(time.Duration(seconds) * time.Second)
}

var (
	beyondLightStart = time.Date(2020, time.November, 10, 9, 0, 0, 0, time.FixedZone("PST", -8*60*60)).Unix()
	witchQueenStart  = time.Date(2022, time.February, 22, 9, 0, 0, 0, time.FixedZone("PST", -8*60*60)).Unix()
	hauntedStart     = time.Date(2022, time.May, 24, 10, 0, 0, 0, time.FixedZone("PDT", -7*60*60)).Unix()
)

var leviHashes = map[uint32]bool{
	2693136600: true, 2693136601: true, 2693136602: true,
	2693136603: true, 2693136604: true, 2693136605: true,
	89727599: true, 287649202: true, 1699948563: true, 1875726950: true,
	3916343513: true, 4039317196: true, 417231112: true, 508802457: true,
	757116822: true, 771164842: true, 1685065161: true, 1800508819: true,
	2449714930: true, 3446541099: true, 4206123728: true, 3912437239: true,
	3879860661: true, 3857338478: true,
}

// isFresh checks if a DestinyPostGameCarnageReportData is considered fresh based on the period start time.
func isFresh(pgcr *bungie.DestinyPostGameCarnageReport, deathless bool) (*bool, error) {
	var result *bool = nil

	start, err := time.Parse(time.RFC3339, pgcr.Period)
	if err != nil {
		logger.Warn("Error parsing 'period'", map[string]any{logging.INSTANCE_ID: pgcr.ActivityDetails.InstanceId, logging.ERROR: err.Error()})
		return nil, err
	}

	startUnix := start.Unix()

	if startUnix >= hauntedStart {
		// Current case, working as normal, using ActivityWasStartedFromBeginning
		result = &pgcr.ActivityWasStartedFromBeginning
	} else if startUnix < beyondLightStart {
		// Pre beyond light, using StartingPhaseIndex
		result = new(bool)
		// sotp
		if pgcr.ActivityDetails.DirectorActivityHash == 548750096 || pgcr.ActivityDetails.DirectorActivityHash == 2812525063 {
			*result = (pgcr.StartingPhaseIndex <= 1)
			// levi
		} else if leviHashes[pgcr.ActivityDetails.DirectorActivityHash] {
			*result = (pgcr.StartingPhaseIndex == 0 || pgcr.StartingPhaseIndex == 2)
		} else {
			*result = (pgcr.StartingPhaseIndex == 0)
		}
	} else if startUnix >= witchQueenStart && (pgcr.ActivityWasStartedFromBeginning || deathless) {
		// WQ: ActivityWasStartedFromBeginning erroneously false when a wipe happens
		result = &pgcr.ActivityWasStartedFromBeginning
	}
	// Beyond Light: ActivityWasStartedFromBeginning always false

	return result, nil
}
