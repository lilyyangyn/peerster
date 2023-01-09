package types

import (
	"fmt"
	"regexp"
	"strings"
)

type MPCSecretValue struct {
	Owner string // node identifier
	Key   string // key in the node DB
	Value string
}

// MPCShareMessage describes a message for MPC secret sharing.
type MPCShareMessage struct {
	ReqID string
	Value MPCSecretValue
}

// MPCInterpolationMessage describes a message for MPC interpolation.
type MPCInterpolationMessage struct {
	ReqID string
	Owner string
	Value string
}

// -----------------------------------------------------------------------------

func GetPostfixAndVariables(expr string) ([]string, map[string]struct{}, error) {
	// change infix to postfix
	postfix, err := infixToPostfix(expr)
	if err != nil {
		return []string{}, map[string]struct{}{}, err
	}

	variablesNeed := map[string]struct{}{}
	for _, exp := range postfix {
		var IsVariableName = regexp.MustCompile(`^[a-zA-Z0-9_\.]+$`).MatchString
		if IsVariableName(exp) {
			variablesNeed[exp] = struct{}{}
		}
	}
	return postfix, variablesNeed, nil
}

func infixToPostfix(infix string) ([]string, error) {
	// '+', "-", is not used as a unary operation (i.e., "+1", "-(2 + 3)"", "-1", "3-(-2)" are invalid).
	infix = strings.ReplaceAll(infix, " ", "")
	var NoInValidChar = regexp.MustCompile(`^[a-zA-Z0-9_\+\-\*\/\^()\.]+$`).MatchString
	if !NoInValidChar(infix) {
		return []string{}, fmt.Errorf("infix contains illegal character")
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
			return []string{}, fmt.Errorf("infix is invalid")
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

// --------------------------------------------------------

type Stack []string

// IsEmpty: check if stack is empty
func (st *Stack) IsEmpty() bool {
	return len(*st) == 0
}

// Push a new value onto the stack
func (st *Stack) Push(str string) {
	*st = append(*st, str) //Simply append the new value to the end of the stack
}

// Remove top element of stack. Return false if stack is empty.
func (st *Stack) Pop() bool {
	if st.IsEmpty() {
		return false
	} else {
		index := len(*st) - 1 // Get the index of top most element.
		*st = (*st)[:index]   // Remove it from the stack by slicing it off.
		return true
	}
}

// Return top element of stack. Return false if stack is empty.
func (st *Stack) Top() string {
	if st.IsEmpty() {
		return ""
	} else {
		index := len(*st) - 1   // Get the index of top most element.
		element := (*st)[index] // Index onto the slice and obtain the element.
		return element
	}
}

// Function to return precedence of operators
func prec(s string) int {
	if s == "^" {
		return 3
	} else if (s == "/") || (s == "*") {
		return 2
	} else if (s == "+") || (s == "-") {
		return 1
	} else {
		return -1
	}
}
