// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/OffchainLabs/nitro/blob/master/LICENSE.md

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
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
		return handleNonInitializeMessage(ctx, arbClient, lastBlockHeader, message, expected_block_header, chainId, extraMessages)
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

func handleNonInitializeMessage(ctx context.Context, arbClient *ArbitrumClient, lastBlockHeader *types.Header, message *arbostypes.L1IncomingMessage, expected_block_header *types.Header, chainId uint64, extraMessages []*arbostypes.L1IncomingMessage) bool {
	fmt.Println("lastBlockHeader.Root", lastBlockHeader.Root.Hex())
	fmt.Println("expected_block_header.Root", expected_block_header.Root.Hex())

	statedb, err := arbClient.GetStateDBFromComprehensiveAccessList(ctx, message, lastBlockHeader, chainId)
	if err != nil {
		panic(fmt.Sprintf("Error opening state db: %v", err.Error()))
	}

	chainConfig := chaininfo.ArbitrumDevTestChainConfig()
	chainConfig.ArbitrumChainParams.InitialArbOSVersion = binary.BigEndian.Uint64(expected_block_header.MixDigest.Bytes()[16:24])

	initMessage := &arbostypes.ParsedInitMessage{
		ChainId:          chainConfig.ChainID,
		InitialL1BaseFee: expected_block_header.BaseFee,
		ChainConfig:      chainConfig,
	}

	_, err = arbosState.InitializeArbosState(statedb, burn.NewSystemBurner(nil, false), chainConfig, initMessage)
	if err != nil {
		panic(fmt.Sprintf("Error initializing ArbOS: %v", err.Error()))
	}

	_ = arbosState.MakeGenesisBlock(lastBlockHeader.ParentHash, lastBlockHeader.Number.Uint64(), lastBlockHeader.Time, statedb.IntermediateRoot(true), chainConfig)
	chainContext := &SimpleChainContext{chainConfig: chainConfig, client: arbClient}

	newBlock, receipts, err := arbos.ProduceBlock(message, 0, lastBlockHeader, statedb, chainContext, false, core.MessageReplayMode)
	if err != nil {
		fmt.Printf("Error producing block: %v\n", err.Error())
		panic(fmt.Sprintf("Error producing block: %v", err.Error()))
	}

	for _, receipt := range receipts {
		fmt.Println("receipt", receipt.TxHash, receipt.Status)
	}

	for i, extraMessage := range extraMessages {
		newBlock, receipts, err = arbos.ProduceBlock(extraMessage, 0, newBlock.Header(), statedb, chainContext, false, core.MessageReplayMode)
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

func validateBlockHeaders(actual *types.Header, expected *types.Header) bool {
	if actual.Root != expected.Root {
		fmt.Println("root equality failed")
		fmt.Println("actual.Root", actual.Root.Hex())
		fmt.Println("expected.Root", expected.Root.Hex())
		return false
	}

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

	return true
}

func debugStateDBContents(statedb *state.StateDB) {
	fmt.Printf("=== State DB Contents ===\n")

	if statedb == nil {
		fmt.Printf("StateDB is nil!\n")
		return
	}

	// Check the trie
	trie := statedb.GetTrie()
	if trie == nil {
		fmt.Printf("Trie is nil!\n")
	} else {
		fmt.Printf("Trie hash: %s\n", trie.Hash().Hex())
	}

	// Check if there are pending changes
	fmt.Printf("Has pending changes: %t\n", statedb.HasSelfDestructed(common.Address{})) // This checks if there are any pending changes

	// Try to get account data for known addresses
	testAddresses := []common.Address{
		common.HexToAddress("0x2EB27d9F51D90C45ea735eE3b68E9BE4AE2aB61f"),
		common.HexToAddress("0xC3c76AaAA7C483c5099aeC225bA5E4269373F16b"),
		// Add other addresses you know should exist
	}

	for _, addr := range testAddresses {
		balance := statedb.GetBalance(addr)
		nonce := statedb.GetNonce(addr)
		codeHash := statedb.GetCodeHash(addr)
		storageRoot := statedb.GetStorageRoot(addr)

		fmt.Printf("Account %s:\n", addr.Hex())
		fmt.Printf("  Balance: %s\n", balance.String())
		fmt.Printf("  Nonce: %d\n", nonce)
		fmt.Printf("  CodeHash: %s\n", codeHash.Hex())
		fmt.Printf("  StorageRoot: %s\n", storageRoot.Hex())
		fmt.Printf("  Code: %s\n", hex.EncodeToString(statedb.GetCode(addr)))
		fmt.Printf("  Exists: %t\n", statedb.Exist(addr))
	}

	// Dump BEFORE finalizing and committing
	fmt.Println("json", statedb.Dump(nil))

	// Check the intermediate root
	root := statedb.IntermediateRoot(false)

	// Only finalize and commit AFTER dumping
	statedb.Finalise(false)
	statedb.Commit(0, false, false)
	fmt.Printf("Intermediate root: %s\n", root.Hex())
}
