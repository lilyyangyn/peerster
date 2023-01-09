package mpc

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/blockchain"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/permissioned-chain"
	"go.dedis.ch/cs438/types"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueDB   *ValueDB
	mpcCenter *MPCCenter

	consensusType peer.MPCConsensus

	pubkeyStore *PubkeyStore
	bcModule    *blockchain.BlockchainModule

	paxos *paxos.PaxosInstance
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule,
	paxosModule *paxos.PaxosModule,
	bcModule *blockchain.BlockchainModule) *MPCModule {

	switch conf.MPCType {
	case peer.MPCConsensusPaxos:
		return newMPCModuleWithPaxos(conf, messageModule, paxosModule)
	case peer.MPCConsensusBC:
		return newMPCModuleWithBlockchain(conf, messageModule, bcModule)
	}
	panic("invalid MPC type")
}

func newMPCModule(conf *peer.Configuration, messageModule *message.MessageModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
		valueDB:       NewValueDB(),
		mpcCenter:     NewMPCCenter(),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCShareMessage{}, m.ProcessMPCShareMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCInterpolationMessage{}, m.ProcessMPCInterpolationMsg)

	return &m
}

/** Feature Functions **/

func (m *MPCModule) Calculate(expression string, budget float64) (int, error) {
	switch m.consensusType {
	case peer.MPCConsensusPaxos:
		return m.CalculatePaxos(expression, budget)
	case peer.MPCConsensusBC:
		return m.CalculateBlockchain(expression, budget)
	}
	panic("invalid MPC type")
}

func (m *MPCModule) SetValueDBAsset(key string, value int, price float64) error {
	myIdentity := m.getIdentifyKey()
	assetsMap := m.GetPeerAssetPrices()

	for peer, assets := range assetsMap {
		if peer == myIdentity {
			continue
		}
		_, found := assets[key]
		if found {
			return fmt.Errorf("add Assets failed. key %s duplicate, peer %s already have the same assets", key, peer)
		}
	}

	ok := m.valueDB.addAsset(key, value)
	if !ok {
		return fmt.Errorf("add Assets failed")
	}

	if m.consensusType != peer.MPCConsensusBC {
		return nil
	}

	id, err := m.bcModule.SendRegAssetsTransaction(map[string]float64{key: price})
	if err != nil {
		return err
	}
	log.Info().Msgf("send regAssets txn %s for Assets %s", id, key)

	return nil
}

// GetPeerAssetPrices returns the assets inside the network with the keys and prices
func (m *MPCModule) GetPeerAssetPrices() map[string]map[string]float64 {
	if m.consensusType != peer.MPCConsensusBC {
		return nil
	}

	states := m.bcModule.GetLatestBlock().States

	return permissioned.GetAllAssetsFromWorldState(states)
}

func (m *MPCModule) ComputeExpression(uniqID string, expr string, prime string) (int, error) {
	fmt.Printf("#################### %s Start Expression %s(%s) #############################\n", m.conf.Socket.GetAddress(), expr, uniqID)

	postfix, variablesNeed, err := permissioned.GetPostfixAndVariables(expr)
	if err != nil {
		return -1, err
	}

	// get MPC
	mpc := m.mpcCenter.GetMPC(uniqID)

	// SSS to all participants that the peer have public key
	for key := range variablesNeed {
		value, found := m.valueDB.getAsset(key)
		if !found {
			// this peer doesn't have this value, continue
			continue
		}
		// Add to temp for secret share
		mpc.addValue(key, *big.NewInt(int64(value)))

		// SSS the value
		log.Info().Msgf("%s: I own value %s, sharing to participants: %s",
			m.conf.Socket.GetAddress(), key, mpc.getParticipants())
		err = m.shareSecret(key, mpc)
		if err != nil {
			// log.Error().Msgf("%s: sss error, %s", m.conf.Socket.GetAddress(), err)
			return -1, fmt.Errorf("%s: sss error, %s", m.conf.Socket.GetAddress(), err)
		}
	}

	ans, err := m.computeResult(postfix, mpc)
	if err != nil {
		// log.Error().Msgf("%s: compute result error, %s", m.conf.Socket.GetAddress(), err)
		return -1, fmt.Errorf("%s: compute result error, expression: %s, %s", m.conf.Socket.GetAddress(), mpc.expression, err)
	}

	fmt.Printf("#################### %s End Expression: %s, uniqID: %s, ans: %d) #############################\n", m.conf.Socket.GetAddress(), expr, uniqID, ans.Int64())
	return int(ans.Int64()), nil
}

// -----------------------------------------------------------------------------
// Private Helper Functions

