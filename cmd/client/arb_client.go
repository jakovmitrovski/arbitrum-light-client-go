package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
	"github.com/offchainlabs/nitro/arbos"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
	"github.com/offchainlabs/nitro/arbos/l1pricing"
	"github.com/offchainlabs/nitro/arbos/l2pricing"
)

// ArbitrumClient wraps an Ethereum JSON-RPC client with extra methods
type ArbitrumClient struct {
	ethClient *ethclient.Client
	rpcClient *rpc.Client
}

// L2 Tracking
type MessageTrackingL2Data struct {
	L2BlockNumber uint64
	L2BlockHash   common.Hash
}

type MessageTrackingL1Data struct {
	Message      arbostypes.L1IncomingMessage
	L1TxHash     common.Hash
	DataLocation uint8
}

type StateAtReturnData struct {
	Message                arbostypes.L1IncomingMessage
	L2BlockNumber          uint64
	L2BlockHash            common.Hash
	L1PricingState         l1pricing.L1PricingState
	L2PricingState         l2pricing.L2PricingState
	BrotliCompressionLevel uint64

	L1TxHash     common.Hash
	DataLocation uint8
}

type L1Index struct {
	StateIndex uint64
}

const ARBITRUM_ONE_GENESIS_BLOCK = 22207817

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

func (c *ArbitrumClient) GetBlockByNumber(ctx context.Context, blockNumber *big.Int) (*types.Block, error) {
	block, err := c.ethClient.BlockByNumber(ctx, blockNumber)
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

type AccessList struct {
	Address     common.Address
	StorageKeys []common.Hash
}

type TransactionArgs struct {
	From  *common.Address `json:"from"`
	To    *common.Address `json:"to"`
	Data  hexutil.Bytes   `json:"data"`
	Value *hexutil.Big    `json:"value"`
	Gas   *hexutil.Big    `json:"gas"`
	Type  *uint64         `json:"type"`
	// GasPrice             *hexutil.Big    `json:"gasPrice,omitempty"`
	MaxPriorityFeePerGas *hexutil.Big `json:"maxPriorityFeePerGas,omitempty"`
	MaxFeePerGas         *hexutil.Big `json:"maxFeePerGas,omitempty"`
	// GasLimit             *hexutil.Big    `json:"gasLimit,omitempty"`
}

func createAccessListArgs(tx *types.Transaction, header *types.Header) TransactionArgs {
	gas := tx.Gas()
	if gas == 0 {
		gas = header.GasLimit
	}

	// TODO: this is a hack...
	if tx.Type() == types.LegacyTxType && tx.Data() != nil {
		gas *= 2
	}

	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		from = common.Address{}
	}

	args := TransactionArgs{
		From:  &from,
		To:    tx.To(),
		Data:  hexutil.Bytes(tx.Data()),
		Value: (*hexutil.Big)(tx.Value()),
		Gas:   (*hexutil.Big)(big.NewInt(int64(gas))),
	}

	switch tx.Type() {
	case types.DynamicFeeTxType:
		args.MaxFeePerGas = (*hexutil.Big)(tx.GasFeeCap())
		if args.MaxFeePerGas.ToInt().Cmp(header.BaseFee) < 0 {
			args.MaxFeePerGas = (*hexutil.Big)(big.NewInt(int64(header.BaseFee.Uint64())))
		}
		args.MaxPriorityFeePerGas = (*hexutil.Big)(tx.GasTipCap())
	default:

		// args.GasPrice = (*hexutil.Big)(tx.GasPrice())
		// if args.GasPrice.ToInt().Cmp(header.BaseFee) < 0 {
		// 	args.GasPrice = (*hexutil.Big)(big.NewInt(int64(header.BaseFee.Uint64())))
		// }
		// args.GasLimit = (*hexutil.Big)(big.NewInt(int64(header.GasLimit)))
	}

	return args
}

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

