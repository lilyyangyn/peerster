package message

import (
	"sync"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/types"
)

// SafeRoutingTable implements a thread-safe routing table
type SafeRoutingTable struct {
	*sync.RWMutex
	table peer.RoutingTable
}

func (t *SafeRoutingTable) add(key string, val string) {
	t.Lock()
	defer t.Unlock()
	t.table[key] = val
}
func (t *SafeRoutingTable) update(key string, val string) {
	t.Lock()
	defer t.Unlock()

	oldVal, ok := t.table[key]
	if ok && oldVal == key {
		return
	}
	t.table[key] = val
}
func (t *SafeRoutingTable) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.table, key)
}
func (t *SafeRoutingTable) get(key string) (string, bool) {
	t.RLock()
	val, ok := t.table[key]
	t.RUnlock()
	return val, ok
}
func (t *SafeRoutingTable) getAll() peer.RoutingTable {
	routingTable := peer.RoutingTable{}
	t.RLock()
	for key, value := range t.table {
		routingTable[key] = value
	}
	t.RUnlock()
	return routingTable
}
func NewSafeRoutingTable(addr string) *SafeRoutingTable {
	rt := SafeRoutingTable{&sync.RWMutex{}, peer.RoutingTable{}}
	rt.add(addr, addr)
	return &rt
}

// --------------------------------------------------------

type RumorsTable map[string][]types.Rumor

// SafeRumorsTable implements a thread-safe rumor table
type SafeRumorsTable struct {
	*sync.RWMutex
	table RumorsTable
}

func (t *SafeRumorsTable) add(rumor types.Rumor) bool {
	t.Lock()
	defer t.Unlock()

	if uint(len(t.table[rumor.Origin]))+1 != rumor.Sequence {
		return false
	}
	t.table[rumor.Origin] = append(t.table[rumor.Origin], rumor)
	return true
}
func (t *SafeRumorsTable) getExpectedSeq(key string) uint {
	t.RLock()
	rumors := t.table[key]
	t.RUnlock()
	return uint(len(rumors)) + 1
}
func (t *SafeRumorsTable) getRumorsFrom(key string, seqID uint) ([]types.Rumor, bool) {
	rumors := []types.Rumor{}
	t.RLock()
	length := uint(len(t.table[key]))
	if seqID > length {
		return rumors, false
	}
	for i := seqID - 1; i < length; i++ {
		rumors = append(rumors, t.table[key][i])
	}
	t.RUnlock()
	return rumors, true
}
func (t *SafeRumorsTable) getStatus() map[string]uint {
	statusTable := make(map[string]uint)
	t.RLock()
	for key, value := range t.table {
		statusTable[key] = uint(len(value))
	}
	t.RUnlock()
	return statusTable
}
func NewSafeRumorsTable() *SafeRumorsTable {
	rt := SafeRumorsTable{&sync.RWMutex{}, RumorsTable{}}
	return &rt
}

// --------------------------------------------------------

type TimerTable map[string]chan struct{}

// TimerController implements a thread-safe table for timer
type TimerController struct {
	*sync.RWMutex
	table TimerTable
}

func (t *TimerController) add(pktID string, done chan struct{}) {
	t.Lock()
	defer t.Unlock()
	t.table[pktID] = done
}
func (t *TimerController) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.table, key)
}
func (t *TimerController) getAndRemove(key string) (chan struct{}, bool) {
	t.Lock()
	val, ok := t.table[key]
	delete(t.table, key)
	t.Unlock()
	return val, ok
}
func NewTimeController() *TimerController {
	rt := TimerController{&sync.RWMutex{}, TimerTable{}}
	return &rt
}

// --------------------------------------------------------

// PubkeyController implements a thread-safe table to store public key of peers
type PubkeyController struct {
	*sync.RWMutex
	table peer.PubkeyStore
}

func (t *PubkeyController) add(peer string, pubkey *types.Pubkey) {
	t.Lock()
	defer t.Unlock()
	if _, ok := t.table[peer]; !ok {
		t.table[peer] = *pubkey
	}
}
func (t *PubkeyController) remove(key string) {
	t.Lock()
	defer t.Unlock()
	delete(t.table, key)
}
func (t *PubkeyController) get(peer string) (types.Pubkey, bool) {
	t.RLock()
	defer t.RUnlock()
	pubkey, ok := t.table[peer]
	return pubkey, ok
}
func (t *PubkeyController) getAll() peer.PubkeyStore {
	pubkeyStore := peer.PubkeyStore{}
	t.RLock()
	for key, value := range t.table {
		pubkeyStore[key] = value
	}
	t.RUnlock()
	return pubkeyStore
}
func NewPubkeyController(self string, selfkey *types.Pubkey) *PubkeyController {
	rt := PubkeyController{&sync.RWMutex{}, peer.PubkeyStore{self: *selfkey}}
	return &rt
}
