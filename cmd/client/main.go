package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/jakovmitrovski/arbitrum-light-client-go/pkg/batchhandler"
	"github.com/joho/godotenv"
	"github.com/offchainlabs/nitro/arbos"
)

type SimpleChainContext struct {
	chainConfig *params.ChainConfig
	header      *types.Header
}

func (c *SimpleChainContext) Engine() consensus.Engine {
	return arbos.Engine{}
}

func (c *SimpleChainContext) GetHeader(hash common.Hash, number uint64) *types.Header {
	return c.header
}

func (c *SimpleChainContext) Config() *params.ChainConfig {
	return c.chainConfig
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	ctx := context.Background()

	ethRpcURL := os.Getenv("ETHEREUM_RPC_URL")
	arbRpcURL := os.Getenv("ARBITRUM_RPC_URL")
	rollupCoreAddr := common.HexToAddress(os.Getenv("ROLLUP_CORE_ADDRESS"))

	accountAddress := common.HexToAddress(os.Getenv("ACCOUNT_ADDRESS"))

	ethClient, err := NewEthereumClient(ethRpcURL, rollupCoreAddr)
	if err != nil {
		log.Fatalf("Failed to init Ethereum client: %v", err)
	}

	arbClient, err := NewArbitrumClient(arbRpcURL) // You’ll need to define this similar to `EthereumClient`
	if err != nil {
		log.Fatalf("Failed to init Arbitrum client: %v", err)
	}

	// 1. Fetch latest confirmed assertion
	latestAssertion, err := ethClient.GetLatestAssertion(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest assertion: %v", err)
	}
	fmt.Printf("Latest Confirmed Assertion: 0x%s\n", hex.EncodeToString(latestAssertion[:]))

	// 2. Fetch assertion node details
	assertionNode, err := ethClient.GetAssertionDetails(ctx, latestAssertion)
	if err != nil {
		log.Fatalf("Failed to get assertion details: %v", err)
	}
	fmt.Printf("Assertion: %+v\n", assertionNode)

	// 3. Get AssertionConfirmed log
	confirmedLog, err := ethClient.GetAssertionConfirmedLog(ctx)
	if err != nil {
		log.Fatalf("Failed to get AssertionConfirmed: %v", err)
	}
	fmt.Printf("AssertionConfirmed: %+v\n", confirmedLog)

	// 4. Get AssertionCreated log
	createdLog, err := ethClient.GetAssertionCreatedLog(ctx)
	if err != nil {
		log.Fatalf("Failed to get AssertionCreated: %v", err)
	}
	fmt.Printf("AssertionCreated: %+v\n", createdLog)

	fmt.Printf("AVM State root: 0x%x\n", createdLog.Assertion.AfterState.EndHistoryRoot)

	// 5. Fetch and verify block from L2
	// block, err := arbClient.GetBlockByHash(ctx, confirmedLog.BlockHash)
	// 2 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xe1ec95df182d2d335e72aae5dcac142210b30c24dbff2a695deb8c0bbf991533"))
	// 3 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0x66aa454cd2c6e38af14d44a1697a0c010da712bae577fdaaf299bd208532e134"))
	block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xb9dc275d50509340a54e252ae6430d3053b4da5ac69c814c183e18f36810b7f0"))
	if err != nil {
		log.Fatalf("Failed to get L2 block by hash: %v", err)
	}

	verified := arbClient.VerifyBlockHash(block.Header(), confirmedLog.BlockHash)
	fmt.Printf("Block hash verified: %v\n", verified)

	// 6. Get and verify state proof
	proof, err := arbClient.GetProof(ctx, *block.Header().Number, accountAddress, []string{})
	if err != nil {
		log.Fatalf("Failed to get proof: %v", err)
	}

	stateVerified := arbClient.VerifyStateProof(block.Header().Root, proof)
	fmt.Printf("Verified: %v\n", stateVerified)
	fmt.Printf("State root: 0x%x\n", block.Header().Root)

	beaconRpcURL := os.Getenv("ETHEREUM_BEACON_RPC_URL")
	arbChainId, err := strconv.ParseUint(os.Getenv("ARBITRUM_ONE_CHAIN_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Error parsing ARBITRUM_CHAIN_ID: %v", err)
	}
	parentRpcURL := os.Getenv("ETHEREUM_RPC_URL")
	// txHash := "0x4e2c7ff3b04c8115834b6edc8d746ffb519d747461e2e0bdd89d102622ec6a5e"
	// txHash := "0x837aae2c9ee352bfdf1ebd74f3f5042e62ef78a3a81d8cf2d6106b0dd7d485dd"
	txHash := "0xcaf788c60948076d6839a9c40524d1eba8a0b6eb4bf62f732d8c98272b7f430d"

	msg, err := batchhandler.StartBatchHandler(ctx, parentRpcURL, txHash, arbChainId, beaconRpcURL)

	if err != nil {
		fmt.Println("err decompressing")
	}

	if err != nil {
		fmt.Println(err)
	}

	// // Create chain config
	// chainConfig := &params.ChainConfig{
	// 	ChainID: big.NewInt(42161), // Arbitrum One
	// 	ArbitrumChainParams: params.ArbitrumChainParams{
	// 		InitialArbOSVersion: params.ArbosVersion_20, // Use the latest version
	// 		GenesisBlockNum:     22207817,               // This should match your state
	// 	},
	// 	// Add other necessary chain parameters
	// }

	// initMessage := &arbostypes.ParsedInitMessage{
	// 	ChainId:          chainConfig.ChainID,
	// 	InitialL1BaseFee: arbostypes.DefaultInitialL1BaseFee,
	// 	ChainConfig:      chainConfig,
	// }

	// 3 expected_block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xb9dc275d50509340a54e252ae6430d3053b4da5ac69c814c183e18f36810b7f0"))
	expected_block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xaa6549925f0a680e76a17c241377e8307accea6c975d5aef968a260062a5a255"))

	replay_message(ctx, ethClient.provider, arbClient, block.Header(), msg, expected_block.Header())

	// // Create chain context
	// chainContext := &SimpleChainContext{chainConfig: chainConfig}

	// // Initialize ArbOS
	// _, err = arbosState.InitializeArbosState(statedb, burn.NewSystemBurner(nil, false), chainConfig, initMessage)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error initializing ArbOS: %v", err.Error()))
	// }

	// // Get initial state root and commit it
	// initialRoot := statedb.IntermediateRoot(true)

	// // Create genesis block with the committed state root
	// genesisBlock := arbosState.MakeGenesisBlock(common.Hash{}, 22207817, 0, initialRoot, chainConfig)

	// // Try to produce the block with the new state database
	// block, _, err1 := arbos.ProduceBlock(msg, 0, genesisBlock.Header(), statedb, chainContext, false, core.MessageReplayMode)
	// if err1 != nil {
	// 	fmt.Println(err1)
	// }

	// stateRoot, err := statedb.Commit(chainConfig.ArbitrumChainParams.GenesisBlockNum, true, false)
	// if err != nil {
	// 	panic(fmt.Sprintf("Error committing state: %v", err.Error()))
	// }

	// fmt.Println("Initial state root:", initialRoot.Hex())
	// fmt.Println("Committed state root:", stateRoot.Hex())
	// fmt.Println("Block state root:", block.Root().Hex())

	fmt.Println(block.Header().Root.Hex())

}
