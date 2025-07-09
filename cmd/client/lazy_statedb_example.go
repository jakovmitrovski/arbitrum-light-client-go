package main

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/offchainlabs/nitro/arbos"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
)

// ExampleLazyStateDBUsage demonstrates how to use the lazy StateDB approach
func ExampleLazyStateDBUsage(ctx context.Context, client *ArbitrumClient, msg *arbostypes.L1IncomingMessage, header *types.Header, chainId uint64) error {
	fmt.Println("🚀 Using Lazy StateDB approach for state reconstruction...")

	// Step 1: Create lazy StateDB
	lazyStateDB := client.GetLazyStateDB(ctx, header)

	// Step 2: Parse L2 transactions
	txs, err := arbos.ParseL2Transactions(msg, big.NewInt(int64(chainId)))
	if err != nil {
		return fmt.Errorf("failed to parse L2 transactions: %w", err)
	}

	fmt.Printf("📝 Processing %d transactions with lazy StateDB\n", len(txs))

	// Step 3: Execute transactions - the lazy StateDB will automatically fetch proofs as needed
	for i, tx := range txs {
		fmt.Printf("Processing transaction %d: %s\n", i, tx.Hash().Hex())

		// Get sender address
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			fmt.Printf("Warning: failed to get sender for tx %d: %v\n", i, err)
			continue
		}

		// Access sender balance - this will trigger lazy loading if not already loaded
		balance := lazyStateDB.GetBalance(from)
		fmt.Printf("  Sender %s balance: %s\n", from.Hex(), balance.String())

		// Access sender nonce - this will trigger lazy loading if not already loaded
		nonce := lazyStateDB.GetNonce(from)
		fmt.Printf("  Sender %s nonce: %d\n", from.Hex(), nonce)

		// If there's a "to" address, access its state too
		if tx.To() != nil {
			toBalance := lazyStateDB.GetBalance(*tx.To())
			fmt.Printf("  Recipient %s balance: %s\n", tx.To().Hex(), toBalance.String())

			toNonce := lazyStateDB.GetNonce(*tx.To())
			fmt.Printf("  Recipient %s nonce: %d\n", tx.To().Hex(), toNonce)

			// Check if recipient has code (contract)
			code := lazyStateDB.GetCode(*tx.To())
			if len(code) > 0 {
				fmt.Printf("  Recipient %s has code (%d bytes)\n", tx.To().Hex(), len(code))
			}
		}

		// Simulate some state changes (in a real implementation, you'd execute the transaction)
		// For this example, we'll just demonstrate that the lazy loading works
		fmt.Printf("  Transaction %d processed successfully\n", i)
	}

	// Step 4: Check final state root
	finalRoot := lazyStateDB.IntermediateRoot(true)
	fmt.Printf("📊 Final state root: %s\n", finalRoot.Hex())
	fmt.Printf("📊 Expected state root: %s\n", header.Root.Hex())

	if finalRoot == header.Root {
		fmt.Println("✅ State roots match! Lazy StateDB approach successful!")
	} else {
		fmt.Printf("❌ State root mismatch: got %s, expected %s\n", finalRoot.Hex(), header.Root.Hex())
	}

	// Step 5: Show statistics about what was loaded
	fmt.Printf("📈 Lazy loading statistics:\n")
	fmt.Printf("  - Accounts accessed: %d\n", len(lazyStateDB.accountCache))

	totalStorageSlots := 0
	for _, storageMap := range lazyStateDB.storageCache {
		totalStorageSlots += len(storageMap)
	}
	fmt.Printf("  - Storage slots accessed: %d\n", totalStorageSlots)

	return nil
}

// CompareApproaches compares the lazy StateDB approach with the comprehensive access list approach
func CompareApproaches(ctx context.Context, client *ArbitrumClient, msg *arbostypes.L1IncomingMessage, header *types.Header, chainId uint64) error {
	fmt.Println("🔍 Comparing StateDB approaches...")

	// Approach 1: Comprehensive Access List (current approach)
	fmt.Println("\n📋 Approach 1: Comprehensive Access List")
	start := time.Now()
	comprehensiveStateDB, err := client.GetStateDBFromComprehensiveAccessList(ctx, msg, header, chainId)
	comprehensiveTime := time.Since(start)
	if err != nil {
		fmt.Printf("❌ Comprehensive approach failed: %v\n", err)
	} else {
		fmt.Printf("✅ Comprehensive approach successful in %v\n", comprehensiveTime)
	}

	// Approach 2: Lazy StateDB (new approach)
	fmt.Println("\n🦥 Approach 2: Lazy StateDB")
	start = time.Now()
	lazyStateDB := client.GetLazyStateDB(ctx, header)

	// Process transactions with lazy loading
	txs, err := arbos.ParseL2Transactions(msg, big.NewInt(int64(chainId)))
	if err != nil {
		return fmt.Errorf("failed to parse L2 transactions: %w", err)
	}

	for _, tx := range txs {
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err == nil {
			lazyStateDB.GetBalance(from)
			lazyStateDB.GetNonce(from)
		}

		if tx.To() != nil {
			lazyStateDB.GetBalance(*tx.To())
			lazyStateDB.GetNonce(*tx.To())
			lazyStateDB.GetCode(*tx.To())
		}
	}

	lazyTime := time.Since(start)
	fmt.Printf("✅ Lazy StateDB approach successful in %v\n", lazyTime)

	// Compare results
	if comprehensiveStateDB != nil {
		comprehensiveRoot := comprehensiveStateDB.IntermediateRoot(true)
		lazyRoot := lazyStateDB.IntermediateRoot(true)
		expectedRoot := header.Root

		fmt.Printf("\n📊 Results Comparison:\n")
		fmt.Printf("  Expected root: %s\n", expectedRoot.Hex())
		fmt.Printf("  Comprehensive root: %s\n", comprehensiveRoot.Hex())
		fmt.Printf("  Lazy root: %s\n", lazyRoot.Hex())

		comprehensiveMatch := comprehensiveRoot == expectedRoot
		lazyMatch := lazyRoot == expectedRoot

		fmt.Printf("\n🎯 Accuracy:\n")
		fmt.Printf("  Comprehensive approach: %s\n", map[bool]string{true: "✅ Match", false: "❌ Mismatch"}[comprehensiveMatch])
		fmt.Printf("  Lazy approach: %s\n", map[bool]string{true: "✅ Match", false: "❌ Mismatch"}[lazyMatch])

		fmt.Printf("\n⏱️ Performance:\n")
		fmt.Printf("  Comprehensive approach: %v\n", comprehensiveTime)
		fmt.Printf("  Lazy approach: %v\n", lazyTime)
		fmt.Printf("  Speedup: %.2fx\n", float64(comprehensiveTime)/float64(lazyTime))
	}

	return nil
}
