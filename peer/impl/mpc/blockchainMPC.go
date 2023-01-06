package mpc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/blockchain"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/types"
)

func newMPCModuleWithBlockchain(conf *peer.Configuration, messageModule *message.MessageModule,
	bcModule *blockchain.BlockchainModule) *MPCModule {
	m := newMPCModule(conf, messageModule)
	m.consensusType = peer.MPCConsensusBC
	m.bcModule = bcModule
	m.pubkeyStore = NewPubkeyStore()

	return m
}

// CalculateBlockchain sends a PreMPC txn to the blockchain
// It will initiate a paxos to start the MPC once it notice the txn is included in the chain
func (m *MPCModule) CalculateBlockchain(expression string, budget float64) (int, error) {
	id, err := m.bcModule.SendPreMPCTransaction(expression, budget, "")
	if err != nil {
		return 0, err
	}
	log.Info().Msgf("send preMPC txn %s", id)

	result := m.mpcCenter.Listen(id)

	return result.result, result.err
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions

func (m *MPCModule) initMPCWithBlockchain(uniqID string, config *permissioned.ChainConfig,
	propose *permissioned.MPCPropose) error {
	mpcPrime, _ := new(big.Int).SetString(propose.Prime, 10)
	mpc := NewMPC(uniqID, *mpcPrime, propose.Initiator, propose.Expression)

	// Use chain participants as MPC participants
	participants := make([]string, 0, len(config.Participants))
	for peer := range config.Participants {
		participants = append(participants, peer)
	}
	sort.Strings(participants)

	// add MPC peer, use position as the mpc id.
	peersMap := map[string]int{}
	for idx, participant := range participants {
		peersMap[participant] = idx
	}
	mpc.addPeers(peersMap)
	mpc.addParticipants(participants)

	m.mpcCenter.RegisterMPC(mpc.id, mpc)

	return nil
}

func (m *MPCModule) boardcastInterpolationResultBlockchain(result big.Int, mpc *MPC) error {
	// boardcast the result and compute the final result
	interpolationMsg := types.MPCInterpolationMessage{
		ReqID: mpc.id,
		Owner: m.conf.Socket.GetAddress(),
		Value: result.Text(10),
	}
	interpolationMsgMarshal, err := m.CreateMsg(interpolationMsg)
	if err != nil {
		return err
	}
	// wrap in private msg
	privRecipients := map[string]struct{}{}
	for _, participant := range mpc.getParticipants() {
		privRecipients[participant] = struct{}{}
	}
	privMsg := types.BCPrivateMessage{
		Recipients: privRecipients,
		Msg:        &interpolationMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}
	return m.Broadcast(privMsgMarshal)
}

// sendShareMessagePaxos sends the share secret in encrypted message
func (m *MPCModule) sendShareMessageBlockchain(uniqID string, peer string, id int, key string, value big.Int) error {
	shareMsg := types.MPCShareMessage{
		ReqID: uniqID,
		Value: types.MPCSecretValue{
			Owner: m.conf.Socket.GetAddress(),
			Key:   key,
			Value: value.Text(10),
		},
	}
	shareMsgMarshal, err := m.CreateMsg(shareMsg)
	if err != nil {
		return err
	}

	// encrypt message
	ptxt, err := json.Marshal(&shareMsgMarshal)
	if err != nil {
		return err
	}
	pubkey, ok := m.pubkeyStore.Get(peer)
	if !ok {
		return fmt.Errorf("no public key for peer %s", peer)
	}
	encryptedMsg, err := m.EncryptAsymetric(ptxt, *pubkey)
	if err != nil {
		return err
	}
	encryptedMsgMarshal, err :=
		m.CreateMsg((*types.EncryptedMessage)(&encryptedMsg))
	if err != nil {
		return err
	}

	// wrap in private msg
	privMsg := types.BCPrivateMessage{
		Recipients: map[string]struct{}{peer: {}},
		Msg:        &encryptedMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}

	// send in rumor
	return m.Broadcast(privMsgMarshal)
}
