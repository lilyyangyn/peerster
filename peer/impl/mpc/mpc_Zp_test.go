package mpc

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// test the prime generator
func Test_prime_generator(t *testing.T) {
	bits := 1024
	primeTestTimes := 10

	for i := 0; i < 10; i++ {
		prime, err := generateRandomPrime(bits)
		fmt.Println(*prime)
		require.NoError(t, err)

		isPrime := prime.ProbablyPrime(primeTestTimes)
		fmt.Println(isPrime)
		fmt.Print("\n")
		require.Equal(t, isPrime, true)
	}
}

// test the pseudo-number generator
func Test_random_number_generator(t *testing.T) {
	bits := 1024
	testTimes := 10

	for i := 0; i < testTimes; i++ {
		prime, err := generateRandomPrime(bits)
		//fmt.Println(prime)
		require.NoError(t, err)

		for j := 0; j < testTimes; j++ {
			n, err := generateRandomNumber(prime)
			//fmt.Println(n)
			require.NoError(t, err)

			compare := n.Cmp(prime)
			require.Equal(t, compare, -1)
		}
		//fmt.Print("\n")
	}
}

// test Lagrange Interpolation with degree as zero, and more data points than needed
func Test_interpolation_zero_degree(t *testing.T) {
	// secret = 5, prime = 11, degree = 0
	xcoord := []big.Int{*big.NewInt(1), *big.NewInt(2), *big.NewInt(3)}
	ycoord := []big.Int{*big.NewInt(5), *big.NewInt(5), *big.NewInt(5)}
	prime := big.NewInt(11)

	res := lagrangeInterpolationZpTest(ycoord, xcoord, prime)
	require.Equal(t, res, *big.NewInt(5))

	// secret = 0, prime = random prime of 1024 bit, degree = 0
	xcoord = []big.Int{*big.NewInt(1), *big.NewInt(2), *big.NewInt(3), *big.NewInt(4), *big.NewInt(5), *big.NewInt(6)}
	ycoord = []big.Int{*big.NewInt(0), *big.NewInt(0), *big.NewInt(0), *big.NewInt(0), *big.NewInt(0), *big.NewInt(0)}
	prime, err := generateRandomPrime(1024)
	require.NoError(t, err)

	res = lagrangeInterpolationZpTest(ycoord, xcoord, prime)
	require.Equal(t, res, *big.NewInt(0))

	// secret = random, prime = random prime of 1024 bit, degree = 0
	prime, err = generateRandomPrime(1024)
	require.NoError(t, err)

	secret, err := generateRandomNumber(prime)
	require.NoError(t, err)

	poly, err := NewRandomPolynomialZp(*secret, 0, prime)
	require.NoError(t, err)

	xcoord = []big.Int{*big.NewInt(1), *big.NewInt(2), *big.NewInt(3), *big.NewInt(4), *big.NewInt(5), *big.NewInt(6), *big.NewInt(7)}
	results := make([]big.Int, 7)
	for idx, id := range xcoord {
		results[idx] = poly.computePolynomialZp(&id, prime)
	}

	res = lagrangeInterpolationZpTest(results, xcoord, prime)
	require.Equal(t, res, *secret)
}

