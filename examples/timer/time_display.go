package main

import (
	"fmt"

	"github.com/spaolacci/murmur3"
)

func main() {
	fmt.Println(murmur3.Sum32([]byte("hello")))
}
