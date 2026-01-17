package bungie

const (
	Success                      = 1
	UnhandledException           = 3    // Transient error - retryable
	SystemDisabled               = 5    // System is currently disabled
	ParameterInvalidRange        = 8    // Parameter outside valid range (e.g., invalid membership ID)
	InsufficientPrivileges       = 12   // Used when PGCRs are blocked by Bungie
	InvalidParameters            = 18   // Invalid input parameters
	GroupNotFound                = 686  // Clan not found
	DestinyAccountNotFound       = 1601 // Account not found
	CharacterNotFound            = 1620
	PGCRNotFound                 = 1653 // Standard 404 error for PGCRs
	DestinyPrivacyRestriction    = 1665 // Privated resource
	DestinyThrottledByGameServer = 1672 // Throttled by game server (expected throttling)
	OtherError                   = 0    // Other/unknown errors (not a real Bungie code)
)

const (
	ModeRaid  = 4
	ModeStory = 2
)

// Bungie membership type constants
// Reference: BungieMembershipType enum
const (
	MembershipTypePSN    = 1
	MembershipTypeXbox   = 2
	MembershipTypeSteam  = 3
	MembershipTypeStadia = 5
	MembershipTypeEpic   = 6
)

// AllViableMembershipTypes lists platform types we actively try when resolving a profile
var AllViableMembershipTypes = []int{
	MembershipTypePSN,
	MembershipTypeXbox,
	MembershipTypeSteam,
	MembershipTypeStadia,
	MembershipTypeEpic,
}