// test Lagrange Interpolation with degree smaller than data points (given more data points than needed)
func Test_interpolation_lower_degree(t *testing.T) {
	// secret = random, prime = random, degree = 1, number of points = 3
	prime, err := generateRandomPrime(10)
	require.NoError(t, err)
	secret, err := generateRandomNumber(prime)
	require.NoError(t, err)

	poly, err := NewRandomPolynomialZp(*secret, 1, prime)
	require.NoError(t, err)

	xcoord := []big.Int{*big.NewInt(1), *big.NewInt(2), *big.NewInt(3)}
	results := make([]big.Int, 3)
	for idx, id := range xcoord {
		results[idx] = poly.computePolynomialZp(&id, prime)
	}

	res := lagrangeInterpolationZpTest(results, xcoord, prime)
	require.Equal(t, res, *secret)

	// secret = random, prime = random, degree = 2, number of points = 5
	prime, err = generateRandomPrime(512)
	require.NoError(t, err)
	secret, err = generateRandomNumber(prime)
	require.NoError(t, err)

	poly, err = NewRandomPolynomialZp(*secret, 2, prime)
	require.NoError(t, err)

	xcoord = []big.Int{*big.NewInt(1), *big.NewInt(2), *big.NewInt(3), *big.NewInt(4), *big.NewInt(5)}
	results = make([]big.Int, 5)
	for idx, id := range xcoord {
		results[idx] = poly.computePolynomialZp(&id, prime)
	}

	res = lagrangeInterpolationZpTest(results, xcoord, prime)
	require.Equal(t, res, *secret)

	// secret = random, prime = random, degree = 6 (full degree), number of points = 7
	prime, err = generateRandomPrime(2048)
	require.NoError(t, err)
	secret, err = generateRandomNumber(prime)
	require.NoError(t, err)

	poly, err = NewRandomPolynomialZp(*secret, 6, prime)
	require.NoError(t, err)

	xcoord = []big.Int{*big.NewInt(1), *big.NewInt(2), *big.NewInt(3), *big.NewInt(4), *big.NewInt(5), *big.NewInt(6), *big.NewInt(7)}
	results = make([]big.Int, 7)
	for idx, id := range xcoord {
		results[idx] = poly.computePolynomialZp(&id, prime)
	}

	res = lagrangeInterpolationZpTest(results, xcoord, prime)
	require.Equal(t, res, *secret)
}

// test correctness of Shamir Secret Sharing and Lagrange interpolation
func Test_SSS_interpolation(t *testing.T) {
	// the number of bits of the large prime. can be any. tested: 5, 10, 20, 512, 1024, 2048
	primeBits := 2048
	prime, err := generateRandomPrime(primeBits)
	fmt.Println("prime:", *prime)
	require.NoError(t, err)

	fmt.Println("test SSS and Lagrange Interpolation")
	fmt.Print("\n")
	// secret can be any number smaller than the prime
	//secret := big.NewInt(24)
	secret, err := generateRandomNumber(prime)
	require.NoError(t, err)
	fmt.Println(*secret)
	// pseudo peer id (x coordinates), tested total number: from 2 to 7
	xcoord := []big.Int{*(big.NewInt(1)), *(big.NewInt(2)), *(big.NewInt(3)), *(big.NewInt(4)), *(big.NewInt(5)), *(big.NewInt(6)), *(big.NewInt(7))}
	//fmt.Print(secret, xcoord, "\n")

	testTimes := 10
	for i := 0; i < testTimes; i++ {
		fmt.Printf("\nNumber %v iteration \n", i)
		ycoord, err := shamirSecretShareZpTest(*secret, *prime, xcoord)
		fmt.Print(ycoord, "\n")
		require.NoError(t, err)

		result := lagrangeInterpolationZpTest(ycoord, xcoord, prime)
		fmt.Println("result: ", result)
		require.Equal(t, result, *secret)
	}
}

// test addition property of mpc Zp computation
func Test_mpc_addition(t *testing.T) {
	/*
		Number 7 iteration
		[{false [621]} {false [870]}]
		result:  {false [372]}

		Number 8 iteration
		[{false [176]} {false [891]}]
		result:  {false [372]}

		Number 9 iteration
		[{false [663]} {false [43]}]
		result:  {false [372]}
	*/
	prime := big.NewInt(911)
	xcoord := []big.Int{*(big.NewInt(1)), *(big.NewInt(2))}
	//secret := big.NewInt(372)

	fmt.Println("test double secret: 372 * 2 = 744")

	secretDouble := big.NewInt(372 * 2 % 911)
	ycoord := []big.Int{*(big.NewInt((176 + 663) % 911)), *(big.NewInt((891 + 43) % 911))}
	result := lagrangeInterpolationZpTest(ycoord, xcoord, prime)

	fmt.Println(result)
	fmt.Println()
	require.Equal(t, result, *secretDouble)

	fmt.Println("test triple secret: 372 * 3 % 911 = 205")

	secretTriple := big.NewInt(372 * 3 % 911)
	ycoord = []big.Int{*(big.NewInt((176 + 663 + 621) % 911)), *(big.NewInt((891 + 43 + 870) % 911))}
	result = lagrangeInterpolationZpTest(ycoord, xcoord, prime)

	fmt.Println(result)
	fmt.Println()
	require.Equal(t, result, *secretTriple)
}

