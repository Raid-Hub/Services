package cheat_detection

import (
	"math"
	"time"
)

const (
	Any         = 0
	Standard    = 1
	Prestige    = 2
	GuidedGames = 3
	Master      = 4
)

func stringOfVersion(version int) string {
	switch version {
	case Any:
		return "Any"
	case Standard:
		return "Standard"
	case Prestige:
		return "Prestige"
	case GuidedGames:
		return "Guided Games"
	case Master:
		return "Master"
	default:
		return "Unknown"
	}
}

type DateRange struct {
	Start time.Time
	End   time.Time
}

type LowmanData struct {
	MinPlayers    int
	Range         []DateRange
	CheatedChance float64
	MinTime       time.Duration
}

type SpeedrunData struct {
	RecordTime time.Duration `json:"time"`
	RecordSet  time.Time     `json:"date"`
}

type ActivityHeuristic struct {
	ActivityId         int8
	RaidBit            uint64
	RaidName           string
	CheckpointName     string
	CheckpointLowman   map[int][]LowmanData
	FreshLowman        map[int][]LowmanData
	SpeedrunCurve      func(daysAfterRelease float64) float64
	MinFreshKills      int
	MinCheckpointKills int
}

var SalvationsEdgeHeuristic = ActivityHeuristic{
	ActivityId:     14,
	RaidBit:        SalvationsEdge,
	RaidName:       "Salvation's Edge",
	CheckpointName: "The Witness",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2024, time.July, 11, 9, 36, 31, 0, time.UTC), End: time.Date(2024, time.July, 23, 17, 0, 0, 0, time.UTC)},
					{Start: time.Date(2025, time.March, 28, 14, 0, 0, 0, time.UTC), End: time.Date(2099, time.December, 31, 17, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.20,
				MinTime:       9 * time.Minute,
			},
			{
				MinPlayers:    2,
				CheatedChance: 0.10,
				MinTime:       4 * time.Minute,
			},
		},
		Master: {
			{
				MinPlayers:    2,
				CheatedChance: 0.15,
				MinTime:       5 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers:    4,
				CheatedChance: 0.04,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 2102.45 - (58.14 * math.Log10(math.Pow(daysAfterRelease+1.00, 10.00)))
	},
	MinFreshKills:      875,
	MinCheckpointKills: 1, // glyph cheese is a thing
}

var PantheonHeuristic = ActivityHeuristic{
	ActivityId:     101,
	RaidBit:        Pantheon,
	RaidName:       "Pantheon",
	CheckpointName: "a Pantheon checkpoint",
	FreshLowman: map[int][]LowmanData{
		129: {
			{
				MinPlayers: 2,
			},
		},
		130: {
			{
				MinPlayers: 3,
			},
		},
		131: {
			{
				MinPlayers: 3,
			},
		},
	},
	SpeedrunCurve: func(_ float64) float64 {
		return 570
	},
	MinFreshKills:      335,
	MinCheckpointKills: 75,
}

var CrotasEndHeuristic = ActivityHeuristic{
	ActivityId:       13,
	RaidBit:          CrotasEnd,
	RaidName:         "Crota's End",
	CheckpointName:   "Crota",
	CheckpointLowman: map[int][]LowmanData{},
	FreshLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 2,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1288.97 - (38.84 * math.Log10(math.Pow(daysAfterRelease+1.00, 9.50)))
	},
	MinFreshKills:      760,
	MinCheckpointKills: 1, // finishers :(
}

var RootOfNightmaresHeuristic = ActivityHeuristic{
	ActivityId:       12,
	RaidBit:          RootOfNightmares,
	RaidName:         "Root of Nightmares",
	CheckpointName:   "Nezarec",
	CheckpointLowman: map[int][]LowmanData{},
	FreshLowman:      map[int][]LowmanData{},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 924.81 - (40.46 * math.Log10(math.Pow(daysAfterRelease+1.00, 4.11)))
	},
	MinFreshKills:      430,
	MinCheckpointKills: 33,
}

