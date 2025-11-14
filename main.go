package main

import (
	"log"

	"instrumentation-score-service/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
