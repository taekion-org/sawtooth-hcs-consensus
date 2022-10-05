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

	fmt.Printf("Subscribing to topic %s\n", topicID)

	_, err = hedera.NewTopicMessageQuery().
		SetTopicID(topicID).
		Subscribe(client, func(message hedera.TopicMessage) {
			fmt.Println(message.ConsensusTimestamp.String(), "received topic message ", string(message.Contents), "\r")
			fmt.Println(message.ConsensusTimestamp, message.SequenceNumber)
		})
	if err != nil {
		HandleError(err)
	}

	//Prevent the program from exiting to display the message from the mirror to the console
	time.Sleep(1000 * time.Second)
}
