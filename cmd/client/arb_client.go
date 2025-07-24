package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
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
	"github.com/offchainlabs/nitro/arbos/arbostypes"
)

type ArbitrumClient struct {
	ethClient *ethclient.Client
	rpcClient *rpc.Client
}

type MessageTrackingL2Data struct {
	L2BlockNumber uint64
	L2BlockHash   common.Hash
}

type MessageTrackingL1Data struct {
	Message      arbostypes.L1IncomingMessage
	L1TxHash     common.Hash
	DataLocation uint8
}

type L1Index struct {
	StateIndex uint64
}

const ARBITRUM_ONE_GENESIS_BLOCK = 22207817

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

func (c *ArbitrumClient) VerifyStateProof(stateRoot common.Hash, proof *EthGetProofResult) bool {
	addr := common.HexToAddress(proof.Address)
	key := crypto.Keccak256(addr.Bytes())

	for _, sp := range proof.StorageProofs {
		slotKeyBytes, err := hex.DecodeString(trimHexPrefix(sp.Key))
		if err != nil {
			fmt.Println("❌ Invalid storage key:", err)
			return false
		}
		slotKeyHash := crypto.Keccak256(slotKeyBytes)

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

		expectedValue := new(big.Int)
		expectedValue.SetString(trimHexPrefix(sp.Value), 16)

		val, err := trie.VerifyProof(common.HexToHash(proof.StorageHash), slotKeyHash, storageDB)
		if err != nil {
			fmt.Println("❌ Storage proof verification failed:", err)
			return false
		}

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
	}

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

	val, err := trie.VerifyProof(stateRoot, key, proofDB)
	if err != nil {
		fmt.Println("❌ Proof verification failed:", err)
		return false
	}

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

	jsonNonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
	jsonBalance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
	jsonCodeHash := common.HexToHash(proof.CodeHash)

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

func (c *ArbitrumClient) GetLatestState(ctx context.Context, chainId uint64) (*MessageTrackingL2Data, error) {
	var index L1Index
	var blockNumber uint64
	c.rpcClient.CallContext(ctx, &index, "lightclient_getLatestIndexL1")

	blockNumber, err := c.ethClient.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	c.rpcClient.CallContext(ctx, &blockNumber, "eth_blockNumber")

	realIndex := min(index.StateIndex-1, blockNumber)

	return c.GetStateAt(ctx, realIndex, chainId)
}

func (c *ArbitrumClient) GetStateAt(ctx context.Context, blockNumber uint64, chainId uint64) (*MessageTrackingL2Data, error) {
	var data MessageTrackingL2Data

	block, err := c.GetBlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}

	data = MessageTrackingL2Data{
		L2BlockNumber: blockNumber,
		L2BlockHash:   block.Hash(),
	}

	return &data, nil
}

func (c *ArbitrumClient) GetL1DataAt(ctx context.Context, blockNumber uint64, chainId uint64) MessageTrackingL1Data {
	var data MessageTrackingL1Data

	c.rpcClient.CallContext(ctx, &data, "lightclient_getL1DataAt", blockNumber)
	return data
}