// test correctness of Shamir Secret Sharing (half degree) and Lagrange interpolation
func Test_SSS_half_interpolation(t *testing.T) {
	// the number of bits of the large prime. can be any.
	primeBits := 1024
	prime, err := generateRandomPrime(primeBits)
	require.NoError(t, err)

	fmt.Println("test SSS of half degree and Lagrange Interpolation")
	fmt.Println()

	// secret: random
	secret, err := generateRandomNumber(prime)
	fmt.Println("secret: ", *secret)
	require.NoError(t, err)

	// pseudo peer id (x coordinates), tested total number: from 2 to 7
	xcoord := []big.Int{*(big.NewInt(1)), *(big.NewInt(2)), *(big.NewInt(3)), *(big.NewInt(4)), *(big.NewInt(5)), *(big.NewInt(6)), *(big.NewInt(7))}

	testTimes := 10
	for i := 0; i < testTimes; i++ {
		fmt.Printf("\nNumber %v iteration \n", i)
		ycoord, err := shamirSecretShareHalfDegreeZpTest(*secret, *prime, xcoord)
		// fmt.Print(ycoord, "\n")
		require.NoError(t, err)

		result := lagrangeInterpolationZpTest(ycoord, xcoord, prime)
		fmt.Println("result: ", result)
		require.Equal(t, result, *secret)
	}
}

// test multiplication property of mpc Zp computation, with simple constant inputs
func Test_mpc_multplication_Simple(t *testing.T) {
	// 3 participants, non-random inputs
	const num = 3
	xcoord := []big.Int{*(big.NewInt(1)), *(big.NewInt(2)), *(big.NewInt(3))}

	// 2 secrets, prime = 11
	prime := big.NewInt(11)
	a := big.NewInt(2)
	b := big.NewInt(4)
	result := multZp(a, b, prime)

	// first SSS
	y1, err := shamirSecretShareHalfDegreeZpTest(*a, *prime, xcoord)
	require.NoError(t, err)
	y2, err := shamirSecretShareHalfDegreeZpTest(*b, *prime, xcoord)
	require.NoError(t, err)

	// each peer computes their own product
	yMult := make([]big.Int, num)
	for idx, y := range y1 {
		tmp := multZp(&y, &(y2[idx]), prime)
		yMult[idx] = tmp
	}

	// each peer shares their own product using SSS of half degree
	var yMatrix [num][num]big.Int
	for i, y := range yMult {
		tmp, err := shamirSecretShareHalfDegreeZpTest(y, *prime, xcoord)
		require.NoError(t, err)

		for j := 0; j < num; j++ {
			yMatrix[i][j] = tmp[j]
		}
	}

	// each peer compute Lagrange Interpolation on its own shares
	shares := make([]big.Int, num)
	for i := 0; i < num; i++ {
		ycoord := make([]big.Int, num)
		for j := 0; j < num; j++ {
			ycoord[j] = yMatrix[j][i]
		}
		shares[i] = lagrangeInterpolationZpTest(ycoord, xcoord, prime)
	}

	// finally, use Lagrange Interpolation to reconstruct the result
	res := lagrangeInterpolationZpTest(shares, xcoord, prime)
	fmt.Println(res)
	require.Equal(t, result, res)
}

