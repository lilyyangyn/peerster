package mpc

import (
	"math/rand"
	"sync"
)

// --------------------------------------------------------
// ValueDB

// ValueDB stores values that can be used in MPC
// Asset is the value know by the peer, temp is the value we save for MPC.
// temp need to refresh for each new MPC round.
type ValueDB struct {
	*sync.RWMutex
	asset map[string]int
}

func (db *ValueDB) addAsset(key string, value int) bool {
	db.Lock()
	defer db.Unlock()
	db.asset[key] = value
	return true
}
func (db *ValueDB) getAsset(key string) (int, bool) {
	db.RLock()
	defer db.RUnlock()
	value, ok := db.asset[key]
	return value, ok
}

func NewValueDB() *ValueDB {
	db := ValueDB{
		&sync.RWMutex{},
		map[string]int{},
	}
	return &db
}

// --------------------------------------------------------
// MPCStore

type MPCCenter struct {
	*sync.RWMutex
	nofitication map[string]chan MPCResult
	store        map[string]*MPC
}

func NewMPCCenter() *MPCCenter {
	return &MPCCenter{
		RWMutex:      &sync.RWMutex{},
		nofitication: map[string]chan MPCResult{},
		store:        map[string]*MPC{},
	}
}

func (c *MPCCenter) GetMPC(id string) (*MPC, bool) {
	c.RLock()
	defer c.RUnlock()

	mpc, ok := c.store[id]
	return mpc, ok
}

func (c *MPCCenter) RegisterMPC(id string, mpc *MPC) {
	c.Lock()
	defer c.Unlock()

	c.store[id] = mpc
	if _, ok := c.nofitication[id]; !ok {
		c.nofitication[id] = make(chan MPCResult, 1)
	}
}

func (c *MPCCenter) Inform(id string, result MPCResult) (err error) {
	c.RLock()
	defer c.RUnlock()

	mpc, ok := c.store[id]
	if ok {
		err = mpc.finalize(result)
	}

	channel, ok := c.nofitication[id]
	if !ok {
		return
	}
	channel <- result

	return err
}

func (c *MPCCenter) Listen(id string) MPCResult {
	c.Lock()
	// first check if MPC already have done
	if mpc, ok := c.store[id]; ok {
		if result, err := mpc.getResult(); err == nil {
			c.Unlock()
			return *result
		}
	}

	channel, ok := c.nofitication[id]
	if !ok {
		channel := make(chan MPCResult)
		c.nofitication[id] = channel
	}
	c.Unlock()

	result := <-channel

	c.Lock()
	defer c.Unlock()

	delete(c.nofitication, id)

	return result
}

// --------------------------------------------------------

// polynomial is an expression of polynomial that can be used in MPC
type polynomial struct {
	degree       int
	coefficients []int
}

// compute computes the value y of x on the polynomial
func (p *polynomial) compute(x int) int {
	if x == 0 {
		return p.coefficients[0]
	}

	value := p.coefficients[p.degree]
	for i := p.degree - 1; i > -1; i-- {
		value *= x
		value += p.coefficients[i]
	}

	return value
}

// RandomPolynomial generate a random polynomial with f(0)=secret
func NewRandomPolynomial(secret int, degree int) *polynomial {
	// random polynomial f of degree d is defined by d + 1 points
	coefficients := make([]int, degree+1)

	// s = f(0) = secret
	coefficients[0] = secret

	// generate randome coefficients
	for i := 0; i < degree; i++ {
		coefficients[i+1] = rand.Int()
	}

	p := polynomial{
		degree:       degree,
		coefficients: coefficients,
	}
	return &p
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
