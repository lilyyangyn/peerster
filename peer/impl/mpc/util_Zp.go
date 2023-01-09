package mpc

import (
	"crypto/rand"
	"math/big"

	"github.com/rs/zerolog/log"
)

// polynomial is an expression of polynomial
// all coefficients are big.Int
type polynomialZp struct {
	degree       int
	coefficients []big.Int
}

// addP adds two numbers in Zp, returns a + b mod p
func addZp(a, b, p *big.Int) big.Int {
	sum := big.NewInt(0)
	sum.Add(a, b)
	sum.Mod(sum, p)
	return *sum
}

// subP subtractcs two numbers in Zp, returns a - b mod p
func subZp(a, b, p *big.Int) big.Int {
	dif := big.NewInt(0)
	dif.Sub(a, b)

	zero := big.NewInt(0)
	if dif.Cmp(zero) == -1 {
		dif.Add(dif, p)
	}
	return *dif
}

// multP multiplies two numbers in Zp, returns a * b mod p
func multZp(a, b, p *big.Int) big.Int {
	prod := big.NewInt(0)
	prod.Mul(a, b)
	prod.Mod(prod, p)
	return *prod
}

// divP divides two numbers in Zp, returns a * b^-1 mod p
func divZp(a, b, p *big.Int) big.Int {
	// if b == 0, panic
	zero := big.NewInt(0)
	if b.Cmp(zero) == 0 {
		// should never happen
		panic("divide by zero")
	}

	b0 := new(big.Int).Set(b)
	bInverse := new(big.Int).Set(b0.ModInverse(b0, p))
	quo := big.NewInt(0)
	quo.Mul(a, bInverse)
	quo.Mod(quo, p)
	return *quo
}

// generate random prime of given bits
// need to set rand.Seed in advance
func generateRandomPrime(bits int) (*big.Int, error) {
	prime, err := rand.Prime(rand.Reader, bits)
	if err != nil {
		log.Err(err)
		return nil, err
	}
	return prime, nil
}

// generate random number in Zp
func generateRandomNumber(p *big.Int) (*big.Int, error) {
	n, err := rand.Int(rand.Reader, p)
	if err != nil {
		log.Err(err)
		return nil, err
	}
	return n, nil
}

// RandomPolynomialP generate a random polynomial with f(0)=secret
// while all coefficients are big.Int, in Zp
// default: secret < p
func NewRandomPolynomialZp(secret big.Int, degree int, p *big.Int) (*polynomialZp, error) {
	// random polynomial f of degree d is defined by d + 1 points
	coefficients := make([]big.Int, degree+1)
	// s = f(0) = secret
	//coefficients[0] = *big.NewInt(int64(secret))
	coefficients[0] = secret

	// generate random coefficients
	for i := 1; i <= degree; i++ {
		n, err := generateRandomNumber(p)
		if err != nil {
			log.Err(err)
			return nil, err
		}
		coefficients[i] = *n
	}

	poly := polynomialZp{
		degree:       degree,
		coefficients: coefficients,
	}

	return &poly, nil
}

// computeP evaluates corresponding y value of the given x coordinate
// on the polynomialP (in Zp)
func (p *polynomialZp) computePolynomialZp(x, prime *big.Int) big.Int {
	// if x == 0 returns polynomial coefficents[0]
	zero := big.NewInt(0)
	if x.Cmp(zero) == 0 {
		return p.coefficients[0]
	}

	value := p.coefficients[p.degree]
	for i := p.degree - 1; i > -1; i-- {
		value = multZp(&value, x, prime)
		value = addZp(&value, &(p.coefficients[i]), prime)
		//value = multGF(value, x)
		//value = addGF(value, p.coefficients[i])
	}
	return value
}
