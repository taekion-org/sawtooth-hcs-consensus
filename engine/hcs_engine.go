package engine

import (
	"encoding/json"
	"fmt"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/hyperledger/sawtooth-sdk-go/consensus"
	"github.com/hyperledger/sawtooth-sdk-go/logging"
	"github.com/taekion-org/sawtooth-hcs-consensus/structures"
	"strconv"
	"time"
)

var logger = logging.Get()

const TICK_TIME_SECONDS = 10

type HCSEngineImpl struct {
	topic            hedera.TopicID
	blockTime        time.Duration
	client           *hedera.Client
	submitPrivateKey hedera.PrivateKey

	service       consensus.ConsensusService
	startupState  consensus.StartupState
	localPeerInfo consensus.PeerInfo
	startTime     time.Time
	updateChan    chan consensus.ConsensusUpdate

	// Consensus state
	stateTracker *HCSStateTracker

	// Our consensus state
	currentIntentBlockNum   uint64
	currentProposalBlockNum uint64
	isBlockPending          bool
}

func NewHCSEngineImpl(client *hedera.Client, submitPrivateKey hedera.PrivateKey) *HCSEngineImpl {
	return &HCSEngineImpl{client: client, submitPrivateKey: submitPrivateKey}
}

func (self *HCSEngineImpl) Name() string {
	return "hcs"
}

func (self *HCSEngineImpl) Version() string {
	return "0.1"
}

func (self *HCSEngineImpl) Start(startupState consensus.StartupState, service consensus.ConsensusService, updateChan chan consensus.ConsensusUpdate) error {
	self.startupState = startupState
	self.service = service
	self.localPeerInfo = startupState.LocalPeerInfo()
	self.startTime = time.Now()
	self.updateChan = updateChan

	// Set topic and block time from chain settings
	settings, err := self.service.GetSettings(startupState.ChainHead().BlockId(), []string{"sawtooth.consensus.hcs.topic", "sawtooth.consensus.hcs.block_time"})
	if err != nil {
		panic(err)
	}
	self.topic, err = hedera.TopicIDFromString(settings["sawtooth.consensus.hcs.topic"])
	if err != nil {
		panic(err)
	}
	blockTime, err := strconv.Atoi(settings["sawtooth.consensus.hcs.block_time"])
	if err != nil {
		panic(err)
	}
	self.blockTime = time.Duration(blockTime)

	logger.Infof("Using HCS topic ID %v", self.topic)

	// Start state tracker
	self.stateTracker = NewHCSStateTracker(self.topic, self.blockTime, self.client)
	self.stateTracker.Start()

	logger.Info("HCS Engine Started...")

	self.stateTracker.Lock()

	// If HCS doesn't have the genesis block yet
	if startupState.ChainHead().BlockNum() == 0 && !self.stateTracker.HasProposalState(0) {
		msg := structures.HCSEngineTopicMessage{
			Type:   structures.BLOCK_PROPOSAL,
			PeerId: self.localPeerInfo.PeerId().String(),
			BlockProposal: structures.HCSEngineBlockProposal{
				PrevStateProof: "",
				PrevBlockHash:  "",
				BlockHash:      startupState.ChainHead().BlockId().String(),
				BlockNumber:    0,
			},
		}
		self.sendTopicMessage(msg)
	}

	for {
		if !self.stateTracker.HasProposalState(0) {
			self.stateTracker.GetProposalCondition(0).Wait()
		} else {
			break
		}
	}
	self.stateTracker.Unlock()

	go self.generateTicks()
	go self.tickLoop()
	go self.updateLoop()

	return nil
}

func (self *HCSEngineImpl) updateLoop() {
	for {
		logger.Debug("updateLoop()")
		select {
		case n := <-self.updateChan:
			switch notification := n.(type) {
			case consensus.UpdateShutdown:
				logger.Debug(notification)
				return
			case consensus.UpdateBlockNew:
				self.handleBlockNew(notification.Block)
			case consensus.UpdateBlockValid:
				self.handleBlockValid(notification.BlockId)
			case consensus.UpdateBlockInvalid:
				self.handleBlockInvalid(notification.BlockId)
			case consensus.UpdateBlockCommit:
				self.handleBlockCommit(notification.BlockId)
			default:
				logger.Debugf("Unhandled Notification: %v\n", notification)
			}
		}
	}
}

