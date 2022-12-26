package mpc

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueDB *ValueDB
	mpc     *MPC
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
		valueDB:       NewValueDB(),
		mpc:           NewMPC(1),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCShareMessage{}, m.ProcessMPCShareMsg)
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCInterpolationMessage{}, m.ProcessMPCInterpolationMsg)

	return &m
}

/** Feature Functions **/

func (m *MPCModule) SetValueDBAsset(key string, value int) error {
	ok := m.valueDB.addAsset(key, value)
	if !ok {
		return xerrors.Errorf("Add Assets failed")
	}
	return nil
}

// This is the entry point of the calling the MPC.
func (m *MPCModule) ComputeExpression(expr string, budget uint) (int, error) {
	// check if we receive all public key

	// change infix to postfix
	postfix, err := infixToPostfix(expr)
	if err != nil {
		return -1, err
	}

	// TODO: change here to paxos
	pubKeyStore := m.GetPubkeyStore()
	participants := make([]string, 0, len(pubKeyStore))
	for key := range pubKeyStore {
		participants = append(participants, key)
	}
	variablesNeed := []string{}
	for _, exp := range postfix {
		var IsVariableName = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`).MatchString
		if IsVariableName(exp) {
			variablesNeed = append(variablesNeed, exp)
		}
	}

	// TODO make prime into the proposer
	mpcPrime := *big.NewInt(1000000009)

	// TODO change here to use hash of pubkey
	// add MPC peer
	peersMap := map[string]int{}
	for _, participant := range participants {
		peerIDstr := strings.Split(participant, ":")[1]
		peerID, _ := strconv.Atoi(peerIDstr)
		peersMap[participant] = peerID
	}
	m.mpc.addPeers(peersMap)

	propose := MPCPropose{
		proposer:     m.conf.Socket.GetAddress(),
		budget:       budget,
		participants: participants,
		postfix:      postfix,
	}
	log.Info().Msgf("MPCPropose, proposer: %s, budget: %d, participans: %s, postfix: %s",
		propose.proposer, propose.budget, propose.participants, propose.postfix)

	// SSS to all participants that the peer have public key
	for _, key := range variablesNeed {
		value, found := m.valueDB.getAsset(key)
		if !found {
			// this peer doesn't have this value, continue
			continue
		}
		// Add to temp for secret share
		m.mpc.addValue(key, *big.NewInt(int64(value)))

		// SSS the value
		log.Info().Msgf("%s: I own value %s, sharing to participants: %s",
			m.conf.Socket.GetAddress(), key, participants)
		err = m.shareSecret(key, participants, mpcPrime)
		if err != nil {
			log.Error().Msgf("%s: sss error, %s", m.conf.Socket.GetAddress(), err)
			return -1, err
		}
	}

	ans, err := m.computeResult(postfix, participants, mpcPrime)
	if err != nil {
		log.Error().Msgf("%s: compute result error, %s", m.conf.Socket.GetAddress(), err)
		return -1, err
	}

	return int(ans.Uint64()), nil
}

/** Private Helpfer Functions **/
func infixToPostfix(infix string) ([]string, error) {
	// '+', "-", is not used as a unary operation (i.e., "+1", "-(2 + 3)"", "-1", "3-(-2)" are invalid).
	infix = strings.ReplaceAll(infix, " ", "")
	var NoInValidChar = regexp.MustCompile(`^[a-zA-Z0-9_\+\-\*\/\^()\.]+$`).MatchString
	if !NoInValidChar(infix) {
		return []string{}, xerrors.Errorf("Infix contains illegal character!")
	}

	var IsVariableName = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`).MatchString
	s := Stack{}
	postfix := []string{}
	valid := true

	curVariable := ""
	for _, char := range infix {
		opchar := string(char)
		// if scanned character is operand, add it to output string
		if IsVariableName(opchar) {
			curVariable += opchar
			continue
		} else {
			if curVariable != "" {
				postfix = append(postfix, curVariable)
			}
			curVariable = ""
		}

		if char == '(' {
			s.Push(opchar)
		} else if char == ')' {
			for s.Top() != "(" {
				postfix = append(postfix, s.Top())
				valid = valid && s.Pop()
			}
			valid = valid && s.Pop()
		} else {
			for !s.IsEmpty() && prec(opchar) <= prec(s.Top()) {
				postfix = append(postfix, s.Top())
				valid = valid && s.Pop()
			}
			s.Push(opchar)
		}

		if !valid {
			return []string{}, xerrors.Errorf("Infix is invalid!")
		}
	}
	if curVariable != "" {
		postfix = append(postfix, curVariable)
	}
	// Pop all the remaining elements from the stack
	for !s.IsEmpty() {
		postfix = append(postfix, s.Top())
		s.Pop()
	}
	return postfix, nil
}

