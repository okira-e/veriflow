package utils

import (
	"crypto/rand"
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