func (c *ArbitrumClient) ReconstructStateFromProofsAndTrace(ctx context.Context, currentHeader *types.Header, previousHeader *types.Header, chainId uint64) (*state.StateDB, map[common.Address]struct{}, map[common.Address]map[common.Hash]struct{}, error) {
	var traceResult []struct {
		TxHash string `json:"txHash"`
		Result map[string]struct {
			Balance string            `json:"balance"`
			Code    string            `json:"code"`
			Nonce   uint64            `json:"nonce"`
			Storage map[string]string `json:"storage"`
		} `json:"result"`
	}

	traceConfig := map[string]interface{}{
		"tracer": "prestateTracer",
		"tracerConfig": map[string]interface{}{
			"diffMode": false,
		},
	}

	err := c.rpcClient.CallContext(ctx, &traceResult, "debug_traceBlockByHash", currentHeader.Hash().Hex(), traceConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("debug_traceBlockByHash failed: %w", err)
	}

	if len(traceResult) == 0 {
		return nil, nil, nil, fmt.Errorf("trace result is empty")
	}

	allAccounts := make(map[common.Address]bool)
	allStorageKeys := make(map[common.Address]map[common.Hash]bool)

	for _, txTrace := range traceResult {
		for addrStr, accountData := range txTrace.Result {
			if !common.IsHexAddress(addrStr) {
				return nil, nil, nil, fmt.Errorf("invalid address in trace result: %s", addrStr)
			}

			addr := common.HexToAddress(addrStr)
			allAccounts[addr] = true

			if allStorageKeys[addr] == nil {
				allStorageKeys[addr] = make(map[common.Hash]bool)
			}

			for keyStr := range accountData.Storage {
				if len(keyStr) != 66 || !strings.HasPrefix(keyStr, "0x") {
					return nil, nil, nil, fmt.Errorf("invalid storage key in trace result: %s", keyStr)
				}

				key := common.HexToHash(keyStr)
				allStorageKeys[addr][key] = true
			}
		}
	}

	knownAccounts := []common.Address{
		common.HexToAddress("0x00000000000000000000000000000000000A4B05"), // ArbOS
		common.HexToAddress("0x5E1497dD1f08C87b2d8FE23e9AAB6c1De833D927"), // local
		common.HexToAddress("0xE6841D92B0C345144506576eC13ECf5103aC7f49"), // arb1 and nova
		common.HexToAddress("0x6EC62D826aDc24AeA360be9cF2647c42b9Cdb19b"), // sepolia
		common.HexToAddress("0xa4B00000000000000000000000000000000000F6"),
		common.HexToAddress("0x64"), // ArbSysAddress
		common.HexToAddress("0x65"), // ArbInfoAddress
		common.HexToAddress("0x66"), // ArbAddressTableAddress
		common.HexToAddress("0x67"), // ArbBLSAddress
		common.HexToAddress("0x68"), // ArbFunctionTableAddress
		common.HexToAddress("0x69"), // ArbosTestAddress
		common.HexToAddress("0x6c"), // ArbGasInfoAddress
		common.HexToAddress("0x6b"), // ArbOwnerPublicAddress
		common.HexToAddress("0x6d"), // ArbAggregatorAddress
		common.HexToAddress("0x6e"), // ArbRetryableTxAddress
		common.HexToAddress("0x6f"), // ArbStatisticsAddress
		common.HexToAddress("0x70"), // ArbOwnerAddress
		common.HexToAddress("0x71"), // ArbWasmAddress
		common.HexToAddress("0x72"), // ArbWasmCacheAddress
		common.HexToAddress("0xc8"), // NodeInterfaceAddress
		common.HexToAddress("0xc9"), // NodeInterfaceDebugAddress
		common.HexToAddress("0xff"), // ArbDebugAddress
	}

	for _, addr := range knownAccounts {
		allAccounts[addr] = true
	}

	memdb := rawdb.NewMemoryDatabase()
	proofs := make(map[common.Address]*EthGetProofResult)

	for addr := range allAccounts {
		var storageKeys []string
		if storageMap, exists := allStorageKeys[addr]; exists {
			for key := range storageMap {
				storageKeys = append(storageKeys, key.Hex())
			}
		}

		if addr == common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff") {
			arbosStorageKeys := c.getAllPossibleArbOSStorageKeys()
			storageKeys = append(storageKeys, arbosStorageKeys...)
			for _, key := range arbosStorageKeys {
				allStorageKeys[addr][common.HexToHash(key)] = true
			}
		}

		proof, err := c.GetProof(ctx, *previousHeader.Number, addr, storageKeys)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
		}

		if proof.Balance == "0x0" && proof.Nonce == "0x0" && proof.StorageHash == "0x0000000000000000000000000000000000000000000000000000000000000000" && proof.CodeHash == "0x0000000000000000000000000000000000000000000000000000000000000000" {
			continue
		}

		// if addr != common.HexToAddress("0x00000000000000000000000000000000000a4b05") {
		// 	if !c.VerifyStateProof(previousHeader.Root, proof) {
		// 		return nil, nil, nil, fmt.Errorf("state proof mismatch for account %s", addr.Hex())
		// 	}
		// }

		proofs[addr] = proof

		for _, encodedNode := range proof.AccountProof {
			nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
			if err != nil {
				return nil, nil, nil, fmt.Errorf("decode account node: %w", err)
			}
			hash := crypto.Keccak256Hash(nodeBytes)
			if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
				return nil, nil, nil, fmt.Errorf("put account node: %w", err)
			}
		}

		for _, sp := range proof.StorageProofs {
			for _, encodedNode := range sp.Proof {
				nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
				if err != nil {
					return nil, nil, nil, fmt.Errorf("decode storage node: %w", err)
				}
				hash := crypto.Keccak256Hash(nodeBytes)
				if err := memdb.Put(hash.Bytes(), nodeBytes); err != nil {
					return nil, nil, nil, fmt.Errorf("put storage node: %w", err)
				}
			}
		}
	}

	tdb := triedb.NewDatabase(memdb, nil)
	sdb := state.NewDatabase(tdb, nil)
	statedb, err := state.NewDeterministic(previousHeader.Root, sdb)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create statedb: %w", err)
	}

	for addr, proof := range proofs {
		nonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
		balance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)

		statedb.SetBalance(addr, uint256.MustFromBig(balance), tracing.BalanceChangeUnspecified)
		statedb.SetNonce(addr, nonce.Uint64(), tracing.NonceChangeUnspecified)

		code, err := c.ethClient.CodeAt(ctx, addr, previousHeader.Number)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get code for account %s: %w", addr, err)
		}

		if len(code) > 0 {
			statedb.SetCode(addr, code)
		}

		for _, sp := range proof.StorageProofs {
			key := common.HexToHash(sp.Key)
			val := common.HexToHash(sp.Value)
			statedb.SetState(addr, key, val)
		}
	}

	computedRootFromProofs := statedb.IntermediateRoot(false)

	if computedRootFromProofs.Hex() != previousHeader.Root.Hex() {
		return nil, nil, nil, fmt.Errorf("state root from proofs mismatch: got %s, expected %s", computedRootFromProofs.Hex(), previousHeader.Root.Hex())
	}

	accountSet := make(map[common.Address]struct{})
	slotSet := make(map[common.Address]map[common.Hash]struct{})

	for addr := range allAccounts {
		accountSet[addr] = struct{}{}
		if keys, exists := allStorageKeys[addr]; exists {
			slotSet[addr] = make(map[common.Hash]struct{})
			for key := range keys {
				slotSet[addr][key] = struct{}{}
			}
		}
	}

	return statedb, accountSet, slotSet, nil
}