func (m *MPCModule) computeResult(postfix []string, participants []string, prime big.Int) (big.Int, error) {
	var s []big.Int
	for i := 0; i < len(postfix); i++ {
		switch ch := postfix[i]; ch {
		case "+", "-", "*", "/":
			num1, num2 := s[len(s)-2], s[len(s)-1]
			s = s[:len(s)-2]
			var res big.Int
			var err error
			if ch == "+" {
				res = addZp(&num1, &num2, &prime)
			} else if ch == "-" {
				res = subZp(&num1, &num2, &prime)
			} else {
				res, err = m.computeMult(num1, num2, prime, i, participants)
			}
			if err != nil {
				return *big.NewInt(0), err
			}
			s = append(s, res)
		default:
			// TODO change num to SSS
			num, err := strconv.ParseInt(ch, 10, 64)
			bigNum := *big.NewInt(num)
			if err != nil {
				// this is a value needed from SSS.
				key := ch + "|" + m.conf.Socket.GetAddress()
				bigNum = m.getValueFromSSS(key)
			}
			s = append(s, bigNum)
		}
	}

	// boardcast the result and compute the final result
	m.boardcastInterpolationResult(s[0], participants)

	// Use interpolation to compute the final result
	peerIDs, err := m.mpc.getPeerIDs(participants)
	if err != nil {
		return *big.NewInt(0), err
	}
	bigPeerIDs := make([]big.Int, len(peerIDs))
	for idx, peerID := range peerIDs {
		bigPeerIDs[idx] = *big.NewInt(int64(peerID))
	}

	// busy wait for other key to receive.
	// TODO: this is not receive from boardcast not sss, might need to change the function name.
	shareResult := make([]big.Int, len(participants))
	for i := 0; i < len(participants); i++ {
		tmpKey := participants[i] + "|InterpolationResult"
		shareResult[i] = m.getValueFromSSS(tmpKey)
	}

	// return m.lagrangeInterpolation(shareResult, peerIDs), nil
	return m.lagrangeInterpolationZp(shareResult, bigPeerIDs, &prime), nil
}

func (m *MPCModule) boardcastInterpolationResult(result big.Int, participants []string) error {
	// boardcast the result and compute the final result
	interpolationMsg := types.MPCInterpolationMessage{
		ReqID: m.mpc.id,
		Owner: m.conf.Socket.GetAddress(),
		Value: result.Text(10),
	}
	interpolationMsgMarshal, err := m.CreateMsg(interpolationMsg)
	if err != nil {
		return err
	}
	// wrap in private msg
	privRecipients := map[string]struct{}{}
	for _, participant := range participants {
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

func (m *MPCModule) getValueFromSSS(key string) big.Int {
	value, ok := m.mpc.getValue(key)
	for !ok {
		// Busy wait here
		time.Sleep(time.Millisecond * 1)
		value, ok = m.mpc.getValue(key)
	}
	return value
}

// func (m *MPCModule) computeAdd(x int, y int, z bool) (int, error) {
// 	if z {
// 		return subZp(x, y), nil
// 	}
// 	return addZp(x, y), nil
// }

func (m *MPCModule) computeMult(a big.Int, b big.Int, prime big.Int, step int, participants []string) (big.Int, error) {
	// 1. ∀Pi: compute di = aibi.
	// 2. ∀Pi: share di → di1, . . . , din.
	// 3. ∀Pj: compute cj = w1d1j + . . . + wndnj.

	d := multZp(&a, &b, &prime)

	key := m.conf.Socket.GetAddress() + "|" + strconv.Itoa(step)
	m.mpc.addValue(key, d)

	// TODO: save participants in proposer and get using proposerID to avoid copy the whole list.
	err := m.shareSecret(key, participants, prime)
	if err != nil {
		return *big.NewInt(0), err
	}

	// generate the list of MPC id
	peerIDs, err := m.mpc.getPeerIDs(participants)
	if err != nil {
		return *big.NewInt(0), err
	}
	bigPeerIDs := make([]big.Int, len(peerIDs))
	for idx, peerID := range peerIDs {
		bigPeerIDs[idx] = *big.NewInt(int64(peerID))
	}

	// collect all share secret
	shareD := make([]big.Int, len(participants))
	for i := 0; i < len(participants); i++ {
		tmpKey := participants[i] + "|" + strconv.Itoa(step) + "|" + m.conf.Socket.GetAddress()
		shareD[i] = m.getValueFromSSS(tmpKey)
	}

	return m.lagrangeInterpolationZp(shareD, bigPeerIDs, &prime), nil
	// return x * y, nil
}
