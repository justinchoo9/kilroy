package engine

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

func NewRunID() (string, error) {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
