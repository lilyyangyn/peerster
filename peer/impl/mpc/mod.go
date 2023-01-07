package mpc

import (
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/peer/impl/paxos"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueDB *ValueDB

	*sync.RWMutex
	mpcstore map[string]*MPC
	paxos    *paxos.PaxosInstance
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule, paxosModule *paxos.PaxosModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
		valueDB:       NewValueDB(),
		RWMutex:       &sync.RWMutex{},
		mpcstore:      map[string]*MPC{},
	}
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

// Calculate start a new MPC from making consensus on budget and expression.
// It will then initiate the MPC automatically
func (m *MPCModule) Calculate(expression string, budget float64) (int, error) {
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

	// channel wait mpc result
	m.RLock()
	resultChan := m.mpcstore[uniqID].resultChan
	m.RUnlock()

	result := <-resultChan

	return result.result, result.err
}

func (m *MPCModule) InitMPC(uniqID string, prime string, initiator string,
	expression string, resultChan chan MPCResult) error {
	m.Lock()
	defer m.Unlock()

	mpcPrime, _ := new(big.Int).SetString(prime, 10)
	m.mpcstore[uniqID] = NewMPC(uniqID, *mpcPrime, initiator, expression, resultChan)

	// Use public key as participants
	pubKeyStore := m.GetPubkeyStore()
	if int(m.conf.TotalPeers) > len(pubKeyStore) {
		return xerrors.Errorf("%s: not received everyone's public key, require %d, have %d",
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
	m.mpcstore[uniqID].addPeers(peersMap)
	m.mpcstore[uniqID].addParticipants(participants)
	return nil
}

func (m *MPCModule) ComputeExpression(uniqID string, expr string, prime string) (int, error) {
	// change infix to postfix
	postfix, err := infixToPostfix(expr)
	if err != nil {
		return -1, err
	}

	// get MPC
	m.RLock()
	mpc, ok := m.mpcstore[uniqID]
	m.RUnlock()
	if !ok {
		return -1, err
	}

	variablesNeed := map[string]struct{}{}
	for _, exp := range postfix {
		var IsVariableName = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`).MatchString
		if IsVariableName(exp) {
			variablesNeed[exp] = struct{}{}
		}
	}

	// SSS to all participants that the peer have public key
	for key, _ := range variablesNeed {
		value, found := m.valueDB.getAsset(key)
		if !found {
			// this peer doesn't have this value, continue
			continue
		}
		// Add to temp for secret share
		mpc.addValue(key, *big.NewInt(int64(value)))

		// SSS the value
		log.Printf("%s: I own value %s, sharing to participants: %s",
			m.conf.Socket.GetAddress(), key, mpc.getParticipants())
		err = m.shareSecret(key, mpc)
		if err != nil {
			log.Printf("%s: sss error, %s", m.conf.Socket.GetAddress(), err)
			return -1, err
		}
	}

	ans, err := m.computeResult(postfix, mpc)
	if err != nil {
		log.Printf("%s: compute result error, %s", m.conf.Socket.GetAddress(), err)
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
				key := ch + "|" + m.conf.Socket.GetAddress()
				bigNum = mpc.waitValueFromTemp(key)
			} else {
				bigNum = *big.NewInt(num)
			}
			s = append(s, bigNum)
		}
	}

	// boardcast the result and compute the final result
	m.boardcastInterpolationResult(s[0], mpc)

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

func (m *MPCModule) boardcastInterpolationResult(result big.Int, mpc *MPC) error {
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

	key := m.conf.Socket.GetAddress() + "|" + strconv.Itoa(step)
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
		tmpKey := participants[i] + "|" + strconv.Itoa(step) + "|" + m.conf.Socket.GetAddress()
		shareD[i] = mpc.waitValueFromTemp(tmpKey)
	}

	return m.lagrangeInterpolationZp(shareD, peerIDs, &mpc.prime), nil
	// return x * y, nil
}
