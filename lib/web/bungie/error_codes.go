package bungie

const (
	Success                      = 1
	SystemDisabled               = 5   // System is currently disabled
	InsufficientPrivileges       = 12  // Used when PGCRs are blocked by Bungie
	InvalidParameters            = 18  // Invalid input parameters
	GroupNotFound                = 686 // Clan not found
	CharacterNotFound            = 1620
	PGCRNotFound                 = 1653 // Standard 404 error for PGCRs
	DestinyThrottledByGameServer = 1672 // Throttled by game server (expected throttling)
	OtherError                   = 0    // Other/unknown errors (not a real Bungie code)
)
