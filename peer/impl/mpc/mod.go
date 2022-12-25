package mpc

import (
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
		m.mpc.addValue(key, value)

		// SSS the value
		log.Info().Msgf("%s: I own value %s, sharing to participants: %s",
			m.conf.Socket.GetAddress(), key, participants)
		err = m.shareSecret(key, participants)
		if err != nil {
			log.Error().Msgf("%s: sss error, %s", m.conf.Socket.GetAddress(), err)
			return -1, err
		}
	}

	ans, err := m.computeResult(postfix, participants)
	if err != nil {
		log.Error().Msgf("%s: compute result error, %s", m.conf.Socket.GetAddress(), err)
		return -1, err
	}

	return ans, nil
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

func (m *MPCModule) computeResult(postfix []string, participants []string) (int, error) {
	var s []int
	for i := 0; i < len(postfix); i++ {
		switch ch := postfix[i]; ch {
		case "+", "-", "*", "/":
			num1, num2 := s[len(s)-2], s[len(s)-1]
			s = s[:len(s)-2]
			var res int
			var err error
			if ch == "+" {
				res, err = m.computeAdd(num1, num2, false)
			} else if ch == "-" {
				res, err = m.computeAdd(num1, num2, true)
			} else {
				res, err = m.computeMult(num1, num2, i, participants)
			}
			if err != nil {
				return 0, err
			}
			s = append(s, res)
		default:
			// TODO change num to SSS
			num, err := strconv.Atoi(ch)
			if err != nil {
				// this is a value needed from SSS.
				key := ch + "|" + m.conf.Socket.GetAddress()
				num = m.getValueFromSSS(key)
			}
			s = append(s, num)
		}
	}

	// boardcast the result and compute the final result
	mpcKey := m.conf.Socket.GetAddress() + "|" + strconv.Itoa(len(postfix))
	m.mpc.addValue(mpcKey, s[0])
	shareMsg := types.MPCShareMessage{
		ReqID: m.mpc.id,
		Value: types.MPCSecretValue{
			Owner: m.conf.Socket.GetAddress(),
			Key:   mpcKey,
			Value: s[0],
		},
	}
	shareMsgMarshal, err := m.CreateMsg(shareMsg)
	if err != nil {
		return 0, err
	}
	// wrap in private msg
	privRecipients := map[string]struct{}{}
	for _, participant := range participants {
		privRecipients[participant] = struct{}{}
	}
	privMsg := types.PrivateMessage{
		Recipients: privRecipients,
		Msg:        &shareMsgMarshal,
	}
	privMsgMarshal, err := m.CreateMsg(privMsg)
	if err != nil {
		return 0, err
	}
	err = m.Broadcast(privMsgMarshal)
	if err != nil {
		return 0, err
	}

	peerIDs, err := m.mpc.getPeerIDs(participants)
	if err != nil {
		return 0, err
	}
	// busy wait for other key to receive.
	// TODO: this is not receive from boardcast not sss, might need to change the function name.
	shareResult := make([]int, len(participants))
	for i := 0; i < len(participants); i++ {
		tmpKey := participants[i] + "|" + strconv.Itoa(len(postfix))
		shareResult[i] = m.getValueFromSSS(tmpKey)
	}

	return m.lagrangeInterpolation(shareResult, peerIDs), nil
	// return s[0], nil
}

func (m *MPCModule) getValueFromSSS(key string) int {
	value, ok := m.mpc.getValue(key)
	for !ok {
		// Busy wait here
		time.Sleep(time.Millisecond * 1)
		value, ok = m.mpc.getValue(key)
	}
	return value
}

func (m *MPCModule) computeAdd(x int, y int, z bool) (int, error) {
	if z {
		return x - y, nil
	}
	return x + y, nil
}

func (m *MPCModule) computeMult(a int, b int, step int, participants []string) (int, error) {
	// 1. ∀Pi: compute di = aibi.
	// 2. ∀Pi: share di → di1, . . . , din.
	// 3. ∀Pj: compute cj = w1d1j + . . . + wndnj.

	d := a * b
	key := m.conf.Socket.GetAddress() + "|" + strconv.Itoa(step)
	m.mpc.addValue(key, d)

	// TODO: save participants in proposer and get using proposerID to avoid copy the whole list.
	m.shareSecret(key, participants)

	// generate the list of MPC id
	peerIDs, err := m.mpc.getPeerIDs(participants)
	if err != nil {
		return 0, err
	}

	// collect all m.shareSecret()
	shareD := make([]int, len(participants))
	for i := 0; i < len(participants); i++ {
		tmpKey := participants[i] + "|" + strconv.Itoa(step) + "|" + m.conf.Socket.GetAddress()
		shareD[i] = m.getValueFromSSS(tmpKey)
	}

	return m.lagrangeInterpolation(shareD, peerIDs), nil
	// return x * y, nil
}
