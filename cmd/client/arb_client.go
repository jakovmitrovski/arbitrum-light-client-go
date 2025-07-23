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

func (c *ArbitrumClient) VerifyStateProof(stateRoot common.Hash, proof *EthGetProofResult) bool {
	addr := common.HexToAddress(proof.Address)
	key := crypto.Keccak256(addr.Bytes()) // MPT key for the address

	for _, sp := range proof.StorageProofs {
		// fmt.Println("🔍 Verifying storage slot:", sp.Key)

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

		// fmt.Printf("✅ Verified slot %x = %s\n", slotKeyHash, decodedValue)
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

	fmt.Println("blockNumber", blockNumber)
	c.rpcClient.CallContext(ctx, &data, "lightclient_getL1DataAt", blockNumber)
	return data
}

// ReconstructStateFromProofsAndTrace reconstructs state using proofs and then updates with trace values
func (c *ArbitrumClient) ReconstructStateFromProofsAndTrace(ctx context.Context, currentHeader *types.Header, previousHeader *types.Header, chainId uint64) (*state.StateDB, map[common.Address]struct{}, map[common.Address]map[common.Hash]struct{}, error) {
	// Step 1: Get trace from current block (shows state before execution)
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

	// Step 2: Extract accounts from trace
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
		// extra
		// common.HexToAddress("0x000000000005BaC754a50d9f3867F49C00c3B07b"),
		// common.HexToAddress("0x000000000001467a230D332f218187FFAfb8Ec0f"),
		// common.HexToAddress("0x0000F90827F1C53a10cb7A02335B175320002935"),
		// common.HexToAddress("0xFF162c694eAA571f685030649814282eA457f169"),
	}

	for _, addr := range knownAccounts {
		allAccounts[addr] = true
	}

	memdb := rawdb.NewMemoryDatabase()
	proofs := make(map[common.Address]*EthGetProofResult)

	for addr := range allAccounts {

		// fmt.Printf("  📝 Processing: %s\n", addr.Hex())

		// Convert storage keys to string slice
		var storageKeys []string
		if storageMap, exists := allStorageKeys[addr]; exists {
			for key := range storageMap {
				storageKeys = append(storageKeys, key.Hex())
			}
		}

		// Special handling for ArbOS address - add all necessary storage slots
		if addr == common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff") {
			// Add all essential ArbOS storage keys
			arbosStorageKeys := c.getAllPossibleArbOSStorageKeys()
			storageKeys = append(storageKeys, arbosStorageKeys...)
			for _, key := range arbosStorageKeys {
				allStorageKeys[addr][common.HexToHash(key)] = true
			}
		}

		// Get proof for this account and its storage
		proof, err := c.GetProof(ctx, *previousHeader.Number, addr, storageKeys)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get proof for %s: %w", addr, err)
		}

		// Skip empty accounts - they don't exist in the state trie
		if proof.Balance == "0x0" && proof.Nonce == "0x0" && proof.StorageHash == "0x0000000000000000000000000000000000000000000000000000000000000000" && proof.CodeHash == "0x0000000000000000000000000000000000000000000000000000000000000000" {
			// fmt.Printf("  ⏭️  Skipping empty account: %s\n", addr.Hex())
			continue
		}

		// This is hacky
		// if addr != common.HexToAddress("0x00000000000000000000000000000000000a4b05") {
		// 	if !c.VerifyStateProof(previousHeader.Root, proof) {
		// 		return nil, nil, nil, fmt.Errorf("state proof mismatch for account %s", addr.Hex())
		// 	}
		// }

		proofs[addr] = proof

		// Store proof nodes in memory database
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

	// Initialize StateDB from proofs
	tdb := triedb.NewDatabase(memdb, nil)
	sdb := state.NewDatabase(tdb, nil)
	statedb, err := state.NewDeterministic(previousHeader.Root, sdb)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create statedb: %w", err)
	}

	// Apply account and storage values from proofs
	for addr, proof := range proofs {
		nonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
		balance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)

		statedb.SetBalance(addr, uint256.MustFromBig(balance), tracing.BalanceChangeUnspecified)
		statedb.SetNonce(addr, nonce.Uint64(), tracing.NonceChangeUnspecified)

		// Get and set code
		code, err := c.ethClient.CodeAt(ctx, addr, previousHeader.Number)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get code for account %s: %w", addr, err)
		}

		if len(code) > 0 {
			statedb.SetCode(addr, code)
		}

		// Apply storage values from proof
		for _, sp := range proof.StorageProofs {
			key := common.HexToHash(sp.Key)
			val := common.HexToHash(sp.Value)
			statedb.SetState(addr, key, val)
		}
	}

	// Verify the state root from proofs matches the previous block's root
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

