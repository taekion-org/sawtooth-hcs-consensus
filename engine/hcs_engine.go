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
	topicChan     chan hedera.TopicMessage

	// Consensus state
	firstBlockIntent   map[uint64]*structures.HCSEngineTopicMessage
	firstBlockProposal map[uint64]*structures.HCSEngineTopicMessage

	highestBlockNum uint64
	pendingBlockNum uint64

	currentTime time.Time
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

	self.firstBlockIntent = make(map[uint64]*structures.HCSEngineTopicMessage)
	self.firstBlockProposal = make(map[uint64]*structures.HCSEngineTopicMessage)

	err := self.setupTopicChan()
	if err != nil {
		return err
	}

	logger.Info("HCS Engine Started...")
	self.mainLoop()

	return nil
}

func (self *HCSEngineImpl) setupTopicChan() error {
	topicChan := make(chan hedera.TopicMessage, 50)
	_, err := hedera.NewTopicMessageQuery().
		SetTopicID(self.topic).
		Subscribe(self.client, func(message hedera.TopicMessage) {
			topicChan <- message
		})
	if err != nil {
		return err
	}

	self.topicChan = topicChan
	return nil
}

func (self *HCSEngineImpl) mainLoop() {
	logger.Debug("mainLoop()")
	ticker := time.NewTicker(time.Second * 10)
	self.service.InitializeBlock(consensus.BLOCK_ID_NULL)

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

		// Topic messages
		case message := <-self.topicChan:
			self.handleTopicMessage(message)
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

func (self *HCSEngineImpl) handleTopicMessage(message hedera.TopicMessage) {
	var msg structures.HCSEngineTopicMessage
	err := json.Unmarshal(message.Contents, &msg)
	if err != nil {
		panic(err)
	}

	// Copy the timestamp into the internal message type
	msg.HederaTimestamp = message.ConsensusTimestamp

	switch msg.Type {
	case structures.BLOCK_INTENT:
		logger.Debugf("Received %s - Sequence %d - Timestamp %s\n", msg.Type, message.SequenceNumber, message.ConsensusTimestamp)
		blockNum := msg.BlockIntent.BlockNumber

		if _, exists := self.firstBlockIntent[blockNum]; !exists {
			self.firstBlockIntent[blockNum] = &msg
		}

		self.highestBlockNum = blockNum

	case structures.BLOCK_PROPOSAL:
		logger.Debugf("Received %s - Sequence %d - Timestamp %s\n", msg.Type, message.SequenceNumber, message.ConsensusTimestamp)
		blockNum := msg.BlockProposal.BlockNumber
		if blockNum == 0 {
			logger.Debugf("-----Genesis block %s", msg.BlockProposal.BlockHash)
		}

		if _, exists := self.firstBlockProposal[blockNum]; !exists {
			self.firstBlockProposal[blockNum] = &msg
		}

		self.highestBlockNum = blockNum

	case structures.TIME_TICK:
		self.currentTime = message.ConsensusTimestamp
		self.onTick()
	}
}

func (self *HCSEngineImpl) onTick() {
	if self.chainHead.BlockNum() == self.highestBlockNum &&
		self.firstBlockProposal[self.highestBlockNum].BlockProposal.BlockHash == self.chainHead.BlockId().String() &&
		self.highestBlockNum >= self.pendingBlockNum {
		msg := structures.HCSEngineTopicMessage{
			Type:   structures.BLOCK_INTENT,
			PeerId: self.localPeerInfo.PeerId(),
			BlockIntent: structures.HCSEngineBlockIntent{
				BlockNumber:   self.highestBlockNum + 1,
				PrevBlockHash: self.chainHead.BlockId().String(),
			},
		}
		self.pendingBlockNum = self.highestBlockNum + 1
		self.service.InitializeBlock(self.chainHead.BlockId())
		go self.sendTopicMessage(msg)

	} else if _, exists := self.firstBlockIntent[self.pendingBlockNum]; exists &&
		self.currentTime.After(self.firstBlockIntent[self.pendingBlockNum].HederaTimestamp.Add(BLOCK_TIME_SECONDS*time.Second)) {

		if _, exists := self.firstBlockProposal[self.pendingBlockNum]; exists {
			return
		}

		_, err := self.service.SummarizeBlock()
		if err != nil && consensus.IsBlockNotReadyError(err) {
			logger.Debug("block not ready to summarize...")
			return
		}

		blockId, err := self.service.FinalizeBlock([]byte{})
		if err != nil && consensus.IsBlockNotReadyError(err) {
			logger.Debug("block not ready to finalize...")
			return
		}
		logger.Debugf("Block finalized successfully: %s", blockId.String())

		msg := structures.HCSEngineTopicMessage{
			Type:   structures.BLOCK_PROPOSAL,
			PeerId: self.localPeerInfo.PeerId(),
			BlockProposal: structures.HCSEngineBlockProposal{
				PrevStateProof: "",
				PrevBlockHash:  self.chainHead.BlockId().String(),
				BlockHash:      blockId.String(),
				BlockNumber:    self.pendingBlockNum,
			},
		}
		go self.sendTopicMessage(msg)
	}
}

func (self *HCSEngineImpl) handleBlockNew(block consensus.Block) {
	logger.Debugf("handleBlockNew: %s", block.BlockId())

	go func() {
		for {
			if _, ok := self.firstBlockProposal[self.pendingBlockNum]; !ok {
				time.Sleep(500 * time.Millisecond)
			} else {
				break
			}
		}

		logger.Debug("Have a proposal...")
		proposal := self.firstBlockProposal[self.pendingBlockNum].BlockProposal
		blockId := consensus.NewBlockIdFromString(proposal.BlockHash)
		if block.BlockId() == blockId {
			logger.Info("checking block")
			self.service.CheckBlocks([]consensus.BlockId{blockId})
		} else {
			self.service.FailBlock(blockId)
		}
	}()

}

func (self *HCSEngineImpl) handleBlockValid(blockId consensus.BlockId) {
	logger.Debugf("handleBlockValid: %s", blockId)

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

}

func (self *HCSEngineImpl) handleBlockCommit(blockId consensus.BlockId) {
	logger.Debugf("handleBlockCommit: %s", blockId)
	blocks, err := self.service.GetBlocks([]consensus.BlockId{blockId})
	if err != nil {
		panic(err)
	}
	self.chainHead = blocks[blockId]
}

//blockId := consensus.NewBlockIdFromString()
//logger.Info("checking blockid ", blockId)
//blocks := []consensus.BlockId{blockId}
//self.service.CheckBlocks(blocks)
