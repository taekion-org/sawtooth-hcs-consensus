package engine

import (
	"encoding/json"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/hyperledger/sawtooth-sdk-go/consensus"
	"github.com/hyperledger/sawtooth-sdk-go/logging"
	"github.com/taekion-org/sawtooth-hcs-consensus/structures"
	"time"
)

var logger = logging.Get()

const BLOCK_TIME_SECONDS = 20

type HCSEngineImpl struct {
	topic  hedera.TopicID
	client *hedera.Client

	service       consensus.ConsensusService
	startupState  consensus.StartupState
	chainHead     consensus.Block
	localPeerInfo consensus.PeerInfo
	startTime     time.Time
	updateChan    chan consensus.ConsensusUpdate

	// Consensus state
	stateTracker *HCSStateTracker

	// Our consensus state
	currentIntentBlockNum   uint64
	currentProposalBlockNum uint64
}

func NewHCSEngineImpl(topic hedera.TopicID, client *hedera.Client) *HCSEngineImpl {
	return &HCSEngineImpl{topic: topic, client: client}
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
	self.chainHead = startupState.ChainHead()
	self.localPeerInfo = startupState.LocalPeerInfo()
	self.startTime = time.Now()
	self.updateChan = updateChan

	// Start state tracker
	self.stateTracker = NewHCSStateTracker(self.topic, self.client)
	self.stateTracker.Start()

	logger.Info("HCS Engine Started...")

	go self.tickLoop()
	self.mainLoop()

	return nil
}

func (self *HCSEngineImpl) mainLoop() {
	logger.Debug("mainLoop()")
	ticker := time.NewTicker(time.Second * 10)
	//self.service.InitializeBlock(consensus.BLOCK_ID_NULL)

	for {
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
			case consensus.UpdateBlockCommit:
				self.handleBlockCommit(notification.BlockId)
			case consensus.UpdateBlockInvalid:
			default:
				logger.Debug(notification)
			}
		// Time tick
		case <-ticker.C:
			msg := structures.HCSEngineTopicMessage{
				Type:   structures.TIME_TICK,
				PeerId: self.localPeerInfo.PeerId(),
			}
			go self.sendTopicMessage(msg)
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
		Execute(self.client)

	if err != nil {
		panic(err)
	}

	//Get the receipt of the transaction
	receipt, err := submitMessage.GetReceipt(self.client)
	if err != nil {
		panic(err)
	}

	//Get the transaction status
	transactionStatus := receipt.Status

	logger.Debugf("Submitted %s, result %s", message.Type, transactionStatus.String())

	return
}

