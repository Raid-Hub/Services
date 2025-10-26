package player

import (
	"raidhub/lib/utils"
)

var PlayerLogger utils.Logger

func init() {
	PlayerLogger = utils.NewLogger("player")
}
