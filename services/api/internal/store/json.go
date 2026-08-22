package store

import "encoding/json"

func unmarshalJSON(b []byte, dest any) error {
	return json.Unmarshal(b, dest)
}
