package bungie

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