func (self *HCSEngineImpl) generateTicks() {
	ticker := time.NewTicker(time.Second * TICK_TIME_SECONDS)
	for {
		select {
		case <-ticker.C:
			msg := structures.HCSEngineTopicMessage{
				Type:   structures.TIME_TICK,
				PeerId: self.localPeerInfo.PeerId().String(),
			}
			self.sendTopicMessage(msg)
		}
	}
}

func (self *HCSEngineImpl) sendTopicMessage(message structures.HCSEngineTopicMessage) {
	// Marshal the message
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		panic(err)
	}

	// Submit the message
	submitMessage, err := hedera.NewTopicMessageSubmitTransaction().
		SetMessage(jsonMessage).
		SetTopicID(self.topic).
		Sign(self.submitPrivateKey).
		Execute(self.client)

	if err != nil {
		panic(err)
	}

	go func() {
		// Get the receipt of the transaction
		receipt, err := submitMessage.GetReceipt(self.client)
		if err != nil {
			panic(err)
		}

		// Get the transaction status
		// TODO: Figure out what to do if the result is not SUCCESS
		transactionStatus := receipt.Status
		if message.Type != structures.TIME_TICK {
			logger.Debugf("Submitted %s, result %s", message.Type, transactionStatus.String())
		}
	}()

	return
}

func (self *HCSEngineImpl) tickLoop() {
	for {
		logger.Debug("tickLoop()")

		self.stateTracker.Lock()

		self.stateTracker.GetTimeCondition().Wait()
		currentTime := self.stateTracker.GetLatestTime()

		chainHead, err := self.service.GetChainHead()
		if err != nil {
			panic(err)
		}

		maxIntent, err := self.stateTracker.GetMaxIntentState()
		if err != nil {
			fmt.Printf("Could not get max intent: %v", err)
		}
		maxProposal, err := self.stateTracker.GetMaxProposalState()
		if err != nil {
			panic(err)
		}

		self.stateTracker.Unlock()

		if maxIntent != nil {
			logger.Debugf("maxIntent: %d", maxIntent.Intent.BlockNumber)
		}
		logger.Debugf("maxProposal: %d", maxProposal.Proposal.BlockNumber)
		logger.Debugf("currentIntentBlock: %d", self.currentIntentBlockNum)
		logger.Debugf("currentProposalBlock: %d", self.currentProposalBlockNum)
		logger.Debugf("chainHeadBlock: %d", chainHead.BlockNum())

		// TODO: What do if consensus doesn't match chain head?
		// If HCS matches chain head, do an intent.
		if chainHead.BlockId().String() == maxProposal.Proposal.BlockHash &&
			chainHead.BlockNum() == maxProposal.Proposal.BlockNumber &&
			chainHead.BlockNum() >= self.currentIntentBlockNum {

			intentBlockNum := maxProposal.Proposal.BlockNumber + 1
			logger.Debugf("Submitting intent: %d", intentBlockNum)

			// Prepare the topic message
			msg := structures.HCSEngineTopicMessage{
				Type:   structures.BLOCK_INTENT,
				PeerId: self.localPeerInfo.PeerId().String(),
				BlockIntent: structures.HCSEngineBlockIntent{
					BlockNumber:   intentBlockNum,
					PrevBlockHash: maxProposal.Proposal.BlockHash,
				},
			}
			self.currentIntentBlockNum = intentBlockNum
			self.service.InitializeBlock(consensus.NewBlockIdFromString(maxProposal.Proposal.BlockHash))
			self.isBlockPending = true
			self.sendTopicMessage(msg)

			// If we have intent pending and time has elapsed propose a block.
		} else if self.isBlockPending == true &&
			chainHead.BlockNum() < self.currentIntentBlockNum &&
			maxIntent != nil &&
			maxIntent.Intent.BlockNumber == self.currentIntentBlockNum &&
			self.currentIntentBlockNum != self.currentProposalBlockNum &&
			currentTime.After(maxIntent.Message.ConsensusTimestamp.Add(self.blockTime*time.Second)) {

			// Try to summarize and finalize block.
			// If we cannot (block is not ready), cancel the block (do not send a proposal).
			// TODO: Decide if we should send an 'empty' proposal instead to show liveness.

			// Summarize block
			_, err := self.service.SummarizeBlock()
			if err != nil && consensus.IsBlockNotReadyError(err) {
				logger.Debug("Block not ready to summarize...")
				continue
			}

			// Finalize Block
			blockId, err := self.service.FinalizeBlock([]byte{})
			if err != nil && consensus.IsBlockNotReadyError(err) {
				logger.Debug("Block not ready to finalize...")
				continue
			}

			logger.Debugf("Block finalized successfully: %s", blockId.String())
			logger.Debugf("Submitting proposal: %d", self.currentIntentBlockNum)

			// Prepare the topic message
			msg := structures.HCSEngineTopicMessage{
				Type:   structures.BLOCK_PROPOSAL,
				PeerId: self.localPeerInfo.PeerId().String(),
				BlockProposal: structures.HCSEngineBlockProposal{
					PrevStateProof: "",
					PrevBlockHash:  chainHead.BlockId().String(),
					BlockHash:      blockId.String(),
					BlockNumber:    self.currentIntentBlockNum,
				},
			}
			self.currentProposalBlockNum = self.currentIntentBlockNum
			self.isBlockPending = false
			self.sendTopicMessage(msg)
			// If we sent a block intent, but the chain head moved and we never proposed a block, cancel our block.
		} else if self.isBlockPending &&
			chainHead.BlockNum() > self.currentIntentBlockNum {
			logger.Debugf("Canceling block...")
			self.service.CancelBlock()
			self.isBlockPending = false
		}
	}
}

