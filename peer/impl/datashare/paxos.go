package datashare

import (
	"github.com/rs/xid"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// initTagConcensus inits a paxos to consensus on tag value
func (m *DataSharingModule) initTagConcensus(name string, mh string) (err error) {
	if m.Type != types.PaxosTypeTag {
		return xerrors.Errorf("invalid operation")
	}

	if val := m.conf.Storage.GetNamingStore().Get(name); len(val) > 0 {
		return xerrors.Errorf("%s already in the name store.", name)
	}

	if step, ok := m.CheckAndWait(); ok {
		return m.proposeTag(name, mh, step)
	}
	return m.initTagConcensus(name, mh)
}

/** Private Helpfer Functions **/

// proposeTag starts a new paxos starting from phase one
func (m *DataSharingModule) proposeTag(name string, mh string, step uint) error {
	proposeValContent := types.PaxosTagValue{
		UniqID:   xid.New().String(),
		Filename: name,
		Metahash: mh,
	}
	proposeVal, err := types.CreatePaxosValue(proposeValContent)
	if err != nil {
		return err
	}

	result := m.StartFromPhaseOne(proposeVal, step)
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
	return m.initTagConcensus(name, mh)
}

// tagThreshold calculates the threshold to enter next paxos stage
func (m *DataSharingModule) tagThreshold() int {
	return m.conf.PaxosThreshold(m.conf.TotalPeers)
}

// tagCallback is a callback function that gets called when TLC advances
func (m *DataSharingModule) tagCallback(value *types.PaxosValue) error {
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
