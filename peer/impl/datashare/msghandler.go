package datashare

import (
	"regexp"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// ProcessDataRequestMessage is a callback function to handle received data request message
func (m *DataSharingModule) ProcessDataRequestMessage(msg types.Message, pkt transport.Packet) error {
	requestMsg, ok := msg.(*types.DataRequestMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	cid := requestMsg.Key
	data := m.conf.Storage.GetDataBlobStore().Get(cid)
	err := m.sendDataReplyMessage(pkt.Header.Source, requestMsg.RequestID, cid, data)

	return err
}

// ProcessDataReplyMessage is a callback function to handle received data reply message
func (m *DataSharingModule) ProcessDataReplyMessage(msg types.Message, pkt transport.Packet) error {
	replyMsg, ok := msg.(*types.DataReplyMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	if len(replyMsg.Value) > 0 {
		// update local storage
		m.conf.Storage.GetDataBlobStore().Set(replyMsg.Key, replyMsg.Value)
	}
	if channel, ok := m.replyChannels.get(replyMsg.RequestID); ok {
		*channel <- true
	}

	return nil
}

// ProcessSearchRequestMessage is a callback function to handle received search request message
func (m *DataSharingModule) ProcessSearchRequestMessage(msg types.Message, pkt transport.Packet) error {
	requestMsg, ok := msg.(*types.SearchRequestMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	// check duplication
	if m.messageRecords.add(requestMsg.RequestID) {
		return nil
	}

	// forward search
	reg, err := regexp.Compile(requestMsg.Pattern)
	if err != nil {
		return err
	}
	// check timeout
	budget := requestMsg.Budget - 1
	if budget > 0 {
		err = m.requestRemoteNames(*reg, requestMsg.Origin, budget, requestMsg.RequestID, 0)
		if err != nil {
			return err
		}
	}

	// construct file info
	fileinfos := make([]types.FileInfo, 0)
	regMatch := func(key string, val []byte) bool {
		if reg.MatchString(key) {
			metahash := string(val)
			if fileinfo, ok := m.getLocalFileInfo(key, metahash); ok {
				fileinfos = append(fileinfos, fileinfo)
			}
		}
		return true
	}
	m.conf.Storage.GetNamingStore().ForEach(regMatch)

	// send reply
	err = m.sendSearchReplyMessage(requestMsg.Origin, pkt.Header.RelayedBy, requestMsg.RequestID, fileinfos)
	if err != nil {
		return err
	}

	return err
}

// ProcessSearchReplyMessage is a callback function to handle received search reply message
func (m *DataSharingModule) ProcessSearchReplyMessage(msg types.Message, pkt transport.Packet) error {
	replyMsg, ok := msg.(*types.SearchReplyMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	for _, fileinfo := range replyMsg.Responses {
		// update naming store
		m.conf.Storage.GetNamingStore().Set(fileinfo.Name, []byte(fileinfo.Metahash))
		// update catalog
		m.catalog.add(fileinfo.Metahash, pkt.Header.Source)
		isFullyKnown := true
		for _, chunkCID := range fileinfo.Chunks {
			if chunkCID == nil {
				isFullyKnown = false
				continue
			}
			m.catalog.add(string(chunkCID), pkt.Header.Source)
		}
		if isFullyKnown {
			m.catalog.addFullyKnown(fileinfo.Name, pkt.Header.Source)
		}
	}
	if channel, ok := m.replyChannels.get(replyMsg.RequestID); ok {
		*channel <- true
	}

	return nil
}