func (m *MPCModule) shareSecret(key string, mpc *MPC) error {
	// log.Printf("%s: start share secret, key: %s, peers: %s",
	// 	m.conf.Socket.GetAddress(), key, peers)

	value, ok := mpc.getValue(key)
	if !ok {
		return fmt.Errorf("no valid value is found for key %s", key)
	}

	// generate the list of MPC id
	peerIDs, err := mpc.getPeerIDs()
	if err != nil {
		return err
	}

	// generate shared secrets
	// results, err := m.shamirSecretShare(value, peerIDs)
	results, err := m.shamirSecretShareHalfDegreeZp(value, mpc.prime, peerIDs)
	if err != nil {
		return err
	}

	// log.Printf("%s: generated sss result: %s: %s", m.conf.Socket.GetAddress(), key, results)

	// send shared secrets
	peers := mpc.getParticipants()
	for idx, result := range results {
		err := m.sendShareMessage(
			mpc.id, peers[idx], int(peerIDs[idx].Uint64()), key+"|"+peers[idx], result)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MPCModule) computeResult(postfix []string, mpc *MPC) (big.Int, error) {
	participants := mpc.getParticipants()
	var s []big.Int
	for i := 0; i < len(postfix); i++ {
		switch ch := postfix[i]; ch {
		case "+", "-", "*", "/":
			num1, num2 := s[len(s)-2], s[len(s)-1]
			s = s[:len(s)-2]
			var res big.Int
			var err error
			if ch == "+" {
				res = addZp(&num1, &num2, &mpc.prime)
			} else if ch == "-" {
				res = subZp(&num1, &num2, &mpc.prime)
			} else if ch == "*" {
				res, err = m.computeMult(num1, num2, i, mpc)
			} else if ch == "/" {
				// only support for constant
				res = divZp(&num1, &num2, &mpc.prime)
			}
			if err != nil {
				return *big.NewInt(0), err
			}
			s = append(s, res)
		default:
			num, err := strconv.ParseInt(ch, 10, 64)
			var bigNum big.Int
			if err != nil {
				// this is a value needed from SSS.
				key := ch + "|" + m.getIdentifyKey()
				bigNum = mpc.waitValueFromTemp(key)
			} else {
				bigNum = *big.NewInt(num)
			}
			s = append(s, bigNum)
		}
	}

	// boardcast the result and compute the final result
	err := m.boardcastInterpolationResult(s[0], mpc)
	if err != nil {
		return *big.NewInt(0), err
	}

	// Use interpolation to compute the final result
	peerIDs, err := mpc.getPeerIDs()
	if err != nil {
		return *big.NewInt(0), err
	}

	// busy wait for other key to receive.
	shareResult := make([]big.Int, len(participants))
	for i := 0; i < len(participants); i++ {
		tmpKey := participants[i] + "|InterpolationResult"
		shareResult[i] = mpc.waitValueFromTemp(tmpKey)
	}

	return m.lagrangeInterpolationZp(shareResult, peerIDs, &mpc.prime), nil
}

// func (m *MPCModule) getValueFromTemp(key string) big.Int {
// 	value, ok := m.mpc.getValue(key)
// 	for !ok {
// 		// Busy wait here
// 		time.Sleep(time.Millisecond * 1)
// 		value, ok = m.mpc.getValue(key)
// 	}
// 	return value
// }

// func (m *MPCModule) computeAdd(x int, y int, z bool) (int, error) {
// 	if z {
// 		return subZp(x, y), nil
// 	}
// 	return addZp(x, y), nil
// }

func (m *MPCModule) computeMult(a big.Int, b big.Int, step int, mpc *MPC) (big.Int, error) {
	// 1. ∀Pi: compute di = aibi.
	// 2. ∀Pi: share di → di1, . . . , din.
	// 3. ∀Pj: compute cj = w1d1j + . . . + wndnj.

	d := multZp(&a, &b, &mpc.prime)

	key := m.getIdentifyKey() + "|" + strconv.Itoa(step)
	mpc.addValue(key, d)

	err := m.shareSecret(key, mpc)
	if err != nil {
		return *big.NewInt(0), err
	}

	// generate the list of MPC id
	peerIDs, err := mpc.getPeerIDs()
	if err != nil {
		return *big.NewInt(0), err
	}

	// collect all share secret
	participants := mpc.getParticipants()
	shareD := make([]big.Int, len(participants))
	for i := 0; i < len(participants); i++ {
		tmpKey := participants[i] + "|" + strconv.Itoa(step) + "|" + m.getIdentifyKey()
		shareD[i] = mpc.waitValueFromTemp(tmpKey)
	}

	return m.lagrangeInterpolationZp(shareD, peerIDs, &mpc.prime), nil
	// return x * y, nil
}

func (m *MPCModule) sendShareMessage(uniqID string, to string, id int, key string, value big.Int) error {
	switch m.consensusType {
	case peer.MPCConsensusPaxos:
		return m.sendShareMessagePaxos(uniqID, to, id, key, value)
	case peer.MPCConsensusBC:
		return m.sendShareMessageBlockchain(uniqID, to, id, key, value)
	}
	panic("invalid MPC type")
}

func (m *MPCModule) boardcastInterpolationResult(result big.Int, mpc *MPC) error {
	switch m.consensusType {
	case peer.MPCConsensusPaxos:
		return m.boardcastInterpolationResultPaxos(result, mpc)
	case peer.MPCConsensusBC:
		return m.boardcastInterpolationResultBlockchain(result, mpc)
	}
	panic("invalid MPC type")
}

func (m *MPCModule) getIdentifyKey() string {
	switch m.consensusType {
	case peer.MPCConsensusPaxos:
		return m.conf.Socket.GetAddress()
	case peer.MPCConsensusBC:
		addr, err := m.bcModule.GetChainAddress()
		if err != nil {
			panic(err)
		}
		return addr.Hex
	}
	panic("invalid MPC type")
}
