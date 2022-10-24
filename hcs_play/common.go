package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/joho/godotenv"
	"os"
)

func GetClient() *hedera.Client {
	//Loads the .env file and throws an error if it cannot load the variables from that file correctly
	err := godotenv.Load(".env")
	if err != nil {
		panic(fmt.Errorf("Unable to load environment variables from .env file. Error:\n%v\n", err))
	}

	//Grab your testnet account ID and private key from the .env file
	myAccountId, err := hedera.AccountIDFromString(os.Getenv("ACCOUNT_ID"))
	if err != nil {
		panic(err)
	}

	myPrivateKey, err := hedera.PrivateKeyFromString(os.Getenv("PRIVATE_KEY"))
	if err != nil {
		panic(err)
	}

	//Print your testnet account ID and private key to the console to make sure there was no error
	fmt.Printf("The account ID is = %v\n", myAccountId)
	fmt.Printf("The private key is = %v\n", myPrivateKey)

	//Create your testnet client
	client := hedera.ClientForTestnet()
	//client.SetOperator(myAccountId, myPrivateKey)

	return client
}

func HandleError(err error) {
	fmt.Println(err)
	os.Exit(-1)
}
