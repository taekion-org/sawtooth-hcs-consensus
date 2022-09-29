package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"os"
	"time"
)

func main() {
	client := GetClient()

	topicID, err := hedera.TopicIDFromString(os.Args[1])
	if err != nil {
		HandleError(err)
	}

	fmt.Printf("Sending to topic %s", topicID)

	//Send "Hello, HCS!" to the topic
	submitMessage, err := hedera.NewTopicMessageSubmitTransaction().
		SetMessage([]byte("Hello, HCS!")).
		SetTopicID(topicID).
		Execute(client)

	if err != nil {
		println(err.Error(), ": error submitting to topic")
		return
	}

	//Get the receipt of the transaction
	receipt, err := submitMessage.GetReceipt(client)

	//Get the transaction status
	transactionStatus := receipt.Status
	fmt.Println("The message transaction status " + transactionStatus.String())

	//Prevent the program from exiting to display the message from the mirror to the console
	time.Sleep(30000)
}
