package pgcr

import (
	"raidhub/lib/utils"
)

var PGCRLogger utils.Logger

func init() {
	PGCRLogger = utils.NewLogger("pgcr")
}
