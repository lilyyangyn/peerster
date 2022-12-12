package datashare

import (
	"sync"

	"go.dedis.ch/cs438/peer"
)

// SafeCatalog implements a thread-safe catalog table
type SafeCatalog struct {
	*sync.RWMutex
	catalog           peer.Catalog
	fullyKnwonCatalog peer.Catalog
}

func (c *SafeCatalog) add(key string, val string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.catalog[key]; ok {
		c.catalog[key][val] = struct{}{}
	} else {
		c.catalog[key] = map[string]struct{}{val: {}}
	}
}
func (c *SafeCatalog) addFullyKnown(name string, peer string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.fullyKnwonCatalog[name]; ok {
		c.fullyKnwonCatalog[name][peer] = struct{}{}
	} else {
		c.fullyKnwonCatalog[name] = map[string]struct{}{peer: {}}
	}
}
func (c *SafeCatalog) forEachFullyKnown(action func(key string, value map[string]struct{}) bool) {
	c.RLock()
	defer c.RUnlock()
	for key, value := range c.fullyKnwonCatalog {
		ok := action(key, value)
		if !ok {
			return
		}
	}
}
func (c *SafeCatalog) getAll() peer.Catalog {
	catalog := peer.Catalog{}
	c.RLock()
	for key, value := range c.catalog {
		innerMap := make(map[string]struct{}, len(value))
		for innerKey := range value {
			innerMap[innerKey] = struct{}{}
		}
		catalog[key] = innerMap
	}
	c.RUnlock()
	return catalog
}
func NewSafeCatalog() *SafeCatalog {
	catalog := SafeCatalog{&sync.RWMutex{}, peer.Catalog{}, peer.Catalog{}}
	return &catalog
}

// SafeChannTable implements a thread-safe channel table
type SafeChannTable struct {
	*sync.RWMutex
	channels map[string]*chan bool
}

func (t SafeChannTable) add(key string, val *chan bool) {
	t.Lock()
	defer t.Unlock()
	t.channels[key] = val
}
func (t SafeChannTable) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.channels, key)
}
func (t *SafeChannTable) get(key string) (*chan bool, bool) {
	t.RLock()
	val, ok := t.channels[key]
	t.RUnlock()
	return val, ok
}
func NewSafeChannTable() *SafeChannTable {
	channels := SafeChannTable{&sync.RWMutex{}, map[string]*chan bool{}}
	return &channels
}

// SafeMsgRecord implements a thread-safe message table
type SafeMsgRecord struct {
	*sync.RWMutex
	messages map[string]struct{}
}

func (t SafeMsgRecord) add(key string) bool {
	t.Lock()
	defer t.Unlock()
	_, ok := t.messages[key]
	t.messages[key] = struct{}{}
	return ok
}
func NewSafeMsgRecord() *SafeMsgRecord {
	records := SafeMsgRecord{&sync.RWMutex{}, map[string]struct{}{}}
	return &records
}