func (c *ArbitrumClient) VerifyStateAgainstProofs(ctx context.Context, statedb *state.StateDB, accountSet map[common.Address]struct{}, slotSet map[common.Address]map[common.Hash]struct{}, expectedHeader *types.Header) error {
	fmt.Printf("🔍 Verifying %d accounts and their storage slots against proofs...\n", len(accountSet))

	statedb2, _, _, err := c.ReconstructStateFromProofsAndTrace(ctx, expectedHeader, expectedHeader, 412346)
	if err != nil {
		return fmt.Errorf("failed to reconstruct state from proofs and trace: %w", err)
	}

	// statedb2.GetTrie().DeleteAccount(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))
	// statedb.GetTrie().DeleteAccount(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))

	root1 := statedb.IntermediateRoot(false)
	root2 := statedb2.IntermediateRoot(false)
	fmt.Printf("📊 Overall State Roots:\n")
	fmt.Printf("  - StateDB1: %s\n", root1.Hex())
	fmt.Printf("  - StateDB2: %s\n", root2.Hex())
	fmt.Printf("  - Match: %t\n", root1 == root2)

	return nil
}

func (c *ArbitrumClient) FindStateDifferences(ctx context.Context, statedb *state.StateDB, accountSet map[common.Address]struct{}, expectedHeader *types.Header) error {
	fmt.Printf("🔍 Finding all state differences against expected block %d...\n", expectedHeader.Number.Uint64())

	localAccounts := make(map[common.Address]bool)
	for addr := range accountSet {
		localAccounts[addr] = true
	}

	fmt.Printf("📊 Local state has %d accounts\n", len(localAccounts))
	for addr := range localAccounts {
		proof, err := c.GetProof(ctx, *expectedHeader.Number, addr, []string{})
		if err != nil {
			fmt.Printf("⚠️  Could not get proof for account %s: %v\n", addr.Hex(), err)
			continue
		}

		proofNonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
		proofBalance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
		proofCodeHash := common.HexToHash(proof.CodeHash)
		proofStorageHash := common.HexToHash(proof.StorageHash)

		localNonce := statedb.GetNonce(addr)
		localBalance := statedb.GetBalance(addr)
		localCodeHash := statedb.GetCodeHash(addr)
		localStorageRoot := statedb.GetStorageRoot(addr)

		accountExistsInProof := proofNonce.Uint64() > 0 || proofBalance.Cmp(big.NewInt(0)) > 0 || proofCodeHash != common.Hash{}
		accountExistsLocally := localNonce > 0 || localBalance.Cmp(uint256.MustFromBig(big.NewInt(0))) > 0 || localCodeHash != common.Hash{}

		if accountExistsInProof != accountExistsLocally {
			fmt.Printf("❌ Account existence mismatch for %s: local exists=%t, proof exists=%t\n",
				addr.Hex(), accountExistsLocally, accountExistsInProof)
			continue
		}

		if !accountExistsInProof {
			continue
		}

		if localNonce != proofNonce.Uint64() {
			fmt.Printf("❌ Nonce mismatch for account %s: local %d, proof %d\n",
				addr.Hex(), localNonce, proofNonce.Uint64())
		}

		if localBalance.Cmp(uint256.MustFromBig(proofBalance)) != 0 {
			fmt.Printf("❌ Balance mismatch for account %s: local %s, proof %s\n",
				addr.Hex(), localBalance.String(), proofBalance.String())
		}

		if localCodeHash != proofCodeHash {
			fmt.Printf("❌ Code hash mismatch for account %s: local %s, proof %s\n",
				addr.Hex(), localCodeHash.Hex(), proofCodeHash.Hex())
		}

		if localStorageRoot != proofStorageHash {
			fmt.Printf("❌ Storage root mismatch for account %s: local %s, proof %s\n",
				addr.Hex(), localStorageRoot.Hex(), proofStorageHash.Hex())
		}
	}

	knownAccounts := []common.Address{
		common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"),
		common.HexToAddress("0x00000000000000000000000000000000000A4B05"),
		common.HexToAddress("0xA4b000000000000000000073657175656e636572"),
	}

	for _, addr := range knownAccounts {
		if !localAccounts[addr] {
			proof, err := c.GetProof(ctx, *expectedHeader.Number, addr, []string{})
			if err == nil {
				proofNonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
				proofBalance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
				proofCodeHash := common.HexToHash(proof.CodeHash)

				if proofNonce.Uint64() > 0 || proofBalance.Cmp(big.NewInt(0)) > 0 || proofCodeHash != (common.Hash{}) {
					fmt.Printf("❌ Missing account in local state: %s (exists in expected block)\n", addr.Hex())
				}
			}
		}
	}

	localRoot := statedb.IntermediateRoot(false)
	if localRoot != expectedHeader.Root {
		fmt.Printf("❌ State root mismatch: local %s, expected %s\n",
			localRoot.Hex(), expectedHeader.Root.Hex())
	} else {
		fmt.Printf("✅ State root matches!\n")
	}

	return nil
}