func (c *ArbitrumClient) GetStateDBFromProofs(ctx context.Context, msg *arbostypes.L1IncomingMessage, header *types.Header, chainId uint64) (*state.StateDB, []struct {
	AccessList []struct {
		Address     string   `json:"address"`
		StorageKeys []string `json:"storageKeys"`
	} `json:"accessList"`
}, error) {
	// Step 1: Parse L2 transactions
	txs, err := arbos.ParseL2Transactions(msg, big.NewInt(int64(chainId)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse L2 transactions: %w", err)
	}

	if err != nil {
		fmt.Println("error", err)
	}

	accessListResults := make([]struct {
		AccessList []struct {
			Address     string   `json:"address"`
			StorageKeys []string `json:"storageKeys"`
		} `json:"accessList"`
	}, len(txs))
	proofs := make([][]*EthGetProofResult, len(txs))

	// Step 2: Create memory DB and collect proofs
	memdb := rawdb.NewMemoryDatabase()

	for i, tx := range txs {
		// Fetch access list for the tx
		var accessListResp struct {
			AccessList []struct {
				Address     string   `json:"address"`
				StorageKeys []string `json:"storageKeys"`
			} `json:"accessList"`
		}

		txArgs := createAccessListArgs(tx, header)
		err = c.rpcClient.CallContext(ctx, &accessListResp, "eth_createAccessList", txArgs, hexutil.EncodeUint64(header.Number.Uint64()))
		if err != nil {
			return nil, nil, fmt.Errorf("access list creation failed: %w", err)
		}

		// Include sender
		accessListResp.AccessList = append(accessListResp.AccessList, struct {
			Address     string   `json:"address"`
			StorageKeys []string `json:"storageKeys"`
		}{Address: txArgs.From.Hex(), StorageKeys: []string{}})

		// Include "to" address if it exists (for value transfers)
		if txArgs.To != nil {
			accessListResp.AccessList = append(accessListResp.AccessList, struct {
				Address     string   `json:"address"`
				StorageKeys []string `json:"storageKeys"`
			}{Address: txArgs.To.Hex(), StorageKeys: []string{}})
		}

		accessListResults[i] = accessListResp

		// Collect proofs and populate memdb
		for _, entry := range accessListResp.AccessList {
			addr := common.HexToAddress(entry.Address)

			proof, err := c.GetProof(ctx, *header.Number, addr, entry.StorageKeys)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
			}

			proofs[i] = append(proofs[i], proof)

			if !c.VerifyStateProof(header.Root, proof) {
				return nil, nil, fmt.Errorf("invalid proof for %s", addr)
			}

			for _, encodedNode := range proof.AccountProof {
				nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
				if err != nil {
					return nil, nil, fmt.Errorf("decode account node: %w", err)
				}
				hash := crypto.Keccak256Hash(nodeBytes)
				if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
					return nil, nil, fmt.Errorf("put account node: %w", err)
				}
			}
			for _, sp := range proof.StorageProofs {
				for _, encodedNode := range sp.Proof {
					nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
					if err != nil {
						return nil, nil, fmt.Errorf("decode storage node: %w", err)
					}
					hash := crypto.Keccak256Hash(nodeBytes)
					if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
						return nil, nil, fmt.Errorf("put storage node: %w", err)
					}
				}
			}
		}
	}

	// Step 3: Initialize triedb and statedb
	tdb := triedb.NewDatabase(memdb, nil)
	sdb := state.NewDatabase(tdb, nil)
	statedb, err := state.NewDeterministic(header.Root, sdb)
	if err != nil {
		fmt.Println("header root: ", header.Root.Hex())
		return nil, nil, fmt.Errorf("failed to create statedb: %w", err)
	}

	// Step 4: Apply known account and storage values
	for i := range proofs {
		// txArgs := createAccessListArgs(tx, header)
		// err := c.rpcClient.CallContext(ctx, &accessListResp, "eth_createAccessList", txArgs, hexutil.EncodeUint64(header.Number.Uint64()))
		// if err != nil {
		// 	return nil, fmt.Errorf("access list creation failed: %w", err)
		// }
		// accessListResp.AccessList = append(accessListResp.AccessList, struct {
		// 	Address     string   `json:"address"`
		// 	StorageKeys []string `json:"storageKeys"`
		// }{Address: txArgs.From.Hex(), StorageKeys: []string{}})

		for _, proof := range proofs[i] {
			addr := common.HexToAddress(proof.Address)

			// proof, err := c.GetProof(ctx, *header.Number, addr, entry.StorageKeys)
			// if err != nil {
			// 	return nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
			// }

			nonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
			balance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
			account := &types.StateAccount{
				Nonce:    nonce.Uint64(),
				Balance:  uint256.MustFromBig(balance),
				CodeHash: common.HexToHash(proof.CodeHash).Bytes(),
				Root:     common.HexToHash(proof.StorageHash),
			}

			statedb.SetBalance(addr, account.Balance, tracing.BalanceChangeUnspecified)
			statedb.SetNonce(addr, account.Nonce, tracing.NonceChangeUnspecified)

			code, err := c.ethClient.CodeAt(ctx, addr, header.Number)
			if err == nil && len(code) > 0 {
				statedb.SetCode(addr, code)
			}

			for _, sp := range proof.StorageProofs {
				key := common.HexToHash(sp.Key)
				val := common.HexToHash(sp.Value)
				statedb.SetState(addr, key, val)
			}
		}
	}

	if statedb.IntermediateRoot(true).Hex() != header.Root.Hex() {
		panic("statedb root mismatch")
	} else {
		fmt.Println("statedb root FINALLY MATCHES.... header root")
	}

	// statedb.Finalise(false)
	// _, err = statedb.Commit(0, false, false)
	// if err != nil {
	// 	fmt.Println("error committing statedb", err)
	// }

	return statedb, accessListResults, nil
}