// test multiplication property of mpc Zp computation, with random inputs
func Test_mpc_multplication_Random(t *testing.T) {
	// 3 participants
	const num = 3
	xcoord := []big.Int{*(big.NewInt(1)), *(big.NewInt(2)), *(big.NewInt(3))}

	// random large prime
	prime, err := generateRandomPrime(100)
	require.NoError(t, err)

	// secrets are random, but product of secrets should be less than the prime (in real life this makes sense)
	limit, err := generateRandomPrime(20)
	require.NoError(t, err)

	a, err := generateRandomNumber(limit)
	require.NoError(t, err)

	b, err := generateRandomNumber(limit)
	require.NoError(t, err)

	result := multZp(a, b, prime)
	fmt.Println("correct product:", result)

	// first SSS
	y1, err := shamirSecretShareHalfDegreeZpTest(*a, *prime, xcoord)
	require.NoError(t, err)
	y2, err := shamirSecretShareHalfDegreeZpTest(*b, *prime, xcoord)
	require.NoError(t, err)

	// each peer computes their own product
	yMult := make([]big.Int, num)
	for idx, y := range y1 {
		tmp := multZp(&y, &(y2[idx]), prime)
		yMult[idx] = tmp
	}

	// each peer shares their own product using SSS of half degree
	var yMatrix [num][num]big.Int
	for i, y := range yMult {
		tmp, err := shamirSecretShareHalfDegreeZpTest(y, *prime, xcoord)
		require.NoError(t, err)

		for j := 0; j < num; j++ {
			yMatrix[i][j] = tmp[j]
		}
	}

	// each peer compute Lagrange Interpolation on its own shares
	shares := make([]big.Int, num)
	for i := 0; i < num; i++ {
		ycoord := make([]big.Int, num)
		for j := 0; j < num; j++ {
			ycoord[j] = yMatrix[j][i]
		}
		shares[i] = lagrangeInterpolationZpTest(ycoord, xcoord, prime)
	}

	// finally, use Lagrange Interpolation to reconstruct the result
	res := lagrangeInterpolationZpTest(shares, xcoord, prime)
	fmt.Println("calculated result:", res)
	require.Equal(t, result, res)
}

// test multiplication property of mpc Zp computation, with large inputs
func Test_mpc_multplication_Hard(t *testing.T) {
	// 6 participants
	const num = 6
	xcoord := []big.Int{*(big.NewInt(1)), *(big.NewInt(2)), *(big.NewInt(3)), *(big.NewInt(4)), *(big.NewInt(5)), *(big.NewInt(6))}

	// random large prime
	prime, err := generateRandomPrime(2048)
	require.NoError(t, err)

	// secrets a, b are random
	// attention: the product of secrets should be less than the prime (in real life this makes sense)
	limit, err := generateRandomPrime(1000)
	require.NoError(t, err)

	a, err := generateRandomNumber(limit)
	require.NoError(t, err)

	b, err := generateRandomNumber(limit)
	require.NoError(t, err)

	result := multZp(a, b, prime)
	fmt.Println("correct product:", result)

	// first SSS
	y1, err := shamirSecretShareHalfDegreeZpTest(*a, *prime, xcoord)
	require.NoError(t, err)
	y2, err := shamirSecretShareHalfDegreeZpTest(*b, *prime, xcoord)
	require.NoError(t, err)

	// each peer computes their own product
	yMult := make([]big.Int, num)
	for idx, y := range y1 {
		tmp := multZp(&y, &(y2[idx]), prime)
		yMult[idx] = tmp
	}

	// each peer shares their own product using SSS of half degree
	var yMatrix [num][num]big.Int
	for i, y := range yMult {
		tmp, err := shamirSecretShareHalfDegreeZpTest(y, *prime, xcoord)
		require.NoError(t, err)

		for j := 0; j < num; j++ {
			yMatrix[i][j] = tmp[j]
		}
	}

	// each peer compute Lagrange Interpolation on its own shares
	shares := make([]big.Int, num)
	for i := 0; i < num; i++ {
		ycoord := make([]big.Int, num)
		for j := 0; j < num; j++ {
			ycoord[j] = yMatrix[j][i]
		}
		shares[i] = lagrangeInterpolationZpTest(ycoord, xcoord, prime)
	}

	// finally, use Lagrange Interpolation to reconstruct the result
	res := lagrangeInterpolationZpTest(shares, xcoord, prime)
	fmt.Println("calculated result:", res)
	require.Equal(t, result, res)
}
