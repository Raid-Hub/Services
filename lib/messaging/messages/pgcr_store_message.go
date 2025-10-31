package messages

import (
	"raidhub/lib/dto"
	"raidhub/lib/web/bungie"
)

// PGCRStoreMessage matches lib/messaging/queue-workers/instance_store.go
type PGCRStoreMessage struct {
	Activity dto.Instance                        `json:"activity"`
	Raw      bungie.DestinyPostGameCarnageReport `json:"raw"`
}

// NewPGCRStoreMessage creates a new PGCR store message
func NewPGCRStoreMessage(activity *dto.Instance, raw *bungie.DestinyPostGameCarnageReport) PGCRStoreMessage {
	return PGCRStoreMessage{
		Activity: *activity,
		Raw:      *raw,
	}
}
