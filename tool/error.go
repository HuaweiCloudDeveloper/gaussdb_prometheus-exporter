package tool

import "fmt"

func errorf(s string, args ...interface{}) {
	panic(fmt.Errorf("pq: %s", fmt.Sprintf(s, args...)))
}
