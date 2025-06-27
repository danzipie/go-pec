package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/danzipie/go-pec/pec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: pec-parser <command> [options]")
		fmt.Println("Commands: verify")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "verify":
		verifyCmd(os.Args[2:])
	default:
		fmt.Println("Unknown command:", os.Args[1])
		os.Exit(1)
	}
}

func verifyCmd(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	in := fs.String("in", "", "Path to ricevuta .eml")
	fs.Parse(args)

	if *in == "" {
		fs.Usage()
		os.Exit(1)
	}

	err := pec.Verify(*in)
	if err != nil {
		log.Fatal("Verification failed:", err)
	}

	fmt.Println("Ricevuta is valid.")
}
