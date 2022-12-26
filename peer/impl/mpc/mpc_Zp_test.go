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
