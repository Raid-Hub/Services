package bungie

import (
	"strconv"
	"strings"
)

func FixBungieGlobalDisplayNameCode(code *int) *string {
	if code == nil {
		return nil
	} else {
		str := strconv.Itoa(*code)
		missingZeroes := 4 - len(str)

		var returnValue string = strings.Repeat("0", missingZeroes) + str
		return &returnValue
	}
}
