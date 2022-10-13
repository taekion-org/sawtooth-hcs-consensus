package engine

import (
	"encoding/json"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/taekion-org/sawtooth-hcs-consensus/structures"
	"sync"
	"time"
)

type HCSBlockIntentState struct {
	Message *hedera.TopicMessage
	Intent  *structures.HCSEngineBlockIntent
}

type HCSBlockProposalState struct {
	Message  *hedera.TopicMessage
	Proposal *structures.HCSEngineBlockProposal
}

type HCSStateTracker struct {
	topic  hedera.TopicID
	client *hedera.Client

	intentState   map[uint64]*HCSBlockIntentState
	proposalState map[uint64]*HCSBlockProposalState

	maxIntentBlockNum   uint64
	maxProposalBlockNum uint64

	latestTime time.Time

	mutex              sync.Mutex
	timeCondition      *sync.Cond
	intentConditions   map[uint64]*sync.Cond
	proposalConditions map[uint64]*sync.Cond
}

func NewHCSStateTracker(topic hedera.TopicID, client *hedera.Client) *HCSStateTracker {
	return &HCSStateTracker{
		topic:               topic,
		client:              client,
		intentState:         make(map[uint64]*HCSBlockIntentState),
		proposalState:       make(map[uint64]*HCSBlockProposalState),
		maxIntentBlockNum:   0,
		maxProposalBlockNum: 0,
		latestTime:          time.Time{},
		mutex:               sync.Mutex{},
		timeCondition:       nil,
		intentConditions:    make(map[uint64]*sync.Cond),
		proposalConditions:  make(map[uint64]*sync.Cond),
	}
}

func (self *HCSStateTracker) Start() error {
	_, err := hedera.NewTopicMessageQuery().
		SetTopicID(self.topic).
		Subscribe(self.client, self.handleTopicMessage)
	if err != nil {
		return err
	}

	logger.Info("HCS State Tracker Started...")

	return nil
}

func (self *HCSStateTracker) Lock() {
	self.mutex.Lock()
}

func (self *HCSStateTracker) Unlock() {
	self.mutex.Unlock()
}

func (self *HCSStateTracker) handleTopicMessage(message hedera.TopicMessage) {
	self.Lock()
	defer self.Unlock()

	// Unmarshal the message from the Hedera topic wrapper.
	var decodedMessage structures.HCSEngineTopicMessage
	err := json.Unmarshal(message.Contents, &decodedMessage)
	if err != nil {
		panic(err)
	}

	debugMsg := func(extra string) {
		msg := fmt.Sprintf("HCS - SEQ: %d TS: %s TYPE: %s", message.SequenceNumber, message.ConsensusTimestamp, decodedMessage.Type)

		peer := decodedMessage.PeerId
		if peer == "" {
			peer = "NA"
		}
		msg += fmt.Sprintf(" PEER: %s", peer)
		if extra != "" {
			msg += " " + extra
		}
		logger.Debug(msg)
	}

	switch decodedMessage.Type {
	case structures.TIME_TICK:
		debugMsg("")

	case structures.BLOCK_INTENT:
		intent := decodedMessage.BlockIntent
		lastProposal := self.proposalState[self.maxProposalBlockNum]
		lastProposalTime := lastProposal.Message.ConsensusTimestamp
		lastProposalBlockNumber := lastProposal.Proposal.BlockNumber

		debugMsg(fmt.Sprintf("BLOCK: %d", intent.BlockNumber))

		// If the intent timestamp is after the last (accepted) proposal time and has incremented
		//the block number by 1, it is valid.
		if message.ConsensusTimestamp.After(lastProposalTime) && intent.BlockNumber == (lastProposalBlockNumber+1) {
			// If we already have an intent for this block number, we do not track this intent, return.
			if self.HasIntentState(intent.BlockNumber) {
				return
			} else { // Otherwise, set up the states
				self.intentState[intent.BlockNumber] = &HCSBlockIntentState{
					Message: &message,
					Intent:  &intent,
				}
				self.maxIntentBlockNum++
			}

			// Signal the condition variable
			self.GetIntentCondition(intent.BlockNumber).Signal()
		}
	case structures.BLOCK_PROPOSAL:
		proposal := decodedMessage.BlockProposal

		debugMsg(fmt.Sprintf("BLOCK: %d HASH: %s", proposal.BlockNumber, proposal.BlockHash))

		// Special case for genesis block.
		if message.SequenceNumber == 1 && proposal.BlockNumber == 0 {
			self.proposalState[proposal.BlockNumber] = &HCSBlockProposalState{
				Message:  &message,
				Proposal: &proposal,
			}
			self.maxProposalBlockNum = 0
			// Signal the condition variable
			self.GetProposalCondition(proposal.BlockNumber).Signal()

			return
		}

		// General case for ongoing consensus
		lastIntent := self.intentState[self.maxIntentBlockNum]
		lastIntentTime := lastIntent.Message.ConsensusTimestamp
		lastIntentBlockNumber := lastIntent.Intent.BlockNumber

		// If the proposal timestamp is after the last (accepted) intent time + the block time
		// and is for the same block number, it is valid.
		if (message.ConsensusTimestamp.After(lastIntentTime.Add(BLOCK_TIME_SECONDS * time.Second))) && (proposal.BlockNumber == lastIntentBlockNumber) {
			// If we already have a proposal for this block number, we do not track this proposal, return.
			if self.HasProposalState(proposal.BlockNumber) {
				return
			} else { // Otherwise, set up the state
				self.proposalState[proposal.BlockNumber] = &HCSBlockProposalState{
					Message:  &message,
					Proposal: &proposal,
				}
				self.maxProposalBlockNum++
				// Signal the condition variable
				self.GetProposalCondition(proposal.BlockNumber).Signal()
			}
		}
	}

	// Update the time regardless of the message type.
	self.latestTime = message.ConsensusTimestamp
	self.GetTimeCondition().Broadcast()
}

