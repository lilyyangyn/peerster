package mpc

import (
	"math/rand"
)

// ValueDB stores values that can be used in MPC
type ValueDB struct {
	table map[string]int
}

func (db *ValueDB) add(key string, value int) bool {
	old, ok := db.table[key]
	if ok && old != value {
		return false
	}
	db.table[key] = value
	return true
}
func (db *ValueDB) get(key string) (int, bool) {
	value, ok := db.table[key]
	return value, ok
}
func NewValueDB() *ValueDB {
	db := ValueDB{
		table: map[string]int{},
	}

	return &db
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
		value += p.coefficients[i-1]
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
