package extra

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl"
	"go.dedis.ch/cs438/registry/standard"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/channel"
)

var peerFac peer.Factory = impl.NewPeer

var channelFac transport.Factory = channel.NewTransport

var defaultRegistry = standard.NewRegistry()
