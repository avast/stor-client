package storclient

import (
	"sync"

	"github.com/avast/hashutil-go"
)

type currentDownloads struct {
	lock    sync.RWMutex
	hashmap map[string]interface{}
}

// Del delete hash from actualdownloads
func (a *currentDownloads) Del(hash hashutil.Hash) {
	a.lock.Lock()
	defer a.lock.Unlock()

	delete(a.hashmap, hash.String())
}

// ContainsOrAdd check if hash is in actualdownloads and if not added him
// returns true if are hash added
func (a *currentDownloads) ContainsOrAdd(hash hashutil.Hash) bool {
	a.lock.Lock()
	defer a.lock.Unlock()

	if !a.contains(hash) {
		a.add(hash)
		return true
	}

	return false
}

func (a *currentDownloads) contains(hash hashutil.Hash) bool {
	_, ok := a.hashmap[hash.String()]
	return ok
}

func (a *currentDownloads) add(hash hashutil.Hash) {
	if a.hashmap == nil {
		a.hashmap = make(map[string]interface{})
	}

	a.hashmap[hash.String()] = nil
}
