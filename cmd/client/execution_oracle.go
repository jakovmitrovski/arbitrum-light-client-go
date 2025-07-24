// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package main

import (
	"bytes"
	"context"
	"fmt"

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
	receipts := types.Receipts{}

	chainContext := &SimpleChainContext{chainConfig: chainConfig, client: arbClient}

	for i, extraMessage := range extraMessages {
		newBlock, receipts, err = arbos.ProduceBlock(extraMessage, 0, newBlock.Header(), statedb, chainContext, false, core.MessageReplayMode)
		if err != nil {
			panic(fmt.Sprintf("Error producing block: %v", err.Error()))
		}

		if i == len(extraMessages)-1 {
			for _, receipt := range receipts {
				fmt.Println("receipt", receipt.TxHash, receipt.Status)
			}
		}
	}

	return validateBlockHeaders(newBlock.Header(), expected_block_header)
}

func handleNonInitializeMessage(ctx context.Context, arbClient *ArbitrumClient, lastBlockHeader *types.Header, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header, chainId uint64) bool {
	statedb, _, _, err := arbClient.ReconstructStateFromProofsAndTrace(ctx, expected_block_header, lastBlockHeader, chainId)
	if err != nil {
		panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	}

	chainConfig := chaininfo.ArbitrumDevTestChainConfig()

	_ = arbosState.MakeGenesisBlock(lastBlockHeader.ParentHash, lastBlockHeader.Number.Uint64(), lastBlockHeader.Time, statedb.IntermediateRoot(false), chainConfig)
	chainContext := &SimpleChainContext{chainConfig: chainConfig, client: arbClient}

	newBlock, _, err := arbos.ProduceBlock(message, 0, lastBlockHeader, statedb, chainContext, false, core.MessageReplayMode)
	if err != nil {
		fmt.Printf("Failed to produce block: %v\n", err)
		return false
	}

	// THIS IS FOR DEBUGGING PURPOSES ONLY.
	// fmt.Printf("New block root: %s\n", newBlock.Root().Hex())
	// fmt.Printf("Expected block root: %s\n", expected_block_header.Root.Hex())

	// Debug: Print receipt information
	// fmt.Printf("Number of receipts: %d\n", len(receipts))
	// for _, receipt := range receipts {
	// 	fmt.Printf("receipt %s %d\n", receipt.TxHash.Hex(), receipt.Status)
	// }

	// fmt.Printf("🔍 Finding all state differences...\n")
	// arbClient.FindStateDifferences(ctx, statedb, accountSet, expected_block_header)
	// arbClient.VerifyStateAgainstProofs(ctx, statedb, accountSet, slotSet, expected_block_header)

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
