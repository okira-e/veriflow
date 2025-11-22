package utils

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
)

func NewId() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789" // base36

	id := make([]byte, 10)
	for i := range id {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			panic("crypto/rand failed to generate a random ID: " + err.Error())
		}
		id[i] = alphabet[num.Int64()]
	}
	return string(id)
}

func PrettyJson(s []byte) (string, error) {
	var v any
	if err := json.Unmarshal(s, &v); err != nil {
		return "", err
	}

	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out), nil
}
