package paxos

import (
	"github.com/rs/xid"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// NewTagPaxos creates a new PaxosInstance that handles tag consensus
func NewTagPaxos(m *PaxosModule) *PaxosInstance {
	p := *NewPaxosInstance(m)
	p.callback = m.tagCallback
	p.threshold = m.tagThreshold
	p.lastBlockKey = storage.LastBlockKey

	return &p
}

// InitTagConensus inits a paxos to consensus on tag value
func (m *PaxosInstance) InitTagConensus(name string, mh string) (err error) {
	if m.Type != types.PaxosTypeTag {
		return xerrors.Errorf("invalid operation")
	}

	if val := m.conf.Storage.GetNamingStore().Get(name); len(val) > 0 {
		return xerrors.Errorf("%s already in the name store.", name)
	}

	m.Lock()
	if !m.Occupied {
		m.Occupied = true
		m.Proposer = true
		m.paxosPromiseChan = make(chan paxosResult, 3)
		step := m.TLC
		m.Unlock()
		return m.proposeTag(name, mh, step)
	}
	m.cond.Wait()
	m.Unlock()
	return m.InitTagConensus(name, mh)
}

/** Private Helpfer Functions **/

// proposeTag starts a new paxos starting from phase one
func (m *PaxosInstance) proposeTag(name string, mh string, step uint) error {
	proposeValContent := types.PaxosTagValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	}
	proposeVal, err := types.CreatePaxosValue(proposeValContent)
	if err != nil {
		return err
	}

	result := m.startFromPhaseOne(proposeVal, step)
	content, err := types.ParsePaxosValueContent(result.Value)
	if err != nil {
		return err
	}
	tagValue, ok := content.(*types.PaxosTagValue)
	if !ok {
		return xerrors.Errorf("wrong type: %T", content)
	}

	if tagValue.Filename == name && tagValue.Metahash == mh {
		return nil
	}
	return m.InitTagConensus(name, mh)
}

// tagThreshold calculates the threshold to enter next paxos stage
func (m *PaxosModule) tagThreshold() int {
	return m.conf.PaxosThreshold(m.conf.TotalPeers)
}

// tagCallback is a callback function that gets called when TLC advances
func (m *PaxosModule) tagCallback(value *types.PaxosValue) error {
	content, err := types.ParsePaxosValueContent(value)
	if err != nil {
		return err
	}
	tagval, ok := content.(*types.PaxosTagValue)
	if !ok {
		return xerrors.Errorf("paxosvalue wrong type: %T", content)
	}

	// set naming store and notify proposer
	m.conf.Storage.GetNamingStore().Set(tagval.Filename, []byte(tagval.Metahash))
	return nil
}
