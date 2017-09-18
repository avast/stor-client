package storclient

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

type Sha256 []byte

var badLength = errors.New("Sha256 must have 32 bytes length")

func (sha Sha256) String() string {
	return hex.EncodeToString(sha)
}

func StringToSha256(str string) (Sha256, error) {
	bytes, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}

	if len(bytes) != sha256.Size {
		return nil, badLength
	}

	return bytes, nil
}

func BytesToSha256(bytes []byte) (Sha256, error) {
	if len(bytes) != sha256.Size {
		return nil, badLength
	}

	for _, b := range bytes {
		if b > 0xff {
			return nil, errors.New("Sha256 must contains only 0-FF bytes")
		}
	}

	return bytes, nil
}
