package storage

import (
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

type Copyable interface {
	Copy() Copyable
}

type Hashable interface {
	Hash() string
}

type KVStore interface {
	Get(key string) (interface{}, bool)
	Put(key string, value interface{}) error
	Del(key string) error
	For(func(key string, value interface{}) error) error
	Copy() KVStore
	Hash() []byte
}

type BasicKV struct {
	KVStore

	store map[string]interface{}
}

func NewBasicKV() *BasicKV {
	return &BasicKV{
		store: make(map[string]interface{}),
	}
}

func (kv *BasicKV) Get(key string) (interface{}, bool) {
	value, ok := kv.store[key]
	return value, ok
}

func (kv *BasicKV) Put(key string, value interface{}) error {
	kv.store[key] = value
	return nil
}

func (kv *BasicKV) Del(key string) error {
	delete(kv.store, key)
	return nil
}

func (kv *BasicKV) For(action func(key string, value interface{}) error) error {
	for k, v := range kv.store {
		err := action(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (kv *BasicKV) Copy() KVStore {
	cp := NewBasicKV()
	for k, v := range kv.store {
		switch vv := v.(type) {
		case Copyable:
			cp.Put(k, vv.Copy())
		default:
			cp.Put(k, v)
		}
	}
	return cp
}

func (kv *BasicKV) Hash() []byte {
	sorted := make([]string, 0, len(kv.store))
	for k := range kv.store {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	h := crypto.SHA256.New()
	for _, key := range sorted {
		v, ok := kv.store[key]
		if !ok {
			continue
		}
		h.Write([]byte(key))

		switch vv := v.(type) {
		case Hashable:
			hash := vv.Hash()
			h.Write([]byte(hash))
		default:
			hash := Hash(vv)
			h.Write([]byte(hash))
		}
	}

	return h.Sum(nil)
}

func Hash(value interface{}) string {
	h := sha256.New()
	bytes, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	h.Write(bytes)

	return hex.EncodeToString(h.Sum(nil))
}
