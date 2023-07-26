module github.com/taekion-org/sawtooth-hcs-consensus

go 1.16

//replace github.com/hyperledger/sawtooth-sdk-go v0.1.4 => github.com/taekion-org/sawtooth-sdk-go v0.1.6
replace github.com/hyperledger/sawtooth-sdk-go v0.1.4 => ../sawtooth-sdk-go

require (
	github.com/hashgraph/hedera-sdk-go/v2 v2.26.1
	github.com/hyperledger/sawtooth-sdk-go v0.1.4
	github.com/jessevdk/go-flags v1.5.0
	github.com/joho/godotenv v1.5.1
	github.com/taekion-org/sawtooth-client-sdk-go v0.1.1
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230725213213-b022f6e96895 // indirect
)