func (self *HCSStateTracker) GetTimeCondition() *sync.Cond {
	if self.timeCondition == nil {
		self.timeCondition = sync.NewCond(&self.mutex)
	}
	return self.timeCondition
}

func (self *HCSStateTracker) GetIntentCondition(blockNumber uint64) *sync.Cond {
	if _, exists := self.intentConditions[blockNumber]; !exists {
		self.intentConditions[blockNumber] = sync.NewCond(&self.mutex)
	}
	return self.intentConditions[blockNumber]
}

func (self *HCSStateTracker) GetProposalCondition(blockNumber uint64) *sync.Cond {
	if _, exists := self.proposalConditions[blockNumber]; !exists {
		self.proposalConditions[blockNumber] = sync.NewCond(&self.mutex)
	}
	return self.proposalConditions[blockNumber]
}

func (self *HCSStateTracker) GetLatestTime() time.Time {
	return self.latestTime
}

func (self *HCSStateTracker) GetIntentState(blockNumber uint64) (*HCSBlockIntentState, error) {
	if !self.HasIntentState(blockNumber) {
		return nil, fmt.Errorf("Intent for block number %d not found in state", blockNumber)
	}
	return self.intentState[blockNumber], nil
}

func (self *HCSStateTracker) GetProposalState(blockNumber uint64) (*HCSBlockProposalState, error) {
	if !self.HasProposalState(blockNumber) {
		return nil, fmt.Errorf("Proposal for block number %d not found in state", blockNumber)
	}
	return self.proposalState[blockNumber], nil
}

func (self *HCSStateTracker) HasIntentState(blockNumber uint64) bool {
	_, exists := self.intentState[blockNumber]
	return exists
}

func (self *HCSStateTracker) HasProposalState(blockNumber uint64) bool {
	_, exists := self.proposalState[blockNumber]
	return exists
}

func (self *HCSStateTracker) GetMaxIntentState() (*HCSBlockIntentState, error) {
	return self.GetIntentState(self.maxIntentBlockNum)
}

func (self *HCSStateTracker) GetMaxProposalState() (*HCSBlockProposalState, error) {
	return self.GetProposalState(self.maxProposalBlockNum)
}