// VerifyStateAgainstProofs compares the statedb values for all accounts and storage slots against eth_getProof for the expected block header
func (c *ArbitrumClient) VerifyStateAgainstProofs(ctx context.Context, statedb *state.StateDB, accountSet map[common.Address]struct{}, slotSet map[common.Address]map[common.Hash]struct{}, expectedHeader *types.Header) error {
	fmt.Printf("🔍 Verifying %d accounts and their storage slots against proofs...\n", len(accountSet))

	// first we rebuild the state for expected header,
	// then we remove arbos account from both,
	// then we compare the roots.
	statedb2, _, _, err := c.ReconstructStateFromProofsAndTrace(ctx, expectedHeader, expectedHeader, 412346)
	if err != nil {
		return fmt.Errorf("failed to reconstruct state from proofs and trace: %w", err)
	}

	// statedb.CreateAccount(common.HexToAddress("0x00000000000000000000000000000000000A4B05"))

	// c.InspectRawStorage(statedb, common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))

	// fmt.Printf("🔍 Code: %s\n", common.Bytes2Hex(statedb.GetCode(common.HexToAddress("0x217788c286797d56cd59af5e493f3699c39cbbe8"))))
	// fmt.Printf("🔍 Code2: %s\n", common.Bytes2Hex(statedb2.GetCode(common.HexToAddress("0x217788c286797d56cd59af5e493f3699c39cbbe8"))))
	// fmt.Printf("nonce: %d\n", statedb.GetNonce(common.HexToAddress("0x5e1497dd1f08c87b2d8fe23e9aab6c1de833d927")))

	// statedb2.GetTrie().DeleteAccount(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))
	// statedb.GetTrie().DeleteAccount(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))

	// statedb.CreateAccount(common.HexToAddress("0x00000000000000000000000000000000000A4B05"))

	// statedb.SetState(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"), common.HexToHash("0x025266682f3ac65a3b6bc07305bb1e428cbeaadd15c7f73f96f1ca4a39565c3f"), common.HexToHash("0xa1a146487afd08fb987bdb501c6ed5aa5d1e9683e49b32cdcd6e698bd6fcfa71"))
	// statedb.SetState(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"), common.HexToHash("03a678782b54156f2309f89e817a2e1943175a99f9869827dcf7ef08d209f623"), common.HexToHash("0x8fdc9957817f2ffda7cc4ee10dcd68e49f1c9908a45cc92204e4065e590c4fa0"))

	root1 := statedb.IntermediateRoot(false)
	root2 := statedb2.IntermediateRoot(false)
	fmt.Printf("📊 Overall State Roots:\n")
	fmt.Printf("  - StateDB1: %s\n", root1.Hex())
	fmt.Printf("  - StateDB2: %s\n", root2.Hex())
	fmt.Printf("  - Match: %t\n", root1 == root2)

	return nil
}

