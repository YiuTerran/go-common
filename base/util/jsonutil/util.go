package jsonutil

import "encoding/json"

func ToRawJson(obj any) *json.RawMessage {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	p := json.RawMessage(bytes)
	return &p
}
