package mpc

import "go.dedis.ch/cs438/peer/impl"

type MPCModule struct {
	*impl.Node
}

func NewMPCModule(n *impl.Node) *MPCModule {
	m := MPCModule{
		Node: n,
	}

	// message registery

	return &m
}
