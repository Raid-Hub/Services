package messages

// CharacterFillMessage matches lib/messaging/queue-workers/character_fill.go
type CharacterFillMessage struct {
	MembershipId int64 `json:"membershipId,string"`
	CharacterId  int64 `json:"characterId,string"`
	InstanceId   int64 `json:"instanceId,string"`
}

// NewCharacterFillMessage creates a new character fill message
func NewCharacterFillMessage(membershipId, characterId, instanceId int64) CharacterFillMessage {
	return CharacterFillMessage{
		MembershipId: membershipId,
		CharacterId:  characterId,
		InstanceId:   instanceId,
	}
}