// FindStateDifferences comprehensively compares the local state against the expected block to find all differences
func (c *ArbitrumClient) FindStateDifferences(ctx context.Context, statedb *state.StateDB, accountSet map[common.Address]struct{}, expectedHeader *types.Header) error {
	fmt.Printf("🔍 Finding all state differences against expected block %d...\n", expectedHeader.Number.Uint64())

	// Get all accounts from the local state
	localAccounts := make(map[common.Address]bool)
	// Since we can't iterate all accounts easily, we'll check the accounts we know about
	// and the ones from our trace
	for addr := range accountSet {
		localAccounts[addr] = true
	}

	fmt.Printf("📊 Local state has %d accounts\n", len(localAccounts))
	// statedb.CreateAccount(common.HexToAddress("0x00000000000000000000000000000000000A4B05"))

	// Check each local account against the expected block
	for addr := range localAccounts {
		// Get proof for this account from expected block
		proof, err := c.GetProof(ctx, *expectedHeader.Number, addr, []string{})
		if err != nil {
			fmt.Printf("⚠️  Could not get proof for account %s: %v\n", addr.Hex(), err)
			continue
		}

		// Check if account exists in expected block
		proofNonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
		proofBalance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
		proofCodeHash := common.HexToHash(proof.CodeHash)
		proofStorageHash := common.HexToHash(proof.StorageHash)

		localNonce := statedb.GetNonce(addr)
		localBalance := statedb.GetBalance(addr)
		localCodeHash := statedb.GetCodeHash(addr)
		localStorageRoot := statedb.GetStorageRoot(addr)

		// Check if account exists in expected block (non-zero nonce, balance, or code)
		accountExistsInProof := proofNonce.Uint64() > 0 || proofBalance.Cmp(big.NewInt(0)) > 0 || proofCodeHash != common.Hash{}
		accountExistsLocally := localNonce > 0 || localBalance.Cmp(uint256.MustFromBig(big.NewInt(0))) > 0 || localCodeHash != common.Hash{}

		if accountExistsInProof != accountExistsLocally {
			fmt.Printf("❌ Account existence mismatch for %s: local exists=%t, proof exists=%t\n",
				addr.Hex(), accountExistsLocally, accountExistsInProof)
			continue
		}

		if !accountExistsInProof {
			// Account doesn't exist in either, skip
			continue
		}

		// Check account data
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

	// Check if there are accounts in the expected block that don't exist locally
	// (This is harder to do efficiently, but we can check some known accounts)
	knownAccounts := []common.Address{
		common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"), // ArbOS
		common.HexToAddress("0x00000000000000000000000000000000000A4B05"), // L1 pricing
		common.HexToAddress("0xA4b000000000000000000073657175656e636572"), // Batch poster
	}

	for _, addr := range knownAccounts {
		if !localAccounts[addr] {
			// Check if this account exists in the expected block
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

	// Check state root
	localRoot := statedb.IntermediateRoot(false)
	if localRoot != expectedHeader.Root {
		fmt.Printf("❌ State root mismatch: local %s, expected %s\n",
			localRoot.Hex(), expectedHeader.Root.Hex())
	} else {
		fmt.Printf("✅ State root matches!\n")
	}

	return nil
}

// Add this function to your ArbitrumClient
func (c *ArbitrumClient) DiagnoseArbOSStorageMismatch(ctx context.Context, statedb *state.StateDB, expectedHeader *types.Header) error {
	fmt.Printf("🔍 Diagnosing ArbOS storage mismatch...\n")

	arbosAddr := common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff")

	// Get proof for ArbOS with ALL possible storage slots
	allPossibleSlots := c.getAllPossibleArbOSStorageKeys()
	fmt.Printf("📊 Requesting proof with %d possible storage slots\n", len(allPossibleSlots))

	proof, err := c.GetProof(ctx, *expectedHeader.Number, arbosAddr, allPossibleSlots)
	if err != nil {
		return fmt.Errorf("failed to get proof: %w", err)
	}

	// Create a map of proof values
	proofMap := make(map[common.Hash]common.Hash)
	for _, sp := range proof.StorageProofs {
		proofMap[common.HexToHash(sp.Key)] = common.HexToHash(sp.Value)
	}

	// Check each possible slot
	missingSlots := []string{}
	mismatchedSlots := []string{}

	for _, slotHex := range allPossibleSlots {
		slot := common.HexToHash(slotHex)
		localValue := statedb.GetState(arbosAddr, slot)
		proofValue, exists := proofMap[slot]

		if !exists {
			if localValue != (common.Hash{}) {
				missingSlots = append(missingSlots, fmt.Sprintf("%s (local: %s, proof: missing)", slotHex, localValue.Hex()))
			}
		} else if localValue != proofValue {
			mismatchedSlots = append(mismatchedSlots, fmt.Sprintf("%s (local: %s, proof: %s)", slotHex, localValue.Hex(), proofValue.Hex()))
		}
	}

	fmt.Printf("�� Analysis Results:\n")
	fmt.Printf("  - Total possible slots: %d\n", len(allPossibleSlots))
	fmt.Printf("  - Slots in proof: %d\n", len(proof.StorageProofs))
	fmt.Printf("  - Missing slots: %d\n", len(missingSlots))
	fmt.Printf("  - Mismatched slots: %d\n", len(mismatchedSlots))

	if len(missingSlots) > 0 {
		fmt.Printf("❌ Missing slots in proof:\n")
		for _, slot := range missingSlots { // Show first 10
			fmt.Printf("  - %s\n", slot)
		}
	}

	if len(mismatchedSlots) > 0 {
		fmt.Printf("❌ Mismatched slots:\n")
		for _, slot := range mismatchedSlots { // Show first 10
			fmt.Printf("  - %s\n", slot)
		}
	}

	// Check storage root
	localStorageRoot := statedb.GetStorageRoot(arbosAddr)
	proofStorageHash := common.HexToHash(proof.StorageHash)

	fmt.Printf("🔍 Storage roots:\n")
	fmt.Printf("  - Local: %s\n", localStorageRoot.Hex())
	fmt.Printf("  - Proof: %s\n", proofStorageHash.Hex())

	return nil
}

// Enhanced function to get ALL possible ArbOS storage keys
func (c *ArbitrumClient) getAllPossibleArbOSStorageKeys() []string {
	// Helper function to compute storage keys like Nitro does
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

// Add this comprehensive comparison function to your ArbitrumClient
func (c *ArbitrumClient) CompareStateDBsWithSets(ctx context.Context, statedb1, statedb2 *state.StateDB, accountSet map[common.Address]struct{}, slotSet map[common.Address]map[common.Hash]struct{}) {
	fmt.Printf("🔍 Deep-dive comparison using accountSet and slotSet...\n")

	// arbOsAddress := common.HexToAddress("0xA4b05FffffFffFFFFfFFfffFfffFFfffFfFfFFFf")
	// statedb2.GetTrie().DeleteAccount(arbOsAddress)

	// // USE STATEDB1 TO POPULATE STATEDB2 WITH THIS...
	// // instead of this, can we use get proof and populate statedb2 like that?

	// statedb2.CreateAccount(arbOsAddress)
	// statedb2.SetBalance(arbOsAddress, statedb1.GetBalance(arbOsAddress), tracing.BalanceChangeDuringEVMExecution)
	// statedb2.SetCode(arbOsAddress, statedb1.GetCode(arbOsAddress))
	// statedb2.SetNonce(arbOsAddress, statedb1.GetNonce(arbOsAddress), tracing.NonceChangeAuthorization)

	// for slot := range slotSet[arbOsAddress] {
	// 	statedb2.SetState(arbOsAddress, slot, statedb1.GetState(arbOsAddress, slot))
	// }

	statedb2.GetTrie().DeleteAccount(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))
	statedb1.GetTrie().DeleteAccount(common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff"))

	// 1. Compare overall state roots
	root1 := statedb1.IntermediateRoot(false)
	root2 := statedb2.IntermediateRoot(false)
	fmt.Printf("📊 Overall State Roots:\n")
	fmt.Printf("  - StateDB1: %s\n", root1.Hex())
	fmt.Printf("  - StateDB2: %s\n", root2.Hex())
	fmt.Printf("  - Match: %t\n", root1 == root2)

	// 2. Compare accounts from accountSet
	fmt.Printf("\n Account Comparison (using accountSet):\n")
	accountDifferences := 0
	storageDifferences := 0

	for addr := range accountSet {
		fmt.Printf("\n🔍 Account: %s\n", addr.Hex())

		// Account existence
		exists1 := statedb1.Exist(addr)
		exists2 := statedb2.Exist(addr)
		fmt.Printf("  - Exists in StateDB1: %t\n", exists1)
		fmt.Printf("  - Exists in StateDB2: %t\n", exists2)

		if exists1 != exists2 {
			fmt.Printf("  ❌ EXISTENCE MISMATCH!\n")
			accountDifferences++
			continue
		}

		if !exists1 {
			fmt.Printf("  - Account doesn't exist in either\n")
			continue
		}

		// Account data comparison
		nonce1 := statedb1.GetNonce(addr)
		nonce2 := statedb2.GetNonce(addr)
		balance1 := statedb1.GetBalance(addr)
		balance2 := statedb2.GetBalance(addr)
		codeHash1 := statedb1.GetCodeHash(addr)
		codeHash2 := statedb2.GetCodeHash(addr)
		storageRoot1 := statedb1.GetStorageRoot(addr)
		storageRoot2 := statedb2.GetStorageRoot(addr)

		accountMismatch := false

		if nonce1 != nonce2 {
			fmt.Printf("  ❌ Nonce mismatch: %d vs %d\n", nonce1, nonce2)
			accountMismatch = true
		} else {
			fmt.Printf("  ✅ Nonce: %d\n", nonce1)
		}

		if balance1.Cmp(balance2) != 0 {
			fmt.Printf("  ❌ Balance mismatch: %s vs %s\n", balance1.String(), balance2.String())
			accountMismatch = true
		} else {
			fmt.Printf("  ✅ Balance: %s\n", balance1.String())
		}

		if codeHash1 != codeHash2 {
			fmt.Printf("  ❌ CodeHash mismatch: %s vs %s\n", codeHash1.Hex(), codeHash2.Hex())
			accountMismatch = true
		} else {
			fmt.Printf("  ✅ CodeHash: %s\n", codeHash1.Hex())
		}

		if storageRoot1 != storageRoot2 {
			fmt.Printf("  ❌ StorageRoot mismatch: %s vs %s\n", storageRoot1.Hex(), storageRoot2.Hex())
			accountMismatch = true
		} else {
			fmt.Printf("  ✅ StorageRoot: %s\n", storageRoot1.Hex())
		}

		if accountMismatch {
			accountDifferences++
		}

		// 3. Compare storage slots for this account
		if slots, exists := slotSet[addr]; exists {
			fmt.Printf("  🔍 Storage slots comparison:\n")
			slotMismatches := 0

			for slot := range slots {
				val1 := statedb1.GetState(addr, slot)
				val2 := statedb2.GetState(addr, slot)

				if val1 != val2 {
					fmt.Printf("    ❌ Slot %s: %s vs %s\n", slot.Hex(), val1.Hex(), val2.Hex())
					slotMismatches++
				} else {
					// fmt.Printf("    ✅ Slot %s: %s\n", slot.Hex(), val1.Hex())
				}
			}

			if slotMismatches > 0 {
				fmt.Printf("    📊 Total slot mismatches for %s: %d\n", addr.Hex(), slotMismatches)
				storageDifferences++
			} else {
				fmt.Printf("    ✅ All storage slots match for %s\n", addr.Hex())
			}
		} else {
			fmt.Printf("  ⏭️  No storage slots defined for this account\n")
		}
	}

	// 4. Summary
	fmt.Printf("\n�� Comparison Summary:\n")
	fmt.Printf("  - Total accounts compared: %d\n", len(accountSet))
	fmt.Printf("  - Account differences: %d\n", accountDifferences)
	fmt.Printf("  - Storage differences: %d\n", storageDifferences)

	if accountDifferences == 0 && storageDifferences == 0 {
		fmt.Printf("  ✅ All accounts and storage slots match!\n")
	} else {
		fmt.Printf("  ❌ Found differences in accounts and/or storage\n")
	}

	// 5. Special focus on ArbOS account if it has issues
	arbosAddr := common.HexToAddress("0xa4b05fffffffffffffffffffffffffffffffffff")
	if _, exists := accountSet[arbosAddr]; exists {
		fmt.Printf("\n🎯 Special ArbOS Analysis:\n")
		c.analyzeArbOSAccount(statedb1, statedb2, arbosAddr, slotSet)
	}
}

// Helper function to analyze ArbOS account specifically
func (c *ArbitrumClient) analyzeArbOSAccount(statedb1, statedb2 *state.StateDB, arbosAddr common.Address, slotSet map[common.Address]map[common.Hash]struct{}) {
	fmt.Printf("🔍 Detailed ArbOS account analysis:\n")

	storageRoot1 := statedb1.GetStorageRoot(arbosAddr)
	storageRoot2 := statedb2.GetStorageRoot(arbosAddr)

	fmt.Printf("  - StorageRoot1: %s\n", storageRoot1.Hex())
	fmt.Printf("  - StorageRoot2: %s\n", storageRoot2.Hex())

	if storageRoot1 == storageRoot2 {
		fmt.Printf("  ✅ Storage roots match!\n")
		return
	}

	fmt.Printf("  ❌ Storage roots differ! Analyzing storage slots...\n")

	// Get all possible ArbOS storage slots
	allArbOSSlots := c.getAllPossibleArbOSStorageKeys()
	fmt.Printf("  - Checking %d possible ArbOS storage slots\n", len(allArbOSSlots))

	mismatchedSlots := 0
	zeroSlots1 := 0
	zeroSlots2 := 0

	for _, slotHex := range allArbOSSlots {
		slot := common.HexToHash(slotHex)
		val1 := statedb1.GetState(arbosAddr, slot)
		val2 := statedb2.GetState(arbosAddr, slot)

		if val1 == (common.Hash{}) {
			zeroSlots1++
		}
		if val2 == (common.Hash{}) {
			zeroSlots2++
		}

		if val1 != val2 {
			mismatchedSlots++
			fmt.Printf("    ❌ Slot %s: %s vs %s\n", slotHex, val1.Hex(), val2.Hex())
		}
	}

	fmt.Printf("  📊 ArbOS Storage Analysis:\n")
	fmt.Printf("    - Total slots checked: %d\n", len(allArbOSSlots))
	fmt.Printf("    - Mismatched slots: %d\n", mismatchedSlots)
	fmt.Printf("    - Zero slots in StateDB1: %d\n", zeroSlots1)
	fmt.Printf("    - Zero slots in StateDB2: %d\n", zeroSlots2)
}

func (c *ArbitrumClient) InspectRawStorage(statedb *state.StateDB, addr common.Address) error {
	fmt.Printf("🔍 Force recomputing storage trie for: %s\n", addr.Hex())

	// Force the StateDB to compute the intermediate root
	// This commits all pending changes to the trie
	fmt.Printf("🔍 Nonce: %d\n", statedb.GetNonce(addr))
	root := statedb.IntermediateRoot(false)
	fmt.Printf("📊 Computed root: %s\n", root.Hex())

	// Get storage root
	storageRoot := statedb.GetStorageRoot(addr)
	fmt.Printf("�� Storage root: %s\n", storageRoot.Hex())

	if storageRoot == (common.Hash{}) {
		fmt.Printf("ℹ️  Account has no storage (empty root)\n")
		return nil
	}

	// Get the database
	db := statedb.Database()
	trieDB := db.TrieDB()

	// Create storage trie ID
	accountHash := crypto.Keccak256Hash(addr.Bytes())
	trieID := trie.StorageTrieID(root, accountHash, storageRoot)

	fmt.Printf("�� Storage trie ID: %s\n", trieID.StateRoot)

	// Try to open the storage trie
	storageTrie, err := trie.NewStateTrie(trieID, trieDB)
	if err != nil {
		return fmt.Errorf("failed to open storage trie: %w", err)
	}

	// Try to iterate
	it, err := storageTrie.NodeIterator(nil)
	if err != nil {
		return fmt.Errorf("failed to create iterator: %w", err)
	}

	nodeCount := 0
	for it.Next(true) {
		nodeCount++
		if it.Leaf() {
			fmt.Printf("  📄 Leaf node %d: Key=%x, Value=%x\n",
				nodeCount, it.LeafKey(), it.LeafBlob())
		} else {
			fmt.Printf("  🔗 Internal node %d: Hash=%x\n",
				nodeCount, it.Hash())
		}
	}

	fmt.Printf("📊 Found %d trie nodes after force recompute\n", nodeCount)
	return nil
}
