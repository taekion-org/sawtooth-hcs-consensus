package engine

import "github.com/hyperledger/sawtooth-sdk-go/consensus"

type HCSEngineImpl struct {
	service      consensus.ConsensusService
	startupState consensus.StartupState
	chainHead    consensus.Block
}

func (self *HCSEngineImpl) Name() string {
	return "hcs"
}

func (self *HCSEngineImpl) Version() string {
	return "0.1"
}

func (self *HCSEngineImpl) Start(startupState consensus.StartupState, service consensus.ConsensusService, updateChan chan consensus.ConsensusUpdate) error {
	return nil
}
