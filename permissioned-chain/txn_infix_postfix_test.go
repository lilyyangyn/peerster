package permissioned

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Infix_To_Postfix(t *testing.T) {
	infix := make([]string, 3)
	infix[0] = "a+b"
	infix[1] = "a + b*  c + d"
	infix[2] = "aa+bb*(cc^dd-ee)^(ff + gg  * hh )-ii"
	expectedAns := make([][]string, 3)
	expectedAns[0] = []string{"a", "b", "+"}
	expectedAns[1] = []string{"a", "b", "c", "*", "+", "d", "+"}
	expectedAns[2] = []string{"aa", "bb", "cc", "dd", "^", "ee", "-", "ff", "gg", "hh", "*", "+", "^", "*", "+", "ii", "-"}

	for i, test := range infix {
		postfix, err := infixToPostfix(test)
		fmt.Printf("test %d, %s infix has %s postfix\n", i, test, postfix)
		require.NoError(t, err)

		require.Len(t, postfix, len(expectedAns[i]))
		for j := 0; j < len(expectedAns[i]); j++ {
			require.Equal(t, postfix[j], expectedAns[i][j])
		}
	}
}
