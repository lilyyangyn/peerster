package mpc

import (
	"fmt"
	"math/big"

	"github.com/rs/zerolog/log"
)

// Lagrange Interpolation in Zp, returns polynomial value on x = 0
// all inputs are big.Int !! must be in Zp
func (m *MPCModule) lagrangeInterpolationZp(ycoord []big.Int, xcoord []big.Int, p *big.Int) big.Int {
	// length of two input arrays must be identical
	// equal to number of nodes

	result := big.NewInt(0)
	for i, y := range ycoord {
		w := big.NewInt(1)
		for j, x := range xcoord {
			if i == j {
				continue
			}
			denominator := subZp(&x, &(xcoord[i]), p)
			tmp := divZp(&x, &denominator, p)
			*w = multZp(w, &tmp, p)

			// tmp := float64(x) / float64(x-xcoord[i])
			// w = w * tmp
		}
		product := multZp(w, &y, p)
		*result = addZp(result, &product, p)

		//floatResult = floatResult + w*float64(y)
	}
	return *result
}

// SSS in Zp
func (m *MPCModule) shamirSecretShareZp(secret, prime big.Int, xcoord []big.Int) (results []big.Int, err error) {
	// no redundancy. assume participants will not down during MPC
	degree := len(xcoord) - 1

	poly, err := NewRandomPolynomialZp(secret, degree, &prime)
	if err != nil {
		return
	}

	zero := big.NewInt(0)
	for _, id := range xcoord {
		// id should not equal to zero, or the secret will be directly leaked
		if id.Cmp(zero) == 0 {
			err = fmt.Errorf("illegal input x equals to 0")
			log.Err(err)
			return
		}
		// results[idx] = poly.computePolynomialZp(&id, &prime)
		results = append(results, poly.computePolynomialZp(&id, &prime))
	}

	return results, nil
}

// ------------------------------------Test Version--------------------------------------

// Lagrange Interpolation in Zp for test
func lagrangeInterpolationZpTest(ycoord []big.Int, xcoord []big.Int, p *big.Int) big.Int {
	// length of two input arrays must be identical
	// equal to number of nodes

	result := big.NewInt(0)
	for i, y := range ycoord {
		w := big.NewInt(1)
		for j, x := range xcoord {
			if i == j {
				continue
			}
			denominator := subZp(&x, &(xcoord[i]), p)
			tmp := divZp(&x, &denominator, p)
			*w = multZp(w, &tmp, p)

			// tmp := float64(x) / float64(x-xcoord[i])
			// w = w * tmp
		}
		product := multZp(w, &y, p)
		*result = addZp(result, &product, p)

		//floatResult = floatResult + w*float64(y)
	}
	return *result
}

// SSS in Zp for test
func shamirSecretShareZpTest(secret, prime big.Int, xcoord []big.Int) (results []big.Int, err error) {
	// no redundancy. assume participants will not down during MPC
	degree := len(xcoord) - 1

	poly, err := NewRandomPolynomialZp(secret, degree, &prime)
	if err != nil {
		return
	}

	zero := big.NewInt(0)
	for _, id := range xcoord {
		// id should not equal to zero, or the secret will be directly leaked
		if id.Cmp(zero) == 0 {
			err = fmt.Errorf("illegal input x equals to 0")
			log.Err(err)
			return
		}
		// results[idx] = poly.computePolynomialZp(&id, &prime)
		results = append(results, poly.computePolynomialZp(&id, &prime))
	}

	return results, nil
}
