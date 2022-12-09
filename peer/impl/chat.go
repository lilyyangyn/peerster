package impl

import (
	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type ChatModule struct {
	*Node
}

func NewChatModule(n *Node) *ChatModule {
	m := ChatModule{
		Node: n,
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, m.ProcessChatMessage)

	return &m
}

/** Message Handler **/

// ProcessChatMessage is a callback function to handle received chat message
func (m *ChatModule) ProcessChatMessage(msg types.Message, pkt transport.Packet) error {
	// cast the message to its actual type. You assume it is the right type.
	chatMsg, ok := msg.(*types.ChatMessage)
	if !ok {
		return xerrors.Errorf("wrong type: %T", msg)
	}

	log.Info().Msgf("%s received a chat message from: %s. Msg: %s",
		m.conf.Socket.GetAddress(),
		pkt.Header.Source,
		chatMsg.Message)
	return nil
}
