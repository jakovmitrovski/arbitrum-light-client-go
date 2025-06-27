// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package main

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/offchainlabs/nitro/arbos"
	"github.com/offchainlabs/nitro/arbos/arbosState"
	"github.com/offchainlabs/nitro/arbos/arbostypes"

	"github.com/offchainlabs/nitro/arbos/burn"
	"github.com/offchainlabs/nitro/cmd/chaininfo"
)

func replay_message(ctx context.Context, ethereumClient *ethclient.Client, arbClient *ArbitrumClient, lastBlockHeader *types.Header, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header, chainId uint64, extraMessages ...*arbostypes.L1IncomingMessage) {
	// wavmio.StubInit()
	// gethhook.RequireHookedGeth()

	// glogger := log.NewGlogHandler(
	// 	log.NewTerminalHandler(io.Writer(os.Stderr), false))
	// glogger.Verbosity(log.LevelError)
	// log.SetDefault(log.NewLogger(glogger))

	// populateEcdsaCaches()

	// lastBlockHash := wavmio.GetLastBlockHash()

	// fmt.Println("lastBlockHash", lastBlockHash)

	fmt.Println("lastBlockHeader.Root", lastBlockHeader.Root.Hex())
	fmt.Println("expected_block_header.Root", expected_block_header.Root.Hex())

	statedb, accessListResults, err := arbClient.GetStateDBFromProofs(ctx, message, lastBlockHeader, chainId)

	if err != nil {
		panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	}

	// statedb, accessListResults, err := arbClient.GetStateDB(ctx, message, lastBlockHeader)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	// }

	// // Create chain config

	// chainConfig := chaininfo.ArbitrumOneChainConfig()
	chainConfig := chaininfo.ArbitrumDevTestChainConfig()

	initMessage := &arbostypes.ParsedInitMessage{
		ChainId:          chainConfig.ChainID,
		InitialL1BaseFee: arbostypes.DefaultInitialL1BaseFee,
		ChainConfig:      chainConfig,
	}

	state, err := arbosState.InitializeArbosState(statedb, burn.NewSystemBurner(nil, false), chainConfig, initMessage)
	if err != nil {
		panic(fmt.Sprintf("Error initializing ArbOS: %v", err.Error()))
	}

	fmt.Println("header.baseFee", lastBlockHeader.BaseFee)

	// state, err := arbosState.OpenSystemArbosState(statedb, nil, false)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error opening ArbOS state: %v", err.Error()))
	// }

	// state.L2PricingState().SetBaseFeeWei(big.NewInt(0))

	state.L2PricingState().SetBaseFeeWei(lastBlockHeader.BaseFee)
	// state.L2PricingState().SetMaxPerBlockGasLimit(expected_block_header.GasLimit)

	// _ = arbosState.MakeGenesisBlock(lastBlockHeader.ParentHash, lastBlockHeader.Number.Uint64(), lastBlockHeader.Time, statedb.IntermediateRoot(false), chainConfig)

	// fmt.Println("genesisBlock.Root()", genesisBlock.Root())
	// fmt.Println("state root:", statedb.IntermediateRoot(false))

	chainContext := &SimpleChainContext{chainConfig: chainConfig}

	newBlock, receipts, err := arbos.ProduceBlock(message, 0, lastBlockHeader, statedb, chainContext, false, core.MessageReplayMode)
	if err != nil {
		panic(fmt.Sprintf("Error producing block: %v", err.Error()))
	}

	for _, receipt := range receipts {
		fmt.Println("receipt", receipt.TxHash, receipt.Status)
	}

	fmt.Println("receipts root", newBlock.Header().ReceiptHash.Hex())
	fmt.Println("receipts root", expected_block_header.ReceiptHash.Hex())
	fmt.Println("txs root", newBlock.Header().TxHash.Hex())
	fmt.Println("txs root", expected_block_header.TxHash.Hex())
	fmt.Println("state root", newBlock.Header().Root.Hex())
	fmt.Println("state root", statedb.IntermediateRoot(true).Hex())
	fmt.Println("state root", expected_block_header.Root.Hex())

	rebuild_state_root, err := arbClient.RebuildStateRootFromDBAndProofs(ctx, message, expected_block_header, statedb, chainId, accessListResults)
	if err != nil {
		panic(fmt.Sprintf("Error rebuilding state root: %v", err.Error()))
	}

	fmt.Println("rebuild_state_root", rebuild_state_root.IntermediateRoot(false))

	fmt.Println("newBlock.Root()", newBlock.Root())

	for _, extraMessage := range extraMessages {
		newBlock, receipts, err = arbos.ProduceBlock(extraMessage, 0, newBlock.Header(), statedb, chainContext, false, core.MessageReplayMode)
		if err != nil {
			panic(fmt.Sprintf("Error producing block: %v", err.Error()))
		}
		fmt.Println("FOR LOOP SOLVING....")
		fmt.Println("newBlock.Root()", newBlock.Root())

		for _, receipt := range receipts {
			fmt.Println("receipt", receipt.TxHash, receipt.Status)
			// TODO: manual gas accounting...
			// subtract := uint256.NewInt(0).Mul(uint256.NewInt(uint64(receipt.GasUsed)), uint256.NewInt(uint64(receipt.EffectiveGasPrice.Uint64())))
			// balance := uint256.NewInt(0).Sub(statedb.GetBalance(common.HexToAddress("0x2EB27d9F51D90C45ea735eE3b68E9BE4AE2aB61f")), subtract)
			// statedb.SetBalance(common.HexToAddress("0x2EB27d9F51D90C45ea735eE3b68E9BE4AE2aB61f"), balance, tracing.BalanceChangeUnspecified)
		}

		rebuild_state_root, err := arbClient.RebuildStateRootFromDBAndProofs(ctx, message, expected_block_header, statedb, chainId, accessListResults)
		if err != nil {
			panic(fmt.Sprintf("Error rebuilding state root: %v", err.Error()))
		}

		fmt.Println("rebuild_state_root", rebuild_state_root.IntermediateRoot(false))

		fmt.Println("newBlock.Root()", newBlock.Root())
	}

	// fmt.Println("statedb.IntermediateRoot(false)", statedb.IntermediateRoot(false))
	// statedb.Commit(newBlock.Header().Number.Uint64(), false, false)

}
