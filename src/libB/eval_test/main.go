package main

import (
	"fmt"

	luaeval "github.com/koeng101/libB/src/c"
)

func main() {
	result, err := luaeval.Eval("return 1+1")
	fmt.Println(result, err)
}
