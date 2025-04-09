package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

// ArbitrumClient wraps an Ethereum JSON-RPC client with extra methods
type ArbitrumClient struct {
	ethClient *ethclient.Client
	rpcClient *rpc.Client
}

// NewArbitrumClient initializes a new ArbitrumClient
func NewArbitrumClient(rpcURL string) (*ArbitrumClient, error) {
	ethClient, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	rpcClient, err := rpc.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	return &ArbitrumClient{ethClient: ethClient, rpcClient: rpcClient}, nil
}

// GetBlockByHash fetches a block by its hash
func (c *ArbitrumClient) GetBlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	block, err := c.ethClient.BlockByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("block not found: %w", err)
	}
	return block, nil
}

// EthGetProofResult maps the response of eth_getProof
type EthGetProofResult struct {
	Address       string   `json:"address"`
	Balance       string   `json:"balance"`
	Nonce         string   `json:"nonce"`
	CodeHash      string   `json:"codeHash"`
	StorageHash   string   `json:"storageHash"`
	AccountProof  []string `json:"accountProof"`
	StorageProofs []struct {
		Key   string   `json:"key"`
		Value string   `json:"value"`
		Proof []string `json:"proof"`
	} `json:"storageProof"`
}

// GetProof calls eth_getProof to get state and storage proof
func (c *ArbitrumClient) GetProof(ctx context.Context, blockNumber big.Int, address common.Address, storageKeys []string) (*EthGetProofResult, error) {
	var result EthGetProofResult
	blockHex := fmt.Sprintf("0x%x", blockNumber.Uint64())
	args := []interface{}{address.Hex(), storageKeys, blockHex}

	err := c.rpcClient.CallContext(ctx, &result, "eth_getProof", args...)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// VerifyBlockHash computes the RLP hash of a header and compares it
func (c *ArbitrumClient) VerifyBlockHash(header *types.Header, expectedHash common.Hash) bool {
	encoded, err := rlp.EncodeToBytes(header)
	if err != nil {
		fmt.Println("RLP encoding failed:", err)
		return false
	}
	actualHash := crypto.Keccak256Hash(encoded)
	return actualHash == expectedHash
}

// VerifyProof is a stub — full MPT proof verification isn't in go-ethereum directly
// func (c *ArbitrumClient) VerifyProof(stateRoot common.Hash, proof *EthGetProofResult) bool {
// 	// Note: This is a placeholder. Full MPT proof verification would require custom implementation.
// 	fmt.Println("🔒 Verifying proof against state root:", stateRoot.Hex())

// 	// Decode account and storage values to verify manually if needed
// 	balance, _ := new(big.Int).SetString(proof.Balance[2:], 16)
// 	nonce, _ := new(big.Int).SetString(proof.Nonce[2:], 16)

// 	fmt.Printf("Account: balance=%s, nonce=%s, codeHash=%s\n", balance.String(), nonce.String(), proof.CodeHash)

// 	for _, sp := range proof.StorageProofs {
// 		fmt.Printf("Storage key %s = %s\n", sp.Key, sp.Value)
// 		// Normally, you'd validate Merkle proofs here
// 	}

// 	fmt.Println("⚠️ Proof validation not implemented — only structure checked.")
// 	return true
// }

func (c *ArbitrumClient) VerifyStateProof(stateRoot common.Hash, proof *EthGetProofResult) bool {
	addr := common.HexToAddress(proof.Address)
	key := crypto.Keccak256(addr.Bytes()) // MPT key for the address

	for _, sp := range proof.StorageProofs {
		fmt.Println("🔍 Verifying storage slot:", sp.Key)

		// Hash the storage slot key
		slotKeyBytes, err := hex.DecodeString(trimHexPrefix(sp.Key))
		if err != nil {
			fmt.Println("❌ Invalid storage key:", err)
			return false
		}
		slotKeyHash := crypto.Keccak256(slotKeyBytes)

		// Build a new proof DB for this slot
		storageDB := memorydb.New()
		for _, encodedNode := range sp.Proof {
			nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
			if err != nil {
				fmt.Println("❌ Failed to decode storage proof node:", err)
				return false
			}
			hash := crypto.Keccak256Hash(nodeBytes)
			if err := storageDB.Put(hash.Bytes(), nodeBytes); err != nil {
				fmt.Println("❌ Failed to insert storage proof node:", err)
				return false
			}
		}

		// Decode expected value
		expectedValue := new(big.Int)
		expectedValue.SetString(trimHexPrefix(sp.Value), 16)

		// Verify proof
		val, err := trie.VerifyProof(common.HexToHash(proof.StorageHash), slotKeyHash, storageDB)
		if err != nil {
			fmt.Println("❌ Storage proof verification failed:", err)
			return false
		}

		// Decode and compare value
		var decodedValue *big.Int
		if len(val) == 0 {
			decodedValue = big.NewInt(0)
		} else {
			decodedValue = new(big.Int)
			if err := rlp.DecodeBytes(val, decodedValue); err != nil {
				fmt.Println("❌ Failed to decode storage value:", err)
				return false
			}
		}

		if decodedValue.Cmp(expectedValue) != 0 {
			fmt.Printf("❌ Storage slot mismatch: got %s, expected %s\n", decodedValue, expectedValue)
			return false
		}

		fmt.Printf("✅ Verified slot %x = %s\n", slotKeyHash, decodedValue)
	}

	// Build the proof DB
	proofDB := memorydb.New()
	for _, encodedNode := range proof.AccountProof {
		nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
		if err != nil {
			fmt.Println("❌ Failed to decode node:", err)
			return false
		}
		hash := crypto.Keccak256Hash(nodeBytes)
		if err := proofDB.Put(hash.Bytes(), nodeBytes); err != nil {
			fmt.Println("❌ Failed to insert into DB:", err)
			return false
		}
	}

	// Verify proof
	val, err := trie.VerifyProof(stateRoot, key, proofDB)
	if err != nil {
		fmt.Println("❌ Proof verification failed:", err)
		return false
	}

	// Decode the RLP into expected Ethereum account format: [nonce, balance, storageRoot, codeHash]
	var decoded struct {
		Nonce       *big.Int
		Balance     *big.Int
		StorageRoot []byte
		CodeHash    []byte
	}

	if err := rlp.DecodeBytes(val, &decoded); err != nil {
		fmt.Println("❌ Failed to decode account value:", err)
		return false
	}

	// Decode expected values from JSON result
	jsonNonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
	jsonBalance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
	jsonCodeHash := common.HexToHash(proof.CodeHash)

	// Compare actual trie values with json-rpc values
	matches := decoded.Nonce.Cmp(jsonNonce) == 0 &&
		decoded.Balance.Cmp(jsonBalance) == 0 &&
		common.BytesToHash(decoded.CodeHash) == jsonCodeHash

	if matches {
		fmt.Println("✅ Account proof verified!")
	} else {
		fmt.Println("❌ Account data mismatch!")
		fmt.Printf("Decoded: nonce=%v, balance=%v, codeHash=%x\n", decoded.Nonce, decoded.Balance, decoded.CodeHash)
		fmt.Printf("Expected: nonce=%v, balance=%v, codeHash=%s\n", jsonNonce, jsonBalance, jsonCodeHash.Hex())
	}

	return matches
}

func trimHexPrefix(s string) string {
	if len(s) >= 2 && s[:2] == "0x" {
		return s[2:]
	}
	return s
}
