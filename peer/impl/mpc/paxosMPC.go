package mpc

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)

func newMPCModuleWithPaxos(conf *peer.Configuration, messageModule *message.MessageModule,
	paxosModule *paxos.PaxosModule) *MPCModule {
	m := newMPCModule(conf, messageModule)
	m.consensusType = peer.MPCConsensusPaxos
	instance, err := paxosModule.CreateNewPaxos(
		types.PaxosTypeMPC,
		storage.MPCLastBlockKey,
		m.mpcThreshold,
		m.mpcCallback,
	)
	if err != nil {
		panic(err)
	}
	m.paxos = instance

	return m
}

// CalculatePaxos start a new MPC from making consensus on budget and expression.
// It will then initiate the MPC automatically
func (m *MPCModule) CalculatePaxos(expression string, budget float64) (int, error) {
	if m.conf.TotalPeers == 1 {
		log.Println("No MPC. Direct calculate the result.")
		return 0, nil
	}

	// // generate random prime, seed is set in advance when the node starts
	// // err: msg too long for RSA key size
	// prime, err := generateRandomPrime(1024)
	// if err != nil {
	// 	return -1, err
	// }
	prime := "1000000009"
	uniqID := xid.New().String()
	err := m.initMPCConcensus(uniqID, budget, expression, prime)
	if err != nil {
		return -1, err
	}

	//channel wait mpc result
	result := m.mpcCenter.Listen(uniqID, time.Duration(math.MaxInt32))

	return result.result, result.err
}

func (m *MPCModule) InitMPCWithPaxos(uniqID string, prime string, initiator string,
	expression string) error {
	if m.consensusType != peer.MPCConsensusPaxos {
		return fmt.Errorf("MPC does not use paxos to consensus")
	}
	mpcPrime, _ := new(big.Int).SetString(prime, 10)
	mpc := NewMPC(uniqID, *mpcPrime, initiator, expression)

	// Use public key as participants
	pubKeyStore := m.GetPubkeyStore()
	if int(m.conf.TotalPeers) > len(pubKeyStore) {
		return fmt.Errorf("%s: not received everyone's public key, require %d, have %d",
			m.conf.Socket.GetAddress(), m.conf.TotalPeers, len(pubKeyStore))
	}
	participants := make([]string, 0, len(pubKeyStore))
	for key := range pubKeyStore {
		participants = append(participants, key)
	}

	// add MPC peer, use port as the mpc id.
	peersMap := map[string]int{}
	for _, participant := range participants {
		peerIDstr := strings.Split(participant, ":")[1]
		peerID, _ := strconv.Atoi(peerIDstr)
		peersMap[participant] = peerID
	}
	mpc.addPeers(peersMap)
	mpc.addParticipants(participants)

	m.mpcCenter.RegisterMPC(mpc.id, mpc)

	return nil
}

// -----------------------------------------------------------------------------
// Private Helpfer Functions

// sendShareMessagePaxos sends the share secret in encrypted message
func (m *MPCModule) sendShareMessagePaxos(uniqID string, peer string, id int, key string, value big.Int) error {
	shareMsg := types.MPCShareMessage{
		ReqID: uniqID,
		Value: types.MPCSecretValue{
			Owner: m.getIdentifyKey(),
			Key:   key,
			Value: value.Text(10),
		},
	}
	shareMsgMarshal, err := m.CreateMsg(shareMsg)
	if err != nil {
		return err
	}
	return m.SendEncryptedMessage(shareMsgMarshal, peer)
}

func (m *MPCModule) boardcastInterpolationResultPaxos(result big.Int, mpc *MPC) error {
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
	privMsg := types.PrivateMessage{
		Recipients: privRecipients,
		Msg:        &interpolationMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return err
	}
	return m.Broadcast(privMsgMarshal)
}