var KingsFallHeuristic = ActivityHeuristic{
	ActivityId:     11,
	RaidBit:        KingsFall,
	RaidName:       "King's Fall",
	CheckpointName: "Oryx",
	CheckpointLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers:    2,
				CheatedChance: 0.05,
				MinTime:       5 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 3,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1138.90 - (39.11 * math.Log10(math.Pow(daysAfterRelease+1.00, 3.94)))
	},
	MinFreshKills:      700,
	MinCheckpointKills: 105,
}

var VowOfTheDiscipleHeuristic = ActivityHeuristic{
	ActivityId:     10,
	RaidBit:        VowOfTheDisciple,
	RaidName:       "Vow of the Disciple",
	CheckpointName: "Rhulk",
	CheckpointLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 3,
				MinTime:    3 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 3,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1887.74 - (38.67 * math.Log10(math.Pow(daysAfterRelease+5.00, 8.05)))
	},
	MinFreshKills:      770,
	MinCheckpointKills: 125,
}

var VaultOfGlassHeuristic = ActivityHeuristic{
	ActivityId:     9,
	RaidBit:        VaultOfGlass,
	RaidName:       "Vault of Glass",
	CheckpointName: "Atheon",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2021, time.July, 14, 0, 0, 0, 0, time.UTC), End: time.Date(2021, time.December, 7, 0, 0, 0, 0, time.UTC)},
					{Start: time.Date(2023, time.October, 20, 0, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.15,
				MinTime:       5 * time.Minute,
			},
			{
				MinPlayers: 2,
				MinTime:    105 * time.Second, // 1 minute 45 seconds
			},
		},
		Master: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2024, time.February, 12, 0, 0, 0, 0, time.UTC), End: time.Date(2999, time.December, 7, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.20,
				MinTime:       2 * time.Minute,
			},
			{
				MinPlayers: 2,
				MinTime:    2 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2024, time.July, 14, 0, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.20,
			},
			{
				MinPlayers: 2,
			},
		},
		Master: {
			{
				MinPlayers:    2,
				CheatedChance: 0.04,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1310.42 - (36.37 * math.Log10(math.Pow(daysAfterRelease+1.00, 3.45)))
	},
	MinFreshKills:      1000,
	MinCheckpointKills: 40,
}

var DeepStoneCryptHeuristic = ActivityHeuristic{
	ActivityId:     8,
	RaidBit:        DeepStoneCrypt,
	RaidName:       "Deep Stone Crypt",
	CheckpointName: "Taniks",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2021, time.March, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2021, time.March, 31, 0, 0, 0, 0, time.UTC)},
					{Start: time.Date(2024, time.September, 12, 0, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.10,
				MinTime:       15 * time.Minute,
			},
			{
				MinPlayers: 2,
				MinTime:    5 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2024, time.September, 12, 0, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.20,
			},
			{
				MinPlayers: 2,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 918.75 - (43.74 * math.Log10(math.Pow(daysAfterRelease+1.00, 2.40)))
	},
	MinFreshKills:      500,
	MinCheckpointKills: 64,
}

var GardenOfSalvationHeuristic = ActivityHeuristic{
	ActivityId:     7,
	RaidBit:        GardenOfSalvation,
	RaidName:       "Garden of Salvation",
	CheckpointName: "The Sanctified Mind",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2023, time.April, 30, 0, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.15,
				MinTime:       10 * time.Minute,
			},
			{
				MinPlayers: 2,
				MinTime:    5 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 3,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1304.29 - (28.15 * math.Log10(math.Pow(daysAfterRelease+5.00, 9.56)))
	},
	MinFreshKills:      385,
	MinCheckpointKills: 120,
}

