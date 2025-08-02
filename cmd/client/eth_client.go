package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/crypto/sha3"

	rollupcore "github.com/jakovmitrovski/arbitrum-light-client-go/cmd/client/rollup-core"
)

type EthereumClient struct {
	provider     *ethclient.Client
	rollupCore   *rollupcore.RollupCore
	contractAddr common.Address
}

// NewEthereumClient connects to the chain and initializes the RollupCore binding
func NewEthereumClient(rpcURL string, contractAddr common.Address) (*EthereumClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	core, err := rollupcore.NewRollupCore(contractAddr, client)
	if err != nil {
		return nil, err
	}
	return &EthereumClient{
		provider:     client,
		rollupCore:   core,
		contractAddr: contractAddr,
	}, nil
}

// GetLatestAssertion fetches the latest confirmed assertion hash
func (ec *EthereumClient) GetLatestAssertion(ctx context.Context) ([32]byte, error) {
	return ec.rollupCore.LatestConfirmed(&bind.CallOpts{Context: ctx})
}

// GetAssertionDetails fetches assertion details by hash
func (ec *EthereumClient) GetAssertionDetails(ctx context.Context, hash [32]byte) (*rollupcore.RollupCoreAssertionNode, error) {
	result, err := ec.rollupCore.GetAssertion(&bind.CallOpts{Context: ctx}, hash)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Generic internal helper to get a log by topic
func (ec *EthereumClient) getAssertionLog(ctx context.Context, topic common.Hash, assertionHash common.Hash) (*types.Log, error) {
	// Get latest block
	latestBlock, err := ec.provider.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	var log *types.Log

	for log == nil {
		query := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(latestBlock - 1499)),
			ToBlock:   big.NewInt(int64(latestBlock)),
			Addresses: []common.Address{ec.contractAddr},
			Topics: [][]common.Hash{
				{topic},
				{assertionHash}, // indexed topic1
			},
		}

		logs, err := ec.provider.FilterLogs(ctx, query)
		if err != nil {
			latestBlock -= 1499
			continue
		}
		if len(logs) == 0 {
			latestBlock -= 1499
			continue
		}

		return &logs[0], nil

	}
	return nil, fmt.Errorf("no log found for topic %s", topic.Hex())
}

func (ec *EthereumClient) GetAssertionConfirmedLog(ctx context.Context) (*rollupcore.RollupCoreAssertionConfirmed, error) {
	// 1. Get latest assertion hash
	hash, err := ec.GetLatestAssertion(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Fetch log
	log, err := ec.getAssertionLog(ctx, common.HexToHash("0xfc42829b29c259a7370ab56c8f69fce23b5f351a9ce151da453281993ec0090c"), hash)
	if err != nil {
		return nil, err
	}

	// 3. Parse it
	return ec.rollupCore.ParseAssertionConfirmed(*log)
}

func (ec *EthereumClient) GetAssertionCreatedLog(ctx context.Context) (*rollupcore.RollupCoreAssertionCreated, error) {
	hash, err := ec.GetLatestAssertion(ctx)
	if err != nil {
		return nil, err
	}

	log, err := ec.getAssertionLog(ctx, common.HexToHash("0x901c3aee23cf4478825462caaab375c606ab83516060388344f0650340753630"), hash)
	if err != nil {
		return nil, err
	}

	created, err := ec.rollupCore.ParseAssertionCreated(*log)
	if err != nil {
		return nil, err
	}

	err = ec.ValidateAssertion(ctx, created)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func HashAssertionState(state rollupcore.RollupCoreAssertionState) ([32]byte, error) {
	var hash [32]byte

	// Construct ABI types
	bytes32Arr2Type, err := abi.NewType("bytes32[2]", "", nil)
	if err != nil {
		return hash, err
	}
	uint64Arr2Type, err := abi.NewType("uint64[2]", "", nil)
	if err != nil {
		return hash, err
	}
	uint8Type, err := abi.NewType("uint8", "", nil)
	if err != nil {
		return hash, err
	}
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return hash, err
	}

	args := abi.Arguments{
		{Type: bytes32Arr2Type}, // globalState.bytes32Vals
		{Type: uint64Arr2Type},  // globalState.u64Vals
		{Type: uint8Type},       // machineStatus
		{Type: bytes32Type},     // endHistoryRoot
	}

	packed, err := args.Pack(
		state.GlobalState.Bytes32Vals,
		state.GlobalState.U64Vals,
		state.MachineStatus,
		state.EndHistoryRoot,
	)
	if err != nil {
		return hash, err
	}

	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(packed)
	hasher.Sum(hash[:0])
	return hash, nil
}

func GenerateAssertionHash(prev [32]byte, state rollupcore.RollupCoreAssertionState, inboxAcc [32]byte) ([32]byte, error) {
	stateHash, err := HashAssertionState(state)
	if err != nil {
		return [32]byte{}, err
	}

	buf := bytes.Join([][]byte{
		prev[:],
		stateHash[:],
		inboxAcc[:],
	}, nil)

	var out [32]byte
	d := sha3.NewLegacyKeccak256()
	d.Write(buf)
	d.Sum(out[:0])
	return out, nil
}

func (ec *EthereumClient) ValidateAssertion(ctx context.Context, assertion *rollupcore.RollupCoreAssertionCreated) error {
	// Step 1: Generate assertion hash
	generatedHash, err := GenerateAssertionHash(
		assertion.ParentAssertionHash,
		assertion.Assertion.AfterState,
		assertion.AfterInboxBatchAcc,
	)
	if err != nil {
		return fmt.Errorf("failed to generate assertion hash: %w", err)
	}

	// Step 2: Assert hash matches
	if generatedHash != assertion.AssertionHash {
		return fmt.Errorf("mismatched assertion hash: expected %x, got %x", assertion.AssertionHash, generatedHash)
	}

	// Step 3: Validate assertion hash onchain
	err = ec.rollupCore.ValidateAssertionHash(&bind.CallOpts{Context: ctx},
		generatedHash,
		assertion.Assertion.AfterState,
		assertion.ParentAssertionHash,
		assertion.AfterInboxBatchAcc,
	)
	if err != nil {
		return fmt.Errorf("validateAssertionHash failed: %w", err)
	}

	// Step 4: Validate parent config
	err = ec.rollupCore.ValidateConfig(&bind.CallOpts{Context: ctx},
		assertion.ParentAssertionHash,
		assertion.Assertion.BeforeStateData.ConfigData,
	)
	if err != nil {
		return fmt.Errorf("validateConfig for parent failed: %w", err)
	}

	// Step 5: Build current config
	currentConfig := rollupcore.RollupCoreConfigData{
		WasmModuleRoot:      assertion.WasmModuleRoot,
		RequiredStake:       assertion.RequiredStake,
		ChallengeManager:    assertion.ChallengeManager,
		ConfirmPeriodBlocks: assertion.ConfirmPeriodBlocks,
		NextInboxPosition:   assertion.InboxMaxCount.Uint64(), // same as `.as_limbs()[0]` from Rust
	}

	// Step 6: Validate new config
	err = ec.rollupCore.ValidateConfig(&bind.CallOpts{Context: ctx},
		assertion.AssertionHash,
		currentConfig,
	)
	if err != nil {
		return fmt.Errorf("validateConfig for new assertion failed: %w", err)
	}

	return nil
}
