package store

import "encoding/json"

type record struct {
	Op    string `json:"op"` // "SET" or "DELETE"
	Key   string `json:"key"`
	Value string `json:"value,omitempty"` // empty for DELETE
}

// encodeRecord marshals a record to JSON. No trailing newline —
// WAL.Append adds that.
func encodeRecord(r record) ([]byte, error) {
	return json.Marshal(r)
}

// decodeRecord parses a JSON line into a record.
func decodeRecord(line []byte) (record, error) {
	var r record
	err := json.Unmarshal(line, &r)
	return r, err
}
