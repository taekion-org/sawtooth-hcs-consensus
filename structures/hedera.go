package structures

import (
	"github.com/hyperledger/sawtooth-sdk-go/consensus"
	"time"
)

type HCSEngineTopicMessageType string

const BLOCK_INTENT HCSEngineTopicMessageType = "BLOCK_INTENT"
const BLOCK_PROPOSAL HCSEngineTopicMessageType = "BLOCK_PROPOSAL"
const TIME_TICK HCSEngineTopicMessageType = "TIME_TICK"

type HCSEngineTopicMessage struct {
	Type            HCSEngineTopicMessageType `json:"type,omitempty"`
	PeerId          consensus.PeerId          `json:"peer_id,omitempty"`
	BlockIntent     HCSEngineBlockIntent      `json:"block_intent,omitempty"`
	BlockProposal   HCSEngineBlockProposal    `json:"block_proposal,omitempty"`
	HederaTimestamp time.Time                 `json:"hedera_timestamp,omitempty"`
}

type HCSEngineBlockIntent struct {
	BlockNumber   uint64 `json:"block_number,omitempty"`
	PrevBlockHash string `json:"prev_block_hash,omitempty"`
}

type HCSEngineBlockProposal struct {
	PrevStateProof string `json:"prev_state_proof,omitempty"`
	PrevBlockHash  string `json:"prev_block_hash,omitempty"`

	BlockHash   string `json:"block_hash,omitempty"`
	BlockNumber uint64 `json:"block_number,omitempty"`

	BatchHashes []string `json:"batch_hashes,omitempty"`
}
