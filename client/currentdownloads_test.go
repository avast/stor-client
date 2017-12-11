package storclient

import (
	"crypto/md5"
	"testing"

	"github.com/avast/hashutil-go"
	"github.com/stretchr/testify/assert"
)

func TestCurrentDownloads(t *testing.T) {
	var cur currentDownloads

	hash := hashutil.EmptyHash(md5.New())

	assert.True(t, cur.ContainsOrAdd(hash))
	assert.False(t, cur.ContainsOrAdd(hash))
	cur.Del(hash)
	assert.True(t, cur.ContainsOrAdd(hash))
	assert.False(t, cur.ContainsOrAdd(hash))
}
