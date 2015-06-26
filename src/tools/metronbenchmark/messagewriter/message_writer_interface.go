package messagewriter

import (
	"fmt"
	"time"
)

func formatMsg(roundId uint, sequence uint, t time.Time) []byte {
	return []byte(fmt.Sprintf("msg:%d:%d:%d:\n", roundId, sequence, t.UnixNano()))
}
