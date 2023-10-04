module github.com/taekion-org/sawtooth-hcs-consensus

go 1.16

replace (
	github.com/hyperledger/sawtooth-sdk-go v0.1.4 => ../sawtooth-sdk-go
	google.golang.org/protobuf v1.27.1 => google.golang.org/protobuf v1.26.1-0.20210525005349-febffdd88e85
)

require (
	github.com/hashgraph/hedera-sdk-go/v2 v2.30.0
	github.com/hyperledger/sawtooth-sdk-go v0.1.4
	github.com/jessevdk/go-flags v1.5.0
	github.com/joho/godotenv v1.5.1
	github.com/taekion-org/sawtooth-client-sdk-go v0.1.1
	google.golang.org/grpc v1.58.2
	gorm.io/driver/sqlite v1.5.3
	gorm.io/gorm v1.25.4
)
