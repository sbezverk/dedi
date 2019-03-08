package main

import (
	"fmt"
)

func main() {
	stop := make(chan struct{})
	fmt.Printf("WIP\n")
	<-stop
}
