package main

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"os"
)

func main() {
	client := GetClient()

	// Generate a new keypair for the validators to use to publish messages.
	submitPrivateKey, err := hedera.PrivateKeyGenerateEd25519()
	if err != nil {
		panic(err)
	}
	submitPublicKey := submitPrivateKey.PublicKey()

	transactionResponse, err := hedera.NewTopicCreateTransaction().
		SetSubmitKey(submitPublicKey).
		Execute(client)

	if err != nil {
		println(err.Error(), ": error creating topic")
		return
	}

	receipt, err := transactionResponse.GetReceipt(client)
	if err != nil {
		println(err.Error(), ": error getting topic create receipt")
		return
	}
	topicID := *receipt.TopicID

	newAccountPrivateKey, err := hedera.PrivateKeyGenerateEd25519()
	if err != nil {
		panic(err)
	}
	newAccountPublicKey := newAccountPrivateKey.PublicKey()

	newAccount, err := hedera.NewAccountCreateTransaction().
		SetKey(newAccountPublicKey).
		SetInitialBalance(hedera.HbarFrom(1000, hedera.HbarUnits.Hbar)).
		Execute(client)

	receipt, err = newAccount.GetReceipt(client)
	if err != nil {
		panic(err)
	}
	newAccountId := *receipt.AccountID

	fmt.Printf("topicID: %v\n", topicID)
	fmt.Printf("submitPrivateKey: %v\n", submitPrivateKey.String())
	fmt.Printf("accountID: %v\n", newAccountId)
	fmt.Printf("accountPrivateKey: %v\n", newAccountPrivateKey.String())

	f, err := os.OpenFile("engine.env", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	f.WriteString(fmt.Sprintf("TOPIC_ID=%v\n", topicID))
	f.WriteString(fmt.Sprintf("ACCOUNT_ID=%v\n", newAccountId))
	f.WriteString(fmt.Sprintf("ACCOUNT_PRIVATE_KEY=%v\n", newAccountPrivateKey.String()))
	f.WriteString(fmt.Sprintf("SUBMIT_PRIVATE_KEY=%v\n", submitPrivateKey.String()))
	f.WriteString("\n")
	f.Close()
}
