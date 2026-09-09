package mailer

import "encoding/json"

func unmarshal(b []byte, into any) error {
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, into)
}
