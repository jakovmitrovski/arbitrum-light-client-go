// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/offchainlabs/nitro/arbos"
	"github.com/offchainlabs/nitro/arbos/arbosState"
	"github.com/offchainlabs/nitro/arbos/arbostypes"

	"github.com/offchainlabs/nitro/arbos/burn"
	"github.com/offchainlabs/nitro/cmd/chaininfo"
)

func replay_message(ctx context.Context, ethereumClient *ethclient.Client, arbClient *ArbitrumClient, lastBlockHeader *types.Header, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header) {
	// wavmio.StubInit()
	// gethhook.RequireHookedGeth()

	// glogger := log.NewGlogHandler(
	// 	log.NewTerminalHandler(io.Writer(os.Stderr), false))
	// glogger.Verbosity(log.LevelError)
	// log.SetDefault(log.NewLogger(glogger))

	// populateEcdsaCaches()

	// lastBlockHash := wavmio.GetLastBlockHash()

	// fmt.Println("lastBlockHash", lastBlockHash)

	statedb, err := arbClient.GetStateDBFromProofs(ctx, message, lastBlockHeader)

	if err != nil {
		panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	}

	// statedb, accessListResults, err := arbClient.GetStateDB(ctx, message, lastBlockHeader)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	// }

	// // Create chain config

	chainConfig := chaininfo.ArbitrumOneChainConfig()

	initMessage := &arbostypes.ParsedInitMessage{
		ChainId:          chainConfig.ChainID,
		InitialL1BaseFee: arbostypes.DefaultInitialL1BaseFee,
		ChainConfig:      chainConfig,
	}

	_, err = arbosState.InitializeArbosState(statedb, burn.NewSystemBurner(nil, false), chainConfig, initMessage)
	if err != nil {
		panic(fmt.Sprintf("Error initializing ArbOS: %v", err.Error()))
	}

	fmt.Println("header.baseFee", lastBlockHeader.BaseFee)

	state, err := arbosState.OpenSystemArbosState(statedb, nil, false)
	if err != nil {
		panic(fmt.Sprintf("Error opening ArbOS state: %v", err.Error()))
	}

	state.L2PricingState().SetBaseFeeWei(big.NewInt(0))

	genesisBlock := arbosState.MakeGenesisBlock(common.Hash{}, 0, 1, statedb.IntermediateRoot(false), chainConfig)

	fmt.Println("genesisBlock.Root()", genesisBlock.Root())
	fmt.Println("state root:", statedb.IntermediateRoot(false))

	chainContext := &SimpleChainContext{chainConfig: chainConfig}

	newBlock, receipts, err := arbos.ProduceBlock(message, 0, genesisBlock.Header(), statedb, chainContext, false, core.MessageReplayMode)
	if err != nil {
		panic(fmt.Sprintf("Error producing block: %v", err.Error()))
	}

	fmt.Println("statedb.IntermediateRoot(false)", statedb.IntermediateRoot(false))
	// statedb.Commit(newBlock.Header().Number.Uint64(), false, false)

	// rebuild_state_root, err := arbClient.RebuildStateRootFromDBAndProofs(ctx, message, expected_block_header, statedb, accessListResults)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error rebuilding state root: %v", err.Error()))
	// }

	// fmt.Println("rebuild_state_root", rebuild_state_root.IntermediateRoot(false))

	for _, receipt := range receipts {
		fmt.Println("receipt", receipt.TxHash, receipt.Status)
	}

	fmt.Println("newBlock.Root()", newBlock.Root())
}
