package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Viberowser - A Web Browser")
	if len(os.Args) > 1 {
		fmt.Printf("URL: %s\n", os.Args[1])
	}
}