func (self *HCSEngineImpl) waitForProposal(block consensus.Block) *HCSBlockProposalState {
	var proposal *HCSBlockProposalState
	var err error

	self.stateTracker.Lock()
	defer self.stateTracker.Unlock()

	for {
		if self.stateTracker.HasProposalState(block.BlockNum()) {
			proposal, err = self.stateTracker.GetProposalState(block.BlockNum())
			if err != nil {
				panic(err)
			}
			return proposal
		} else {
			self.stateTracker.GetProposalCondition(block.BlockNum()).Wait()
		}
	}
}

func (self *HCSEngineImpl) handleBlockNew(block consensus.Block) {
	logger.Debugf("handleBlockNew: %s", block.BlockId())
	self.service.CheckBlocks([]consensus.BlockId{block.BlockId()})
}

func (self *HCSEngineImpl) handleBlockValid(blockId consensus.BlockId) {
	blocks, err := self.service.GetBlocks([]consensus.BlockId{blockId})
	if err != nil {
		panic(err)
	}
	block := blocks[blockId]

	proposal := self.waitForProposal(block)
	proposalBlockId := consensus.NewBlockIdFromString(proposal.Proposal.BlockHash)
	logger.Debugf("Have a proposal for block %d (%s) P(%s)", block.BlockNum(), block.BlockId().String(), proposalBlockId)

	if block.BlockId() == proposalBlockId {
		logger.Debugf("Have consensus, checking block %d (%s)", block.BlockNum(), block.BlockId().String())
		self.service.CommitBlock(blockId)
	} else {
		logger.Debugf("No consensus, failing block %d (%s)", block.BlockNum(), block.BlockId().String())
		self.service.FailBlock(blockId)
	}
}

func (self *HCSEngineImpl) handleBlockInvalid(blockId consensus.BlockId) {
	logger.Debugf("handleBlockInvalid: %s", blockId)
	logger.Debugf("Failing: %s", blockId)
	self.service.FailBlock(blockId)
}

func (self *HCSEngineImpl) handleBlockCommit(blockId consensus.BlockId) {
	logger.Debugf("handleBlockCommit: %s", blockId)
}