func (self *HCSEngineImpl) tickLoop() {
	for {
		self.stateTracker.Lock()
		self.stateTracker.GetTimeCondition().Wait()
		currentTime := self.stateTracker.GetLatestTime()

		chainHead, err := self.service.GetChainHead()
		if err != nil {
			panic(err)
		}

		maxIntent, err := self.stateTracker.GetMaxIntentState()
		if err != nil {
			panic(err)
		}
		maxProposal, err := self.stateTracker.GetMaxProposalState()
		if err != nil {
			panic(err)
		}

		// TODO: What do if consensus doesn't match chain head?
		// If HCS matches chain head, do an intent.
		if chainHead.BlockId().String() == maxProposal.Proposal.BlockHash &&
			chainHead.BlockNum() == maxProposal.Proposal.BlockNumber &&
			chainHead.BlockNum() >= self.currentIntentBlockNum {

			intentBlockNum := maxProposal.Proposal.BlockNumber + 1

			// Prepare the topic message
			msg := structures.HCSEngineTopicMessage{
				Type:   structures.BLOCK_INTENT,
				PeerId: self.localPeerInfo.PeerId(),
				BlockIntent: structures.HCSEngineBlockIntent{
					BlockNumber:   intentBlockNum,
					PrevBlockHash: maxProposal.Proposal.BlockHash,
				},
			}
			self.currentIntentBlockNum = intentBlockNum
			self.service.InitializeBlock(consensus.NewBlockIdFromString(maxProposal.Proposal.BlockHash))
			go self.sendTopicMessage(msg)
			// If we have intent pending and time has elapsed propose a block.
		} else if chainHead.BlockNum() < self.currentIntentBlockNum &&
			maxIntent.Intent.BlockNumber == self.currentIntentBlockNum &&
			currentTime.After(maxIntent.Message.ConsensusTimestamp.Add(BLOCK_TIME_SECONDS*time.Second)) {

			// Summarize block
			_, err := self.service.SummarizeBlock()
			if err != nil && consensus.IsBlockNotReadyError(err) {
				logger.Debug("Block not ready to summarize...")
				return
			}

			// Finalize Block
			blockId, err := self.service.FinalizeBlock([]byte{})
			if err != nil && consensus.IsBlockNotReadyError(err) {
				logger.Debug("Block not ready to finalize...")
				return
			}
			logger.Debugf("Block finalized successfully: %s", blockId.String())

			// Prepare the topic message
			msg := structures.HCSEngineTopicMessage{
				Type:   structures.BLOCK_PROPOSAL,
				PeerId: self.localPeerInfo.PeerId(),
				BlockProposal: structures.HCSEngineBlockProposal{
					PrevStateProof: "",
					PrevBlockHash:  self.chainHead.BlockId().String(),
					BlockHash:      blockId.String(),
					BlockNumber:    self.currentIntentBlockNum,
				},
			}
			self.currentProposalBlockNum = self.currentIntentBlockNum
			go self.sendTopicMessage(msg)
		}

		self.stateTracker.Unlock()
	}

	// If consensus is past chain head, wait

	// If consensus is at chain head, signal intent.

	// If we are intending on next block and time passes, send proposal
	// Wait for running proposal

}

func (self *HCSEngineImpl) handleBlockNew(block consensus.Block) {
	logger.Debugf("handleBlockNew: %s", block.BlockId())

	var proposal *HCSBlockProposalState
	var err error

	self.stateTracker.Lock()
	for {
		if !self.stateTracker.HasProposalState(block.BlockNum()) {
			self.stateTracker.GetProposalCondition(block.BlockNum()).Wait()
		} else {
			proposal, err = self.stateTracker.GetProposalState(block.BlockNum())
			if err != nil {
				panic(err)
			}
			break
		}
	}
	self.stateTracker.Unlock()
	logger.Debug("Have a proposal for block %d", block.BlockNum())

	blockId := consensus.NewBlockIdFromString(proposal.Proposal.BlockHash)
	if block.BlockId() == blockId {
		logger.Debug("Have consensus, checking block")
		self.service.CheckBlocks([]consensus.BlockId{blockId})
	} else {
		logger.Debug("No consensus, failing block")
		self.service.FailBlock(blockId)
	}
}

func (self *HCSEngineImpl) handleBlockValid(blockId consensus.BlockId) {
	logger.Debugf("handleBlockValid: %s", blockId)
	/*
		go func() {
			for {
				if _, ok := self.firstBlockProposal[self.pendingBlockNum]; !ok {
					time.Sleep(500 * time.Millisecond)
				} else {
					break
				}
			}

			logger.Debug("Have a valid block...")
			self.service.CommitBlock(blockId)
		}()
	*/
}

func (self *HCSEngineImpl) handleBlockCommit(blockId consensus.BlockId) {
	logger.Debugf("handleBlockCommit: %s", blockId)
	/*
		blocks, err := self.service.GetBlocks([]consensus.BlockId{blockId})
		if err != nil {
			panic(err)
		}
		self.chainHead = blocks[blockId]
	*/
}

//blockId := consensus.NewBlockIdFromString()
//logger.Info("checking blockid ", blockId)
//blocks := []consensus.BlockId{blockId}
//self.service.CheckBlocks(blocks)
