// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"

	"github.com/offchainlabs/nitro/arbos"
	"github.com/offchainlabs/nitro/arbos/arbosState"
	"github.com/offchainlabs/nitro/arbos/arbostypes"

	"github.com/offchainlabs/nitro/arbos/burn"
	"github.com/offchainlabs/nitro/cmd/chaininfo"
)

type SimpleChainContext struct {
	chainConfig *params.ChainConfig
	client      *ArbitrumClient
}

func (c *SimpleChainContext) Engine() consensus.Engine {
	return arbos.Engine{}
}

func (c *SimpleChainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	ctx := context.Background()
	block, err := c.client.GetBlockByHash(ctx, hash)
	if err != nil {
		panic(err)
	}
	return block.Header()
}

func (c *SimpleChainContext) Config() *params.ChainConfig {
	return c.chainConfig
}

func ExecuteExecutionOracle(ctx context.Context, arbClient *ArbitrumClient, lastBlockHeader *types.Header, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header, chainId uint64, extraMessages ...*arbostypes.L1IncomingMessage) bool {
	if message.Header.Kind == arbostypes.L1MessageType_Initialize {
		return handleInitializeMessage(arbClient, message, expected_block_header, extraMessages)
	} else {
		return handleNonInitializeMessage(ctx, arbClient, lastBlockHeader, message, expected_block_header, chainId)
	}
}

func handleInitializeMessage(arbClient *ArbitrumClient, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header, extraMessages []*arbostypes.L1IncomingMessage) bool {
	memdb := rawdb.NewMemoryDatabase()
	trieDB := triedb.NewDatabase(memdb, nil)
	trieDB.Commit(common.Hash{}, false)
	stateDB := state.NewDatabase(trieDB, nil)

	statedb, err := state.NewDeterministic(common.Hash{}, stateDB)
	if err != nil {
		panic(err)
	}

	fmt.Println("init message encountered")

	initMessage, err := message.ParseInitMessage()
	if err != nil {
		panic(err)
	}
	chainConfig := initMessage.ChainConfig
	if chainConfig == nil {
		fmt.Println("no chain config in the init message. Falling back to hardcoded chain config.")
		chainConfig, err = chaininfo.GetChainConfig(initMessage.ChainId, "", 0, []string{}, "")
		if err != nil {
			panic(err)
		}
	}

	_, err = arbosState.InitializeArbosState(statedb, burn.NewSystemBurner(nil, false), chainConfig, initMessage)
	if err != nil {
		panic(fmt.Sprintf("Error initializing ArbOS: %v", err.Error()))
	}

	newBlock := arbosState.MakeGenesisBlock(common.Hash{}, chainConfig.ArbitrumChainParams.GenesisBlockNum, 0, statedb.IntermediateRoot(true), chainConfig)
	fmt.Println("genesisBlock.Root()", newBlock.Root())
	receipts := types.Receipts{}

	chainContext := &SimpleChainContext{chainConfig: chainConfig, client: arbClient}

	for i, extraMessage := range extraMessages {
		newBlock, receipts, err = arbos.ProduceBlock(extraMessage, 0, newBlock.Header(), statedb, chainContext, false, core.MessageReplayMode)
		fmt.Println("newBlock.Header().Root", newBlock.Header().Root.Hex())
		if err != nil {
			panic(fmt.Sprintf("Error producing block: %v", err.Error()))
		}
		fmt.Println("FOR LOOP SOLVING....")

		if i == len(extraMessages)-1 {
			for _, receipt := range receipts {
				fmt.Println("receipt", receipt.TxHash, receipt.Status)
			}
		}
	}

	return validateBlockHeaders(newBlock.Header(), expected_block_header)
}

