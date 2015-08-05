package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	res := []byte(`"asdfasfd"`)

	var s string = ""
	err := json.Unmarshal(res, &s)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("s is this:", s)
}