func (c *ArbitrumClient) getAllPossibleArbOSStorageKeys() []string {
	computeStorageKey := func(parentKey []byte, id []byte) []byte {
		return crypto.Keccak256(parentKey, id)
	}

	mapAddress := func(storageKey []byte, key common.Hash) common.Hash {
		keyBytes := key.Bytes()
		boundary := common.HashLength - 1
		mapped := make([]byte, 0, common.HashLength)
		mapped = append(mapped, crypto.Keccak256(storageKey, keyBytes[:boundary])[:boundary]...)
		mapped = append(mapped, keyBytes[boundary])
		return common.BytesToHash(mapped)
	}

	var storageKeys []string

	// Core ArbOS storage keys (offsets 0-7) - these are the main storage slots
	for offset := uint64(0); offset <= 7; offset++ {
		key := common.BigToHash(big.NewInt(int64(offset)))
		storageKeys = append(storageKeys, key.Hex())
	}

	// L1 pricing subspace storage keys (subspace 0) - expanded range
	l1PricingSubspaceKey := computeStorageKey([]byte{}, []byte{0})
	for offset := uint64(0); offset <= 3800; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(l1PricingSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// L2 pricing subspace storage keys (subspace 1) - expanded range
	l2PricingSubspaceKey := computeStorageKey([]byte{}, []byte{1})
	for offset := uint64(0); offset <= 10; offset++ { // Increased range
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(l2PricingSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Retryables subspace storage keys (subspace 2) - expanded
	retryablesSubspaceKey := computeStorageKey([]byte{}, []byte{2})
	for offset := uint64(0); offset <= 1000; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(retryablesSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Address table subspace storage keys (subspace 3) - expanded
	addressTableSubspaceKey := computeStorageKey([]byte{}, []byte{3})
	for offset := uint64(0); offset <= 5; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(addressTableSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Chain owners subspace storage keys (subspace 4) - expanded
	chainOwnerSubspaceKey := computeStorageKey([]byte{}, []byte{4})
	for offset := uint64(0); offset <= 2; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(chainOwnerSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Send merkle subspace storage keys (subspace 5) - expanded
	sendMerkleSubspaceKey := computeStorageKey([]byte{}, []byte{5})
	for offset := uint64(0); offset <= 50; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(sendMerkleSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Blockhashes subspace storage keys (subspace 6) - expanded
	blockhashesSubspaceKey := computeStorageKey([]byte{}, []byte{6})
	for offset := uint64(0); offset <= 10; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(blockhashesSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	for offset := uint64(10); offset <= 300; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(blockhashesSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Chain config subspace storage keys (subspace 7) - expanded
	chainConfigSubspaceKey := computeStorageKey([]byte{}, []byte{7})
	for offset := uint64(0); offset <= 20; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(chainConfigSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Programs subspace storage keys (subspace 8) - expanded
	programsSubspaceKey := computeStorageKey([]byte{}, []byte{8})
	for offset := uint64(0); offset <= 10; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(programsSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Features subspace storage keys (subspace 9) - expanded
	featuresSubspaceKey := computeStorageKey([]byte{}, []byte{9})
	for offset := uint64(0); offset <= 50; offset++ { // More slots
		key := common.BigToHash(big.NewInt(int64(offset)))
		mappedKey := mapAddress(featuresSubspaceKey, key)
		storageKeys = append(storageKeys, mappedKey.Hex())
	}

	// Add some additional subspaces that might exist
	for subspaceID := uint8(10); subspaceID <= 20; subspaceID++ {
		subspaceKey := computeStorageKey([]byte{}, []byte{subspaceID})
		for offset := uint64(0); offset <= 3; offset++ {
			key := common.BigToHash(big.NewInt(int64(offset)))
			mappedKey := mapAddress(subspaceKey, key)
			storageKeys = append(storageKeys, mappedKey.Hex())
		}
	}

	return storageKeys
}