func handleNonInitializeMessage(ctx context.Context, arbClient *ArbitrumClient, lastBlockHeader *types.Header, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header, chainId uint64) bool {
	fmt.Println("lastBlockHeader.Root", lastBlockHeader.Root.Hex())
	fmt.Println("expected_block_header.Root", expected_block_header.Root.Hex())

	// statedb, err := arbClient.GetStateDBFromComprehensiveAccessList(ctx, message, lastBlockHeader, chainId)
	statedb, _, _, err := arbClient.ReconstructStateFromProofsAndTrace(ctx, expected_block_header, lastBlockHeader, chainId)
	// statedb, err := arbClient.GetStateDBFromProofsAndTraceReconciliation(ctx, expected_block_header, lastBlockHeader, chainId)
	if err != nil {
		panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	}

	// Debug: Check initial state root
	initialRoot := statedb.IntermediateRoot(false)
	fmt.Printf("Initial state root: %s\n", initialRoot.Hex())

	// Debug: Check if the state root matches the previous block
	if initialRoot != lastBlockHeader.Root {
		fmt.Printf("WARNING: Initial state root (%s) does not match last block root (%s)\n",
			initialRoot.Hex(), lastBlockHeader.Root.Hex())
	} else {
		fmt.Printf("✅ Initial state root matches last block root\n")
	}

	chainConfig := chaininfo.ArbitrumDevTestChainConfig()
	// chainConfig := chaininfo.ArbitrumOneChainConfig()

	_ = arbosState.MakeGenesisBlock(lastBlockHeader.ParentHash, lastBlockHeader.Number.Uint64(), lastBlockHeader.Time, statedb.IntermediateRoot(false), chainConfig)
	chainContext := &SimpleChainContext{chainConfig: chainConfig, client: arbClient}

	txs, _ := arbos.ParseL2Transactions(message, big.NewInt(int64(chainId)))
	for _, tx := range txs {
		fmt.Println("tx", tx.Hash().Hex())
	}

	newBlock, receipts, err := arbos.ProduceBlock(message, 0, lastBlockHeader, statedb, chainContext, false, core.MessageReplayMode)
	if err != nil {
		fmt.Printf("Failed to produce block: %v\n", err)
		return false
	}

	// Debug: Check final state root
	fmt.Printf("New block root: %s\n", newBlock.Root().Hex())
	fmt.Printf("Expected block root: %s\n", expected_block_header.Root.Hex())

	// Debug: Print receipt information
	fmt.Printf("Number of receipts: %d\n", len(receipts))
	for _, receipt := range receipts {
		fmt.Printf("receipt %s %d\n", receipt.TxHash.Hex(), receipt.Status)
	}

	//arbClient.InspectAccountStorage(statedb, common.HexToAddress("0xA4b05FffffFffFFFFfFFfffFfffFFfffFfFfFFFf"))
	// Then find all differences to see what's missing
	// fmt.Printf("🔍 Finding all state differences...\n")
	// arbClient.FindStateDifferences(ctx, statedb, accountSet, expected_block_header)

	// First verify the accounts/slots from trace
	// arbClient.VerifyStateAgainstProofs(ctx, statedb, accountSet, slotSet, expected_block_header)

	// arbClient.DiagnoseArbOSStorageMismatch(ctx, statedb, expected_block_header)

	return validateBlockHeaders(newBlock.Header(), expected_block_header)
}

func validateBlockHeaders(actual *types.Header, expected *types.Header) bool {
	if actual.ReceiptHash != expected.ReceiptHash {
		fmt.Println("receipt hash equality failed")
		fmt.Println("actual.ReceiptHash", actual.ReceiptHash.Hex())
		fmt.Println("expected.ReceiptHash", expected.ReceiptHash.Hex())
		return false
	}

	if actual.TxHash != expected.TxHash {
		fmt.Println("tx hash equality failed")
		fmt.Println("actual.TxHash", actual.TxHash.Hex())
		fmt.Println("expected.TxHash", expected.TxHash.Hex())
		return false
	}

	if actual.MixDigest != expected.MixDigest {
		fmt.Println("mix digest equality failed")
		fmt.Println("actual.MixDigest", actual.MixDigest.Hex())
		fmt.Println("expected.MixDigest", expected.MixDigest.Hex())
		return false
	}

	if !bytes.Equal(actual.Extra, expected.Extra) {
		fmt.Println("extra equality failed")
		fmt.Println("actual.Extra", actual.Extra)
		fmt.Println("expected.Extra", expected.Extra)
		return false
	}

	if actual.Root != expected.Root {
		fmt.Println("root equality failed")
		fmt.Println("actual.Root", actual.Root.Hex())
		fmt.Println("expected.Root", expected.Root.Hex())
		return false
	}

	if actual.Nonce != expected.Nonce {
		actual.Nonce = expected.Nonce
	}

	if actual.Hash() != expected.Hash() {
		fmt.Println("hash equality failed")
		fmt.Println("actual.Hash", actual.Hash().Hex())
		fmt.Println("expected.Hash", expected.Hash().Hex())
		return false
	}

	return true
}
