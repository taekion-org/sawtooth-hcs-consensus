package structures

type BlockProposal struct {
	PrevStateProof string `json:"prev_state_proof,omitempty"`
	PrevBlockHash  string `json:"prev_block_hash,omitempty"`

	BlockHash   string `json:"block_hash,omitempty"`
	BlockNumber uint64 `json:"block_number,omitempty"`

	TransactionHashes []string `json:"transaction_hashes,omitempty"`
}
