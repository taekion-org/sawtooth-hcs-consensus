package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/taekion-org/sawtooth-client-sdk-go/transport/rest"
	"github.com/taekion-org/sawtooth-hcs-consensus/structures"
	"net/url"
	"os"
)

func main() {
	client := GetClient()

	//Create a new topic
	transactionResponse, err := hedera.NewTopicCreateTransaction().
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

	url, err := url.Parse(os.Args[1])
	if err != nil {
		panic(err)
	}

	transport, err := rest.NewSawtoothClientTransportRest(url)
	if err != nil {
		panic(err)
	}

	iter := transport.GetBlockIterator(1, false)
	iter.Next()

	genesisBlock, err := iter.Current()
	if genesisBlock.Header.BlockNum != "0" {
		panic("Not the genesis block...")
	}

	batchHashes := make([]string, len(genesisBlock.Batches))
	for i, batch := range genesisBlock.Batches {
		batchHashes[i] = batch.HeaderSignature
	}

	message := structures.HCSEngineTopicMessage{
		Type:   structures.BLOCK_PROPOSAL,
		PeerId: "",
		BlockProposal: structures.HCSEngineBlockProposal{
			PrevStateProof: "",
			PrevBlockHash:  "",
			BlockHash:      genesisBlock.HeaderSignature,
			BlockNumber:    0,
			BatchHashes:    batchHashes,
		},
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}

	submitMessage, err := hedera.NewTopicMessageSubmitTransaction().
		SetMessage(msgBytes).
		SetTopicID(topicID).
		Execute(client)

	if err != nil {
		panic(err)
	}

	//Get the receipt of the transaction
	receipt, err := submitMessage.GetReceipt(client)

	//Get the transaction status
	transactionStatus := receipt.Status
	fmt.Println("Genesis transaction status " + transactionStatus.String())
}
