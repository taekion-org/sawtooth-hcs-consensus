package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
)

func main() {
	client := GetClient()

	privateKey, err := hedera.PrivateKeyGenerateEd25519()
	if err != nil {
		HandleError(err)
	}

	fmt.Println(privateKey)

	//Create a new topic
	transactionResponse, err := hedera.NewTopicCreateTransaction().
		SetSubmitKey(privateKey.PublicKey()).
		Execute(client)

	if err != nil {
		println(err.Error(), ": error creating topic")
		return
	}

	//Get the topic create transaction receipt
	transactionReceipt, err := transactionResponse.GetReceipt(client)

	if err != nil {
		println(err.Error(), ": error getting topic create receipt")
		return
	}

	//Get the topic ID from the transaction receipt
	topicID := *transactionReceipt.TopicID

	//Log the topic ID to the console
	fmt.Printf("topicID: %v\n", topicID)

}
