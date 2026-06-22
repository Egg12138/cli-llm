package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func hashEntry(entry Entry) (string, error) {
	entry.ID = ""
	encoded, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])[:12], nil
}
