package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
)

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
	block, err := arbClient.GetBlockByHash(ctx, confirmedLog.BlockHash)
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

}