func (c *ArbitrumClient) RebuildStateRootFromDBAndProofs(ctx context.Context, msg *arbostypes.L1IncomingMessage, header *types.Header, statedb *state.StateDB, chainId uint64, accessListResults []struct {
	AccessList []struct {
		Address     string   `json:"address"`
		StorageKeys []string `json:"storageKeys"`
	} `json:"accessList"`
}) (*state.StateDB, error) {
	// Step 1: Parse L2 transactions
	txs, err := arbos.ParseL2Transactions(msg, big.NewInt(int64(chainId)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse L2 transactions: %w", err)
	}

	// Step 2: Create memory DB and collect proofs
	memdb := rawdb.NewMemoryDatabase()

	for i, _ := range txs {
		// Collect proofs and populate memdb
		for _, entry := range accessListResults[i].AccessList {
			addr := common.HexToAddress(entry.Address)
			fmt.Println("addr", addr)
			proof, err := c.GetProof(ctx, *header.Number, addr, entry.StorageKeys)
			if err != nil {
				return nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
			}

			if !c.VerifyStateProof(header.Root, proof) {
				return nil, fmt.Errorf("invalid proof for %s", addr)
			}

			for _, encodedNode := range proof.AccountProof {
				nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
				if err != nil {
					return nil, fmt.Errorf("decode account node: %w", err)
				}
				hash := crypto.Keccak256Hash(nodeBytes)
				if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
					return nil, fmt.Errorf("put account node: %w", err)
				}
			}
			for _, sp := range proof.StorageProofs {
				for _, encodedNode := range sp.Proof {
					nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
					if err != nil {
						return nil, fmt.Errorf("decode storage node: %w", err)
					}
					hash := crypto.Keccak256Hash(nodeBytes)
					if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
						return nil, fmt.Errorf("put storage node: %w", err)
					}
				}
			}
		}
	}

	// Step 3: Initialize triedb and statedb
	tdb := triedb.NewDatabase(memdb, nil)
	sdb := state.NewDatabase(tdb, nil)
	statedbNew, err := state.NewDeterministic(header.Root, sdb)
	if err != nil {
		return nil, fmt.Errorf("failed to create statedb: %w", err)
	}

	// Step 4: Apply known account and storage values
	for i, _ := range txs {
		for _, entry := range accessListResults[i].AccessList {
			addr := common.HexToAddress(entry.Address)

			proof, err := c.GetProof(ctx, *header.Number, addr, entry.StorageKeys)
			if err != nil {
				return nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
			}

			nonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
			balance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
			account := &types.StateAccount{
				Nonce:    nonce.Uint64(),
				Balance:  uint256.MustFromBig(balance),
				CodeHash: common.HexToHash(proof.CodeHash).Bytes(),
				Root:     common.HexToHash(proof.StorageHash),
			}

			localNonce := statedb.GetNonce(addr)
			localBalance := statedb.GetBalance(addr)
			localCodeHash := statedb.GetCodeHash(addr)
			localRoot := statedb.GetStorageRoot(addr)

			if localNonce != account.Nonce {
				fmt.Println("nonce mismatch for account", addr, localNonce, account.Nonce)
			}

			if localBalance.Cmp(account.Balance) != 0 {
				fmt.Println("balance mismatch for account", addr, localBalance, account.Balance)
			}

			if localCodeHash.Cmp(common.BytesToHash(account.CodeHash)) != 0 {
				fmt.Println("codeHash mismatch for account", addr, localCodeHash, account.CodeHash)
			}

			if localRoot.Cmp(account.Root) != 0 {
				fmt.Println("root mismatch for account", addr, localRoot, account.Root)
			}

			statedbNew.SetBalance(addr, account.Balance, tracing.BalanceChangeUnspecified)
			statedbNew.SetNonce(addr, account.Nonce, tracing.NonceChangeUnspecified)

			code, err := c.ethClient.CodeAt(ctx, addr, header.Number)
			if err == nil && len(code) > 0 {
				statedbNew.SetCode(addr, code)
			}

			for _, sp := range proof.StorageProofs {
				localVal := statedb.GetState(addr, common.HexToHash(sp.Key))
				key := common.HexToHash(sp.Key)
				val := common.HexToHash(sp.Value)
				if localVal != val {
					fmt.Println("key", sp.Key)
					fmt.Println("sp.value", sp.Value)
					fmt.Println("storage mismatch", localVal, val)
				}
				statedbNew.SetState(addr, key, val)
			}
		}
	}

	if statedbNew.IntermediateRoot(false).Hex() != header.Root.Hex() {
		panic("statedb root mismatch")
	}

	return statedbNew, nil
}

