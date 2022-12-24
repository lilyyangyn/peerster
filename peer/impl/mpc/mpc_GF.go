package mpc

import "golang.org/x/xerrors"

// Lagrange Interpolation in GF(2^8)
// all inputs are uint8 !! must be in GF(2^8)
func (m *MPCModule) lagrangeInterpolationGF(ycoord []uint8, xcoord []uint8) uint8 {
	// length of two input arrays must be identical
	// equal to number of nodes

	var result uint8 = 0
	for i, y := range ycoord {
		var w uint8 = 1
		for j, x := range xcoord {
			if i == j {
				continue
			}
			denominator := addGF(x, xcoord[i])
			tmp := divGF(x, denominator)
			w = multGF(w, tmp)

			// tmp := float64(x) / float64(x-xcoord[i])
			// w = w * tmp
		}
		product := multGF(w, y)
		result = addGF(result, product)

		//floatResult = floatResult + w*float64(y)
	}
	return result
}

// SSS in GF(2^8)
func (m *MPCModule) shamirSecretShareGF(secret uint8, xcoord []uint8) (results []uint8, err error) {
	// no redundancy. assume participants will not down during MPC
	degree := len(xcoord)

	p, err := NewRandomPolynomialGF(secret, degree)
	if err != nil {
		return
	}
	for idx, id := range xcoord {
		// id should not equal to zero, or the secret will be directly leaked
		if id == 0 {
			err = xerrors.Errorf("illegal input x equals to 0")
			return
		}
		results[idx] = p.computeGF(id)
	}

	return results, nil
}
