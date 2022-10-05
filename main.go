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
	TopicID string `short:"T" long:"topic" description:"HCS topic ID" required:"true"`
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
	err = godotenv.Load(".env")
	if err != nil {
		panic(fmt.Errorf("Unable to load environment variables from .env file. Error:\n%v\n", err))
	}

	//Grab your testnet account ID and private key from the .env file
	myAccountId, err := hedera.AccountIDFromString(os.Getenv("MY_ACCOUNT_ID"))
	if err != nil {
		panic(err)
	}

	myPrivateKey, err := hedera.PrivateKeyFromString(os.Getenv("MY_PRIVATE_KEY"))
	if err != nil {
		panic(err)
	}

	//Create your testnet client
	client := hedera.ClientForTestnet()
	client.SetOperator(myAccountId, myPrivateKey)

	topicID, err := hedera.TopicIDFromString(opts.TopicID)
	if err != nil {
		panic(err)
	}
	logger.Infof("Using HCS topic ID %v", topicID)

	impl := engine.NewHCSEngineImpl(topicID, client)
	hcs_engine := consensus.NewConsensusEngine(endpoint, impl)
	hcs_engine.ShutdownOnSignal(syscall.SIGINT, syscall.SIGTERM)
	hcs_engine.Start()
	if err != nil {
		logger.Errorf("Consensus engine stopped: %v", err)
	}
}
