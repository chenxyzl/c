package gate

import (
	"fmt"
	"testing"
)

func Test_a(t *testing.T) {
	a := 1
	switch a {
	case 1:
		fmt.Print("1")
		fallthrough
	case 2:
		a = 3
		fmt.Print("2")
	case 3:
		fmt.Print("3")
	}
}
