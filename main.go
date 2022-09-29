package main

import (
	"fmt"
	"github.com/hyperledger/sawtooth-sdk-go/consensus"
	"github.com/hyperledger/sawtooth-sdk-go/logging"
	"github.com/jessevdk/go-flags"
	"github.com/taekion-org/sawtooth-hcs-consensus/engine"
	"os"
	"syscall"
)

type Opts struct {
	Verbose []bool `short:"v" long:"verbose" description:"Increase verbosity"`
	Connect string `short:"C" long:"connect" description:"Validator consensus endpoint to connect to" default:"tcp://localhost:5050"`
}

func main() {
	var opts Opts

	logger := logging.Get()

	parser := flags.NewParser(&opts, flags.Default)
	remaining, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			logger.Errorf("Failed to parse args: %v", err)
			os.Exit(2)
		}
	}

	if len(remaining) > 0 {
		fmt.Printf("Error: Unrecognized arguments passed: %v\n", remaining)
		os.Exit(2)
	}

	endpoint := opts.Connect

	switch len(opts.Verbose) {
	case 2:
		logger.SetLevel(logging.DEBUG)
	case 1:
		logger.SetLevel(logging.INFO)
	default:
		logger.SetLevel(logging.WARN)
	}

	hcs_engine := consensus.NewConsensusEngine(endpoint, &engine.HCSEngineImpl{})
	hcs_engine.ShutdownOnSignal(syscall.SIGINT, syscall.SIGTERM)
	hcs_engine.Start()
	if err != nil {
		logger.Errorf("Consensus engine stopped: %v", err)
	}
}