func (c *ArbitrumClient) GetLatestState(ctx context.Context, chainId uint64) (*MessageTrackingL2Data, error) {
	var index L1Index
	var blockNumber uint64
	c.rpcClient.CallContext(ctx, &index, "lightclient_getLatestIndexL1")

	blockNumber, err := c.ethClient.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	c.rpcClient.CallContext(ctx, &blockNumber, "eth_blockNumber")

	if chainId == 42161 {
		blockNumber -= ARBITRUM_ONE_GENESIS_BLOCK
	}

	realIndex := min(index.StateIndex-1, blockNumber)

	block, err := c.GetBlockByNumber(ctx, big.NewInt(int64(realIndex)))
	if err != nil {
		return nil, err
	}

	if chainId == 42161 {
		realIndex += ARBITRUM_ONE_GENESIS_BLOCK
	}

	return &MessageTrackingL2Data{
		L2BlockNumber: realIndex,
		L2BlockHash:   block.Hash(),
	}, nil
}

func (c *ArbitrumClient) GetStateAt(ctx context.Context, blockNumber uint64, chainId uint64) MessageTrackingL2Data {
	var data MessageTrackingL2Data
	if chainId == 42161 {
		blockNumber -= ARBITRUM_ONE_GENESIS_BLOCK
	}
	c.rpcClient.CallContext(ctx, &data, "lightclient_getStateAt", blockNumber)

	return data
}

func (c *ArbitrumClient) GetFullDataAt(ctx context.Context, blockNumber uint64, chainId uint64) StateAtReturnData {
	var data StateAtReturnData
	if chainId == 42161 {
		blockNumber -= ARBITRUM_ONE_GENESIS_BLOCK
	}
	c.rpcClient.CallContext(ctx, &data, "lightclient_getFullDataAt", blockNumber)

	return data
}

func (c *ArbitrumClient) GetL1DataAt(ctx context.Context, blockNumber uint64, chainId uint64) MessageTrackingL1Data {
	var data MessageTrackingL1Data
	c.rpcClient.CallContext(ctx, &data, "lightclient_getL1DataAt", blockNumber)

	return data
}

// GetModifiedAccounts gets all accounts that were modified in a specific block
func (c *ArbitrumClient) GetModifiedAccounts(ctx context.Context, blockNumber uint64) ([]common.Address, error) {
	var addresses []string
	err := c.rpcClient.CallContext(ctx, &addresses, "debug_getModifiedAccountsByNumber", blockNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get modified accounts: %w", err)
	}

	// Convert string addresses to common.Address
	result := make([]common.Address, len(addresses))
	for i, addr := range addresses {
		result[i] = common.HexToAddress(addr)
	}

	return result, nil
}

