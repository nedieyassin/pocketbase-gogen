package main

import (
	"log"

	"github.com/nedieyassin/pocketbase-gogen/cmd"
)

func main() {
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	cmd.Execute()
}
