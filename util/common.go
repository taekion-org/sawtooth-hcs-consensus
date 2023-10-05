package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/joho/godotenv"
	"os"
)

func GetClient() *hedera.Client {
	// Loads the .env file and throws an error if it cannot load the variables from that file correctly
	var err error
	envFile, ok := os.LookupEnv("ENV")
	if ok {
		err = godotenv.Load(envFile)
	} else {
		err = godotenv.Load(".env")
	}
	if err != nil {
		panic(fmt.Errorf("Unable to load environment variables from .env file (%s). Error:\n%v\n", envFile, err))
	}

	network := os.Getenv("NETWORK")

	myAccountId, err := hedera.AccountIDFromString(os.Getenv("ACCOUNT_ID"))
	if err != nil {
		panic(err)
	}

	myPrivateKey, err := hedera.PrivateKeyFromString(os.Getenv("ACCOUNT_PRIVATE_KEY"))
	if err != nil {
		panic(err)
	}

	var client *hedera.Client
	switch network {
	case "testnet":
		client = hedera.ClientForTestnet()
	case "mainnet":
		client = hedera.ClientForMainnet()
	default:
		panic("Unspecified or unsupported Hedera network")
	}

	client.SetOperator(myAccountId, myPrivateKey)

	return client
}

func HandleError(err error) {
	fmt.Println(err)
	os.Exit(-1)
}
