package mpc

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"go.dedis.ch/cs438/types"
)

// --------------------------------------------------------
// ValueDB

// ValueDB stores values that can be used in MPC
// Asset is the value know by the peer, temp is the value we save for MPC.
// temp need to refresh for each new MPC round.
type ValueDB struct {
	*sync.RWMutex
	asset map[string]int
}

func (db *ValueDB) addAsset(key string, value int) bool {
	db.Lock()
	defer db.Unlock()
	db.asset[key] = value
	return true
}
func (db *ValueDB) getAsset(key string) (int, bool) {
	db.RLock()
	defer db.RUnlock()
	value, ok := db.asset[key]
	return value, ok
}

func NewValueDB() *ValueDB {
	db := ValueDB{
		&sync.RWMutex{},
		map[string]int{},
	}
	return &db
}

// --------------------------------------------------------
// MPCCenter

type MPCCenter struct {
	*sync.RWMutex
	nofitication map[string]chan MPCResult
	store        map[string]*MPC
	conds        map[string]*sync.Cond
}

func NewMPCCenter() *MPCCenter {
	return &MPCCenter{
		RWMutex:      &sync.RWMutex{},
		nofitication: map[string]chan MPCResult{},
		store:        map[string]*MPC{},
		conds:        map[string]*sync.Cond{},
	}
}

func (c *MPCCenter) GetMPC(id string) *MPC {
	var mpc *MPC
	c.Lock()
	for {
		m, ok := c.store[id]
		if ok {
			mpc = m
			break
		}

		cond, ok := c.conds[id]
		if !ok {
			cond = sync.NewCond(c.RWMutex)
		}
		cond.Wait()
	}
	c.Unlock()

	return mpc
}

func (c *MPCCenter) RegisterMPC(id string, mpc *MPC) {
	c.Lock()
	defer c.Unlock()

	// add old values to the mpc instance
	oldMPC, ok := c.store[id]
	if ok {
		mpc.addValues(oldMPC.interStore)
	}
	c.store[id] = mpc

	// register notification
	if _, ok := c.nofitication[id]; !ok {
		c.nofitication[id] = make(chan MPCResult, 2)
	}

	// notify if anyone is block waiting
	cond, ok := c.conds[id]
	if ok {
		cond.Broadcast()
	}
}

func (c *MPCCenter) AddValue(id string, key string, value big.Int) {
	c.Lock()
	defer c.Unlock()

	mpc, ok := c.store[id]
	if !ok {
		mpc := NewMPC(id, big.Int{}, "", "")
		c.store[id] = mpc
	}
	mpc.addValue(key, value)
}

func (c *MPCCenter) InformMPCStart(id string) {
	c.RLock()
	defer c.RUnlock()

	channel, ok := c.nofitication[id]
	if !ok {
		return
	}
	channel <- MPCResult{}
}

func (c *MPCCenter) InformMPCComplete(id string, result MPCResult) (err error) {
	c.RLock()
	defer c.RUnlock()

	mpc, ok := c.store[id]
	if ok {
		err = mpc.finalize(result)
	}
	channel, ok := c.nofitication[id]
	if !ok {
		return
	}
	channel <- result

	return err
}

func (c *MPCCenter) Listen(id string, timeout time.Duration) MPCResult {
	c.Lock()
	// first check if MPC already have done
	if mpc, ok := c.store[id]; ok {
		if result, err := mpc.getResult(); err == nil {
			c.Unlock()
			return *result
		}
	}

	channel, ok := c.nofitication[id]
	if !ok {
		channel = make(chan MPCResult)
		c.nofitication[id] = channel
	}
	c.Unlock()

	select {
	case <-time.After(timeout):
		return MPCResult{result: 0, err: fmt.Errorf("MPC Timeout")}
	case <-channel:
		result := <-channel

		c.Lock()
		defer c.Unlock()

		delete(c.nofitication, id)
		return result
	}

}

// --------------------------------------------------------
// PubkeyStore

type PubkeyStore struct {
	*sync.RWMutex
	store map[string]*rsa.PublicKey
}

func NewPubkeyStore() *PubkeyStore {
	return &PubkeyStore{
		RWMutex: &sync.RWMutex{},
		store:   make(map[string]*rsa.PublicKey),
	}
}

func (s *PubkeyStore) Get(id string) (*types.Pubkey, bool) {
	s.RLock()
	defer s.RUnlock()

	key, ok := s.store[id]
	return (*types.Pubkey)(key), ok
}

func (s *PubkeyStore) Add(raw map[string]string) error {
	s.Lock()
	defer s.Unlock()

	failed := make([]string, 0)

	for addr, pubEncode := range raw {
		// now do not support pubkey change
		_, ok := s.store[addr]
		if ok {
			continue
		}

		pubBytes, err := hex.DecodeString(pubEncode)
		if err != nil {
			failed = append(failed, addr)
			continue
		}
		pubkey, err := x509.ParsePKIXPublicKey(pubBytes)
		if err != nil {
			failed = append(failed, addr)
			continue
		}
		rsaPubkey, ok := pubkey.(*rsa.PublicKey)
		if !ok {
			failed = append(failed, addr)
			continue
		}
		s.store[addr] = rsaPubkey
	}

	if len(failed) > 0 {
		return fmt.Errorf("fail to parse encryption pubkey: %s", failed)
	}
	return nil
}
