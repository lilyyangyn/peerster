package types

import "fmt"

// -----------------------------------------------------------------------------
// BCPrivateMessage

// NewEmpty implements types.Message.
func (m BCPrivateMessage) NewEmpty() Message {
	return &BCPrivateMessage{}
}

// Name implements types.Message.
func (m BCPrivateMessage) Name() string {
	return "blockchainPrivate"
}

// String implements types.Message.
func (m BCPrivateMessage) String() string {
	return fmt.Sprintf("{blockchainPrivate for %s}",
		m.Recipients)
}

// HTML implements types.Message.
func (m BCPrivateMessage) HTML() string {
	return m.String()
}

// -----------------------------------------------------------------------------
// BCTxnMessag

// NewEmpty implements types.Message.
func (m BCTxnMessag) NewEmpty() Message {
	return &BCTxnMessag{}
}

// Name implements types.Message.
func (m BCTxnMessag) Name() string {
	return "blockchainTxn"
}

// String implements types.Message.
func (m BCTxnMessag) String() string {
	return fmt.Sprintf("{blockchainTxn %s - %s}",
		m.Origin, m.Txn.HashBytes())
}

// HTML implements types.Message.
func (m BCTxnMessag) HTML() string {
	return m.String()
}

// -----------------------------------------------------------------------------
// BCBlkMessage

// NewEmpty implements types.Message.
func (m BCBlkMessage) NewEmpty() Message {
	return &BCBlkMessage{}
}

// Name implements types.Message.
func (m BCBlkMessage) Name() string {
	return "blockchainBlk"
}

// String implements types.Message.
func (m BCBlkMessage) String() string {
	return fmt.Sprintf("{blockchainBlk %s - %s (h=%d)}",
		m.Origin, m.BlkHeader.Hash(), m.BlkHeader.Height)
}

// HTML implements types.Message.
func (m BCBlkMessage) HTML() string {
	return m.String()
}

// -----------------------------------------------------------------------------
// BCAskSyncMessage

// NewEmpty implements types.Message.
func (m BCAskSyncMessage) NewEmpty() Message {
	return &BCAskSyncMessage{}
}

// Name implements types.Message.
func (m BCAskSyncMessage) Name() string {
	return "blockchainSync"
}

// String implements types.Message.
func (m BCAskSyncMessage) String() string {
	return fmt.Sprintf("{blockchainAskSync %s-%d}",
		m.Origin, m.LatestHeight)
}

// HTML implements types.Message.
func (m BCAskSyncMessage) HTML() string {
	return m.String()
}

// -----------------------------------------------------------------------------
// BCSyncMessage

// NewEmpty implements types.Message.
func (m BCSyncMessage) NewEmpty() Message {
	return &BCSyncMessage{}
}

// Name implements types.Message.
func (m BCSyncMessage) Name() string {
	return "blockchainSync"
}

// String implements types.Message.
func (m BCSyncMessage) String() string {
	return fmt.Sprintf("{blockchainSync from %s}",
		m.Origin)
}

// HTML implements types.Message.
func (m BCSyncMessage) HTML() string {
	return m.String()
}
