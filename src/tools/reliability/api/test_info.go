package api

import (
	"bytes"
	"time"
)

// Test is used to decode the body from a request.
type Test struct {
	ID     int64  `json:"id"`
	Cycles uint64 `json:"cycles"`
	// How many writes an individual worker does. This value changes depending
	// on the worker.
	WriteCycles uint64    `json:"write_cycles"`
	Delay       Duration  `json:"delay"`
	Timeout     Duration  `json:"timeout"`
	StartTime   time.Time `json:"start_time"`
}

// Duration is a time.Duration that implements json.Unmarshal and
// json.Marshal.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaller
func (d *Duration) UnmarshalJSON(b []byte) error {
	val := bytes.Trim(b, `"`)
	dur, err := time.ParseDuration(string(val))

	if err != nil {
		return err
	}

	*d = Duration(dur)

	return nil
}

// MarshalJSON implements json.Unmarshaller
func (d *Duration) MarshalJSON() ([]byte, error) {
	return []byte("\"" + (time.Duration)(*d).String() + "\""), nil
}
