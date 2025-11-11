package messages

import (
	"raidhub/lib/dto"
	"raidhub/lib/web/bungie"
)

// PGCRStoreMessage matches lib/messaging/queue-workers/instance_store.go
type PGCRStoreMessage struct {
	Instance dto.Instance                        `json:"instance"`
	PGCR     bungie.DestinyPostGameCarnageReport `json:"pgcr"`
}

// NewPGCRStoreMessage creates a new PGCR store message
func NewPGCRStoreMessage(instance *dto.Instance, pgcr *bungie.DestinyPostGameCarnageReport) PGCRStoreMessage {
	return PGCRStoreMessage{
		Instance: *instance,
		PGCR:     *pgcr,
	}
}
