package rollupcore

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type GlobalState struct {
	Bytes32Vals [2][32]byte
	U64Vals     [2]uint64
}

type MachineStatus uint8

const (
	MachineStatusRunning MachineStatus = iota
	MachineStatusFinished
	MachineStatusErrored
)

type AssertionState struct {
	GlobalState    GlobalState
	MachineStatus  MachineStatus
	EndHistoryRoot [32]byte
}

type ConfigData struct {
	WasmModuleRoot      [32]byte
	RequiredStake       *big.Int
	ChallengeManager    common.Address
	ConfirmPeriodBlocks uint64
	NextInboxPosition   uint64
}

type BeforeStateData struct {
	PrevPrevAssertionHash [32]byte
	SequencerBatchAcc     [32]byte
	ConfigData            ConfigData
}

type AssertionInputs struct {
	BeforeState     AssertionState
	AfterState      AssertionState
	BeforeStateData BeforeStateData
}
