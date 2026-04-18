package main

import (
	"ecommerce/cmd/cli/commands"
	"log"
	"os"
)

func main() {
	if err := commands.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
