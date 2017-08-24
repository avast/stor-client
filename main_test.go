package main

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestReadShaFromReader(t *testing.T) {
	expected := []string{
		"01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b",
		"edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb",
	}
	var x = `
nosha
01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b
a/b/c/edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb
`
	r := strings.NewReader(x)

	shas := readShaFromReader(r)

	got := make([]string, 0)
	for sha := range shas {
		got = append(got, sha)
	}

	assert.Equal(t, expected, got)
}