func (c *ArbitrumClient) GetStateDBFromComprehensiveAccessList(ctx context.Context, msg *arbostypes.L1IncomingMessage, header *types.Header, chainId uint64) (*state.StateDB, error) {
	// Step 1: Parse L2 transactions
	txs, err := arbos.ParseL2Transactions(msg, big.NewInt(int64(chainId)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse L2 transactions: %w", err)
	}

	// Step 2: Create comprehensive access list including all affected accounts
	allAccounts := make(map[common.Address]bool)
	memdb := rawdb.NewMemoryDatabase()

	for i, tx := range txs {
		fmt.Printf("Processing transaction %d: %s\n", i, tx.Hash().Hex())

		// Get sender address
		from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			from = common.Address{}
		}
		allAccounts[from] = true

		// Get "to" address if it exists (for value transfers)
		if tx.To() != nil {
			allAccounts[*tx.To()] = true
		}

		// Get access list from RPC
		txArgs := createAccessListArgs(tx, header)
		var accessListResp struct {
			AccessList []struct {
				Address     string   `json:"address"`
				StorageKeys []string `json:"storageKeys"`
			} `json:"accessList"`
		}

		err = c.rpcClient.CallContext(ctx, &accessListResp, "eth_createAccessList", txArgs, hexutil.EncodeUint64(header.Number.Uint64()))
		if err != nil {
			fmt.Printf("Warning: access list creation failed for tx %d: %v\n", i, err)
			continue
		}

		// Add all addresses from access list
		for _, entry := range accessListResp.AccessList {
			addr := common.HexToAddress(entry.Address)
			allAccounts[addr] = true
		}
	}

	var accounts []common.Address
	for addr := range allAccounts {
		accounts = append(accounts, addr)
	}

	for _, addr := range accounts {
		exists, err := c.AccountExists(ctx, addr, header.Number)
		if err != nil {
			fmt.Printf("Warning: failed to check if account %s exists: %v\n", addr.Hex(), err)
			continue
		}

		if !exists {
			fmt.Printf("Account %s does not exist, skipping proof\n", addr.Hex())
			continue
		}

		proof, err := c.GetProof(ctx, *header.Number, addr, []string{})
		if err != nil {
			return nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
		}

		if !c.VerifyStateProof(header.Root, proof) {
			return nil, fmt.Errorf("invalid proof for %s", addr)
		}

		for _, encodedNode := range proof.AccountProof {
			nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
			if err != nil {
				return nil, fmt.Errorf("decode account node: %w", err)
			}
			hash := crypto.Keccak256Hash(nodeBytes)
			if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
				return nil, fmt.Errorf("put account node: %w", err)
			}
		}

		for _, sp := range proof.StorageProofs {
			for _, encodedNode := range sp.Proof {
				nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
				if err != nil {
					return nil, fmt.Errorf("decode storage node: %w", err)
				}
				hash := crypto.Keccak256Hash(nodeBytes)
				if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
					return nil, fmt.Errorf("put storage node: %w", err)
				}
			}
		}
	}

	tdb := triedb.NewDatabase(memdb, nil)
	sdb := state.NewDatabase(tdb, nil)
	statedb, err := state.NewDeterministic(header.Root, sdb)
	if err != nil {
		return nil, fmt.Errorf("failed to create statedb: %w", err)
	}

	for _, addr := range accounts {
		proof, err := c.GetProof(ctx, *header.Number, addr, []string{})
		if err != nil {
			return nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
		}

		nonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
		balance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)

		statedb.SetBalance(addr, uint256.MustFromBig(balance), tracing.BalanceChangeUnspecified)
		statedb.SetNonce(addr, nonce.Uint64(), tracing.NonceChangeUnspecified)

		code, err := c.ethClient.CodeAt(ctx, addr, header.Number)
		if err == nil && len(code) > 0 {
			statedb.SetCode(addr, code)
		}

		for _, sp := range proof.StorageProofs {
			key := common.HexToHash(sp.Key)
			val := common.HexToHash(sp.Value)
			statedb.SetState(addr, key, val)
		}
	}

	if statedb.IntermediateRoot(true).Hex() != header.Root.Hex() {
		return nil, fmt.Errorf("state root mismatch: got %s, expected %s",
			statedb.IntermediateRoot(true).Hex(), header.Root.Hex())
	}

	return statedb, nil
}

// AccountExists checks if an account exists in the state
func (c *ArbitrumClient) AccountExists(ctx context.Context, address common.Address, blockNumber *big.Int) (bool, error) {
	// Try to get the account's balance - if it's 0 and nonce is 0, it might not exist
	balance, err := c.ethClient.BalanceAt(ctx, address, blockNumber)
	if err != nil {
		return false, err
	}

	// If balance is 0, check nonce and code
	if balance.Cmp(big.NewInt(0)) == 0 {
		nonce, err := c.ethClient.NonceAt(ctx, address, blockNumber)
		if err != nil {
			return false, err
		}

		code, err := c.ethClient.CodeAt(ctx, address, blockNumber)
		if err != nil {
			return false, err
		}

		// Account exists if it has non-zero nonce or code
		return nonce > 0 || len(code) > 0, nil
	}

	// If balance is non-zero, account definitely exists
	return true, nil
}

// GetLazyStateDB creates a lazy-loading StateDB that automatically fetches proofs when state is accessed
func (c *ArbitrumClient) GetLazyStateDB(ctx context.Context, header *types.Header) *LazyStateDB {
	return NewLazyStateDB(ctx, c, header.Number, header.Root)
}
