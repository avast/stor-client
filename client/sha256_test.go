package storclient

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/assert"
)

const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestSha256(t *testing.T) {
	s1, err := StringToSha256(emptyHash)
	assert.NoError(t, err, "Conversion string to Sha256 without errors")

	s2, err := StringToSha256("E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855")
	assert.NoError(t, err, "Conversion string to Sha256 without errors")

	assert.Equal(t, s1, s2, "Upper and lower of same Sha256 are equal")

	h := sha256.New()
	assert.Equal(t, s1, (Sha256)(h.Sum(nil)), "Empty hash calculated by crypto library are equal to hash from string")

	assert.Equal(t, s1.String(), emptyHash, "Converse Sha256 to strings")

	_, err = StringToSha256("")
	assert.Error(t, err, "Empty string isn't valid Sha256")

	_, err = StringToSha256("X3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B855")
	assert.Error(t, err, "X isn't valid char in Sha256")
}