var CrownOfSorrowHeuristic = ActivityHeuristic{
	ActivityId:     6,
	RaidBit:        CrownOfSorrow,
	RaidName:       "Crown of Sorrow",
	CheckpointName: "Gahlran",
	CheckpointLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 2,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 2,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 829.30 - (45.43 * math.Log10(math.Pow(daysAfterRelease+1.00, 2.80)))
	},
	MinFreshKills:      425,
	MinCheckpointKills: 76,
}

var ScourgeOfThePastHeuristic = ActivityHeuristic{
	ActivityId:     5,
	RaidBit:        ScourgeOfThePast,
	RaidName:       "Scourge of the Past",
	CheckpointName: "Insurrection Prime",
	CheckpointLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 2,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		0: {
			{
				MinPlayers: 2,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 776.04 - (23.69 * math.Log10(math.Pow(daysAfterRelease+1.00, 8.14)))
	},
	MinFreshKills:      103,
	MinCheckpointKills: 27,
}

var LastWishHeuristic = ActivityHeuristic{
	ActivityId:     4,
	RaidBit:        LastWish,
	RaidName:       "Last Wish",
	CheckpointName: "Queenswalk",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2019, time.November, 25, 2, 0, 0, 0, time.UTC), End: time.Date(2020, time.November, 10, 17, 0, 0, 0, time.UTC)},
					{Start: time.Date(2021, time.September, 3, 19, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				MinTime: 2 * time.Minute,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2019, time.November, 25, 2, 0, 0, 0, time.UTC), End: time.Date(2020, time.November, 10, 17, 0, 0, 0, time.UTC)},
					{Start: time.Date(2021, time.September, 3, 19, 0, 0, 0, time.UTC), End: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC)},
				},
				CheatedChance: 0.05,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 645.33 - (38.31 * math.Log10(math.Pow(daysAfterRelease+1.00, 3.24)))
	},
	MinFreshKills:      18,
	MinCheckpointKills: 9,
}

var SpireOfStarsHeuristic = ActivityHeuristic{
	ActivityId:     3,
	RaidBit:        SpireOfStars,
	RaidName:       "Spire of Stars",
	CheckpointName: "Val Ca'uor",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 5,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 5,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1298.54 - (38.42 * math.Log10(math.Pow(daysAfterRelease+5.00, 7.16)))
	},
	MinFreshKills:      400,
	MinCheckpointKills: 0, // fotl ciphers instant complete cheese
}

var EaterOfWorldsHeuristic = ActivityHeuristic{
	ActivityId:     2,
	RaidBit:        EaterOfWorlds,
	RaidName:       "Eater of Worlds",
	CheckpointName: "Argos",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2018, time.August, 29, 3, 15, 0, 0, time.UTC), End: time.Date(2020, time.November, 10, 17, 0, 0, 0, time.UTC)},
				},
			},
		},
		Prestige: {
			{
				MinPlayers: 1,
				Range: []DateRange{
					{Start: time.Date(2018, time.October, 25, 18, 5, 0, 0, time.UTC), End: time.Date(2020, time.November, 10, 17, 0, 0, 0, time.UTC)},
				},
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 4,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 1607.05 - (34.00 * math.Log10(math.Pow(daysAfterRelease+1.00, 6.86)))
	},
	MinFreshKills:      325,
	MinCheckpointKills: 69,
}

var LeviathanHeuristic = ActivityHeuristic{
	ActivityId:     1,
	RaidBit:        Leviathan,
	RaidName:       "Leviathan",
	CheckpointName: "Calus",
	CheckpointLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 2,
			},
		},
	},
	FreshLowman: map[int][]LowmanData{
		Any: {
			{
				MinPlayers: 4,
			},
		},
	},
	SpeedrunCurve: func(daysAfterRelease float64) float64 {
		return 2028.95 - (52.55 * math.Log10(math.Pow(daysAfterRelease+5.00, 7.96)))
	},
	MinFreshKills:      300,
	MinCheckpointKills: 85,
}
