package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"os"
	"time"
)

func main() {
	client := GetClient()

	//Generate new keys for the account you will create
	newAccountPrivateKey, err := hedera.PrivateKeyGenerateEd25519()

	if err != nil {
		panic(err)
	}

	newAccountPublicKey := newAccountPrivateKey.PublicKey()

	//Create new account and assign the public key
	newAccount, err := hedera.NewAccountCreateTransaction().
		SetKey(newAccountPublicKey).
		SetInitialBalance(hedera.HbarFrom(1000, hedera.HbarUnits.Hbar)).
		Execute(client)

	//Request the receipt of the transaction
	receipt, err := newAccount.GetReceipt(client)
	if err != nil {
		panic(err)
	}

	//Get the new account ID from the receipt
	newAccountId := *receipt.AccountID

	//Log the account ID
	fmt.Printf("The new account ID is %v\n", newAccountId)

	client.SetOperator(newAccountId, newAccountPrivateKey)

	topicID, err := hedera.TopicIDFromString(os.Args[1])
	if err != nil {
		HandleError(err)
	}

	signingKey, err := hedera.PrivateKeyFromString(os.Args[2])
	if err != nil {
		HandleError(err)
	}

	fmt.Printf("Sending to topic %s", topicID)
	fmt.Println(signingKey)
	//Send "Hello, HCS!" to the topic
	submitMessage, err := hedera.NewTopicMessageSubmitTransaction().
		SetMessage([]byte("Hello, HCS!")).
		SetTopicID(topicID).
		Sign(signingKey).
		Execute(client)

	if err != nil {
		println(err.Error(), ": error submitting to topic")
		return
	}

	//Get the receipt of the transaction
	receipt, err = submitMessage.GetReceipt(client)

	//Get the transaction status
	transactionStatus := receipt.Status
	fmt.Println("The message transaction status " + transactionStatus.String())

	//Get the receipt of the transaction
	record, err := submitMessage.GetRecord(client)

	//Get the transaction status
	fmt.Printf("The message record transaction time %v", record.ConsensusTimestamp)

	//Prevent the program from exiting to display the message from the mirror to the console
	time.Sleep(30000)
}
