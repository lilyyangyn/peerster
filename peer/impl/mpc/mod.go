package mpc

import (
	"regexp"
	"strconv"
	"strings"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/peer/impl/message"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

type MPCModule struct {
	*message.MessageModule
	conf *peer.Configuration

	valueDB *ValueDB
	*MPC
}

func NewMPCModule(conf *peer.Configuration, messageModule *message.MessageModule) *MPCModule {
	m := MPCModule{
		MessageModule: messageModule,
		conf:          conf,
		valueDB:       NewValueDB(),
	}

	// message registery
	m.conf.MessageRegistry.RegisterMessageCallback(types.MPCShareMessage{}, m.ProcessMPCShareMsg)

	return &m
}

/** Feature Functions **/

func (m *MPCModule) SetMPCValue(key string, value int) error {
	ok := m.valueDB.add(key, value)
	if !ok {
		return xerrors.Errorf("key for MPC value already used")
	}

	return nil
}

// This is the entry point of the calling the MPC.
func (m *MPCModule) ComputeExpression(expr string) (int, error) {
	// change infix to postfix
	postfix, err := infixToPostfix(expr)
	if err != nil {
		return -1, err
	}

	ans, err := computeResult(postfix, m)
	if err != nil {
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

func computeResult(postfix []string, m *MPCModule) (int, error) {
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
				res, err = m.computeMult(num1, num2)
			}
			if err != nil {
				return 0, err
			}
			s = append(s, res)
		default:
			// TODO change num to SSS
			num, _ := strconv.Atoi(ch)
			s = append(s, num)
		}
	}
	return s[0], nil
}
