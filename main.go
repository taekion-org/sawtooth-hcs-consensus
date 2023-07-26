package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/hyperledger/sawtooth-sdk-go/consensus"
	"github.com/hyperledger/sawtooth-sdk-go/logging"
	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
	"github.com/taekion-org/sawtooth-hcs-consensus/engine"
	"os"
	"syscall"
)

type Opts struct {
	Verbose []bool `short:"v" long:"verbose" description:"Increase verbosity"`
	Connect string `short:"C" long:"connect" description:"Validator consensus endpoint to connect to" default:"tcp://localhost:5050"`
	EnvFile string `short:"E" long:"env" description:"Path to .env file" default:"engine.env"`
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
			//logger.Errorf("Failed to parse args: %v", err)
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

	// Set up Hedera Connection
	//Loads the .env file and throws an error if it cannot load the variables from that file correctly
	err = godotenv.Load(opts.EnvFile)
	if err != nil {
		panic(fmt.Errorf("Unable to load environment variables from .env file. Error:\n%v\n", err))
	}

	//Grab your testnet account ID and private key from the .env file
	accountId, err := hedera.AccountIDFromString(os.Getenv("ACCOUNT_ID"))
	if err != nil {
		panic(err)
	}

	accountPrivateKey, err := hedera.PrivateKeyFromString(os.Getenv("ACCOUNT_PRIVATE_KEY"))
	if err != nil {
		panic(err)
	}

	submitPrivateKey, err := hedera.PrivateKeyFromString(os.Getenv("SUBMIT_PRIVATE_KEY"))
	if err != nil {
		panic(err)
	}

	logger.Info("Connecting to Hedera network...")

	//Create your testnet client
	client := hedera.ClientForTestnet()
	client.SetOperator(accountId, accountPrivateKey)

	logger.Info("Hedera network connection ready...")

	impl := engine.NewHCSEngineImpl(client, submitPrivateKey)
	hcs_engine := consensus.NewConsensusEngine(endpoint, impl)
	hcs_engine.ShutdownOnSignal(syscall.SIGINT, syscall.SIGTERM)
	hcs_engine.Start()
	if err != nil {
		logger.Errorf("Consensus engine stopped: %v", err)
	}
}
