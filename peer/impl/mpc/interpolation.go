package mpc

// float version of Lagrange Interpolation
// assume no OVERFLOW !!
// TO DO: GF(2^8) version?
func (m *MPCModule) lagrangeInterpolation(values []int, xcoord []int) (result int) {
	// length of two input arrays must be identical
	// equal to number of nodes

	var floatResult float64 = 0
	for i, y := range values {
		var w float64 = 1
		for j, x := range xcoord {
			if i == j {
				continue
			}
			tmp := float64(x) / float64(x-xcoord[i])
			w = w * tmp
		}
		floatResult = floatResult + w*float64(y)
	}
	result = int(floatResult)
	return
}
