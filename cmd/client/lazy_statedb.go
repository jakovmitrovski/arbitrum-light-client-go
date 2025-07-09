package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
)

// Helper to check if a hash is zero
func isZeroHash(h common.Hash) bool {
	for _, b := range h {
		if b != 0 {
			return false
		}
	}
	return true
}

// LazyStateDB embeds StateDB and provides lazy loading of state from proofs
type LazyStateDB struct {
	client    *ArbitrumClient
	ctx       context.Context
	blockNum  *big.Int
	stateRoot common.Hash
	memdb     ethdb.Database
	trieDB    *triedb.Database

	// Cache for fetched accounts and their proofs
	accountCache map[common.Address]*accountData
	storageCache map[common.Address]map[common.Hash]common.Hash

	// The actual StateDB, created lazily
	statedb     *state.StateDB
	statedbLock sync.Mutex

	mu sync.RWMutex
}

type accountData struct {
	account *types.StateAccount
	proof   *EthGetProofResult
	loaded  bool
}

// NewLazyStateDB creates a new LazyStateDB that starts with empty state
func NewLazyStateDB(ctx context.Context, client *ArbitrumClient, blockNum *big.Int, stateRoot common.Hash) *LazyStateDB {
	memdb := rawdb.NewMemoryDatabase()
	trieDB := triedb.NewDatabase(memdb, nil)
	return &LazyStateDB{
		client:       client,
		ctx:          ctx,
		blockNum:     blockNum,
		stateRoot:    stateRoot,
		memdb:        memdb,
		trieDB:       trieDB,
		accountCache: make(map[common.Address]*accountData),
		storageCache: make(map[common.Address]map[common.Hash]common.Hash),
	}
}

// ensureAccountLoaded fetches and loads an account if not already loaded
func (l *LazyStateDB) ensureAccountLoaded(addr common.Address) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if cached, exists := l.accountCache[addr]; exists && cached.loaded {
		return nil
	}

	// Fetch proof for this account
	proof, err := l.client.GetProof(l.ctx, *l.blockNum, addr, []string{})
	if err != nil {
		return fmt.Errorf("get proof: %w", err)
	}
	if !l.client.VerifyStateProof(l.stateRoot, proof) {
		return fmt.Errorf("invalid proof for %s", addr)
	}

	// Parse account data from proof
	nonce, _ := new(big.Int).SetString(trimHexPrefix(proof.Nonce), 16)
	balance, _ := new(big.Int).SetString(trimHexPrefix(proof.Balance), 16)
	account := &types.StateAccount{
		Nonce:    nonce.Uint64(),
		Balance:  uint256.MustFromBig(balance),
		CodeHash: common.HexToHash(proof.CodeHash).Bytes(),
		Root:     common.HexToHash(proof.StorageHash),
	}

	// Store all trie nodes from the proof in the memdb
	db := l.memdb
	for _, encodedNode := range proof.AccountProof {
		nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
		if err != nil {
			return fmt.Errorf("decode account node: %w", err)
		}
		hash := crypto.Keccak256Hash(nodeBytes)
		if err := db.Put(hash.Bytes(), nodeBytes); err != nil {
			return fmt.Errorf("put account node: %w", err)
		}
	}
	for _, sp := range proof.StorageProofs {
		for _, encodedNode := range sp.Proof {
			nodeBytes, err := hex.DecodeString(trimHexPrefix(encodedNode))
			if err != nil {
				return fmt.Errorf("decode storage node: %w", err)
			}
			hash := crypto.Keccak256Hash(nodeBytes)
			if err := db.Put(hash.Bytes(), nodeBytes); err != nil {
				return fmt.Errorf("put storage node: %w", err)
			}
		}
	}

	// Cache the loaded account
	l.accountCache[addr] = &accountData{
		account: account,
		proof:   proof,
		loaded:  true,
	}
	l.storageCache[addr] = make(map[common.Hash]common.Hash)

	// Invalidate the statedb so it will be recreated with new trie nodes
	l.statedbLock.Lock()
	l.statedb = nil
	l.statedbLock.Unlock()

	return nil
}

// getStateDB creates the StateDB if needed, after all trie nodes are inserted
func (l *LazyStateDB) getStateDB() (*state.StateDB, error) {
	l.statedbLock.Lock()
	defer l.statedbLock.Unlock()
	if l.statedb != nil {
		return l.statedb, nil
	}
	stateDB := state.NewDatabase(l.trieDB, nil)
	statedb, err := state.NewDeterministic(l.stateRoot, stateDB)
	if err != nil {
		return nil, err
	}
	// For each loaded account, set nonce, balance, code, and storage
	for addr, data := range l.accountCache {
		statedb.CreateAccount(addr)
		statedb.SetNonce(addr, data.account.Nonce, tracing.NonceChangeUnspecified)
		statedb.SetBalance(addr, data.account.Balance, tracing.BalanceChangeUnspecified)
		if len(data.account.CodeHash) > 0 && !isZeroHash(common.BytesToHash(data.account.CodeHash)) {
			code, err := l.client.ethClient.CodeAt(l.ctx, addr, l.blockNum)
			if err == nil && len(code) > 0 {
				statedb.SetCode(addr, code)
			}
		}
		for _, sp := range data.proof.StorageProofs {
			key := common.HexToHash(sp.Key)
			value := new(big.Int)
			value.SetString(trimHexPrefix(sp.Value), 16)
			statedb.SetState(addr, key, common.BigToHash(value))
		}
	}
	l.statedb = statedb
	return statedb, nil
}

// Lazy loading methods - these trigger proof fetching when state is accessed
func (l *LazyStateDB) GetBalance(addr common.Address) *uint256.Int {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetBalance(addr)
}

func (l *LazyStateDB) GetNonce(addr common.Address) uint64 {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetNonce(addr)
}

func (l *LazyStateDB) GetCode(addr common.Address) []byte {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetCode(addr)
}

func (l *LazyStateDB) GetCodeHash(addr common.Address) common.Hash {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetCodeHash(addr)
}

func (l *LazyStateDB) GetState(addr common.Address, key common.Hash) common.Hash {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetState(addr, key)
}

func (l *LazyStateDB) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetCommittedState(addr, key)
}

func (l *LazyStateDB) GetStorageRoot(addr common.Address) common.Hash {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetStorageRoot(addr)
}

func (l *LazyStateDB) Exist(addr common.Address) bool {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.Exist(addr)
}

func (l *LazyStateDB) GetCodeSize(addr common.Address) int {
	_ = l.ensureAccountLoaded(addr)
	statedb, _ := l.getStateDB()
	return statedb.GetCodeSize(addr)
}

// Forward all other methods directly to the embedded StateDB
func (l *LazyStateDB) GetRefund() uint64    { statedb, _ := l.getStateDB(); return statedb.GetRefund() }
func (l *LazyStateDB) AddRefund(gas uint64) { statedb, _ := l.getStateDB(); statedb.AddRefund(gas) }
func (l *LazyStateDB) SubRefund(gas uint64) { statedb, _ := l.getStateDB(); statedb.SubRefund(gas) }

func (l *LazyStateDB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	statedb, _ := l.getStateDB()
	return statedb.GetTransientState(addr, key)
}

func (l *LazyStateDB) SetTransientState(addr common.Address, key, value common.Hash) {
	statedb, _ := l.getStateDB()
	statedb.SetTransientState(addr, key, value)
}

func (l *LazyStateDB) CreateAccount(addr common.Address) {
	statedb, _ := l.getStateDB()
	statedb.CreateAccount(addr)
}
func (l *LazyStateDB) CreateContract(addr common.Address) {
	statedb, _ := l.getStateDB()
	statedb.CreateContract(addr)
}

func (l *LazyStateDB) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	statedb, _ := l.getStateDB()
	return statedb.SubBalance(addr, amount, reason)
}

func (l *LazyStateDB) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	statedb, _ := l.getStateDB()
	return statedb.AddBalance(addr, amount, reason)
}

func (l *LazyStateDB) SetBalance(addr common.Address, balance *uint256.Int, reason tracing.BalanceChangeReason) {
	statedb, _ := l.getStateDB()
	statedb.SetBalance(addr, balance, reason)
}

func (l *LazyStateDB) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason) {
	statedb, _ := l.getStateDB()
	statedb.SetNonce(addr, nonce, reason)
}

func (l *LazyStateDB) SetCode(addr common.Address, code []byte) []byte {
	statedb, _ := l.getStateDB()
	return statedb.SetCode(addr, code)
}

func (l *LazyStateDB) SetState(addr common.Address, key, value common.Hash) common.Hash {
	statedb, _ := l.getStateDB()
	return statedb.SetState(addr, key, value)
}

func (l *LazyStateDB) SelfDestruct(addr common.Address) uint256.Int {
	statedb, _ := l.getStateDB()
	return statedb.SelfDestruct(addr)
}

func (l *LazyStateDB) SelfDestruct6780(addr common.Address) (uint256.Int, bool) {
	statedb, _ := l.getStateDB()
	return statedb.SelfDestruct6780(addr)
}

func (l *LazyStateDB) HasSelfDestructed(addr common.Address) bool {
	statedb, _ := l.getStateDB()
	return statedb.HasSelfDestructed(addr)
}

func (l *LazyStateDB) Database() state.Database {
	statedb, _ := l.getStateDB()
	return statedb.Database()
}

func (l *LazyStateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	statedb, _ := l.getStateDB()
	return statedb.IntermediateRoot(deleteEmptyObjects)
}

func (l *LazyStateDB) Commit(block uint64, deleteEmptyObjects bool, noStorageWiping bool) (common.Hash, error) {
	statedb, _ := l.getStateDB()
	return statedb.Commit(block, deleteEmptyObjects, noStorageWiping)
}

func (l *LazyStateDB) Finalise(deleteEmptyObjects bool) {
	statedb, _ := l.getStateDB()
	statedb.Finalise(deleteEmptyObjects)
}

func (l *LazyStateDB) RevertToSnapshot(revid int) {
	statedb, _ := l.getStateDB()
	statedb.RevertToSnapshot(revid)
}

func (l *LazyStateDB) Snapshot() int { statedb, _ := l.getStateDB(); return statedb.Snapshot() }

func (l *LazyStateDB) AddLog(log *types.Log) { statedb, _ := l.getStateDB(); statedb.AddLog(log) }

func (l *LazyStateDB) AddPreimage(hash common.Hash, preimage []byte) {
	statedb, _ := l.getStateDB()
	statedb.AddPreimage(hash, preimage)
}

func (l *LazyStateDB) Preimages() map[common.Hash][]byte {
	statedb, _ := l.getStateDB()
	return statedb.Preimages()
}

func (l *LazyStateDB) GetLogs(hash common.Hash, blockNumber uint64, blockHash common.Hash) []*types.Log {
	statedb, _ := l.getStateDB()
	return statedb.GetLogs(hash, blockNumber, blockHash)
}

func (l *LazyStateDB) Logs() []*types.Log { statedb, _ := l.getStateDB(); return statedb.Logs() }

func (l *LazyStateDB) FilterTx()      { statedb, _ := l.getStateDB(); statedb.FilterTx() }
func (l *LazyStateDB) ClearTxFilter() { statedb, _ := l.getStateDB(); statedb.ClearTxFilter() }
func (l *LazyStateDB) IsTxFiltered() bool {
	statedb, _ := l.getStateDB()
	return statedb.IsTxFiltered()
}

func (l *LazyStateDB) Recording() bool { statedb, _ := l.getStateDB(); return statedb.Recording() }
func (l *LazyStateDB) Deterministic() bool {
	statedb, _ := l.getStateDB()
	return statedb.Deterministic()
}

func (l *LazyStateDB) CreateZombieIfDeleted(addr common.Address) {
	statedb, _ := l.getStateDB()
	statedb.CreateZombieIfDeleted(addr)
}

func (l *LazyStateDB) GetStylusPages() (uint16, uint16) {
	statedb, _ := l.getStateDB()
	return statedb.GetStylusPages()
}
func (l *LazyStateDB) GetStylusPagesOpen() uint16 {
	statedb, _ := l.getStateDB()
	return statedb.GetStylusPagesOpen()
}
func (l *LazyStateDB) SetStylusPagesOpen(open uint16) {
	statedb, _ := l.getStateDB()
	statedb.SetStylusPagesOpen(open)
}
func (l *LazyStateDB) AddStylusPages(new uint16) (uint16, uint16) {
	statedb, _ := l.getStateDB()
	return statedb.AddStylusPages(new)
}
func (l *LazyStateDB) AddStylusPagesEver(new uint16) {
	statedb, _ := l.getStateDB()
	statedb.AddStylusPagesEver(new)
}

func (l *LazyStateDB) ActivateWasm(moduleHash common.Hash, asmMap map[ethdb.WasmTarget][]byte) {
	statedb, _ := l.getStateDB()
	statedb.ActivateWasm(moduleHash, asmMap)
}
func (l *LazyStateDB) TryGetActivatedAsm(target ethdb.WasmTarget, moduleHash common.Hash) (asm []byte, err error) {
	statedb, _ := l.getStateDB()
	return statedb.TryGetActivatedAsm(target, moduleHash)
}
func (l *LazyStateDB) TryGetActivatedAsmMap(targets []ethdb.WasmTarget, moduleHash common.Hash) (asmMap map[ethdb.WasmTarget][]byte, err error) {
	statedb, _ := l.getStateDB()
	return statedb.TryGetActivatedAsmMap(targets, moduleHash)
}
func (l *LazyStateDB) RecordCacheWasm(wasm state.CacheWasm) {
	statedb, _ := l.getStateDB()
	statedb.RecordCacheWasm(wasm)
}
func (l *LazyStateDB) RecordEvictWasm(wasm state.EvictWasm) {
	statedb, _ := l.getStateDB()
	statedb.RecordEvictWasm(wasm)
}
func (l *LazyStateDB) GetRecentWasms() state.RecentWasms {
	statedb, _ := l.getStateDB()
	return statedb.GetRecentWasms()
}

func (l *LazyStateDB) ExpectBalanceBurn(b *big.Int) {
	statedb, _ := l.getStateDB()
	statedb.ExpectBalanceBurn(b)
}
func (l *LazyStateDB) GetSelfDestructs() []common.Address {
	statedb, _ := l.getStateDB()
	return statedb.GetSelfDestructs()
}
func (l *LazyStateDB) GetCurrentTxLogs() []*types.Log {
	statedb, _ := l.getStateDB()
	return statedb.GetCurrentTxLogs()
}
func (l *LazyStateDB) GetUnexpectedBalanceDelta() *big.Int {
	statedb, _ := l.getStateDB()
	return statedb.GetUnexpectedBalanceDelta()
}

func (l *LazyStateDB) SetArbFinalizer(f func(*state.ArbitrumExtraData)) {
	statedb, _ := l.getStateDB()
	statedb.SetArbFinalizer(f)
}

func (l *LazyStateDB) AccessEvents() *state.AccessEvents {
	statedb, _ := l.getStateDB()
	return statedb.AccessEvents()
}
func (l *LazyStateDB) AddAddressToAccessList(addr common.Address) {
	statedb, _ := l.getStateDB()
	statedb.AddAddressToAccessList(addr)
}
func (l *LazyStateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	statedb, _ := l.getStateDB()
	statedb.AddSlotToAccessList(addr, slot)
}
func (l *LazyStateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	statedb, _ := l.getStateDB()
	return statedb.SlotInAccessList(addr, slot)
}
func (l *LazyStateDB) AddressInAccessList(addr common.Address) bool {
	statedb, _ := l.getStateDB()
	return statedb.AddressInAccessList(addr)
}

func (l *LazyStateDB) Empty(addr common.Address) bool {
	statedb, _ := l.getStateDB()
	return statedb.Empty(addr)
}
func (l *LazyStateDB) PointCache() *utils.PointCache {
	statedb, _ := l.getStateDB()
	return statedb.PointCache()
}
func (l *LazyStateDB) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	statedb, _ := l.getStateDB()
	statedb.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
}

func (l *LazyStateDB) Witness() *stateless.Witness {
	statedb, _ := l.getStateDB()
	return statedb.Witness()
}
func (l *LazyStateDB) Reader() state.Reader { statedb, _ := l.getStateDB(); return statedb.Reader() }

// Additional methods that *state.StateDB implements
func (l *LazyStateDB) Error() error         { statedb, _ := l.getStateDB(); return statedb.Error() }
func (l *LazyStateDB) TxIndex() int         { statedb, _ := l.getStateDB(); return statedb.TxIndex() }
func (l *LazyStateDB) GetTrie() state.Trie  { statedb, _ := l.getStateDB(); return statedb.GetTrie() }
func (l *LazyStateDB) Copy() *state.StateDB { statedb, _ := l.getStateDB(); return statedb.Copy() }
func (l *LazyStateDB) SetTxContext(thash common.Hash, ti int) {
	statedb, _ := l.getStateDB()
	statedb.SetTxContext(thash, ti)
}

// Dump methods
func (l *LazyStateDB) Dump(opts *state.DumpConfig) []byte {
	statedb, _ := l.getStateDB()
	return statedb.Dump(opts)
}
func (l *LazyStateDB) RawDump(opts *state.DumpConfig) state.Dump {
	statedb, _ := l.getStateDB()
	return statedb.RawDump(opts)
}
func (l *LazyStateDB) DumpToCollector(c state.DumpCollector, conf *state.DumpConfig) (nextKey []byte) {
	statedb, _ := l.getStateDB()
	return statedb.DumpToCollector(c, conf)
}
func (l *LazyStateDB) IterativeDump(opts *state.DumpConfig, output *json.Encoder) {
	statedb, _ := l.getStateDB()
	statedb.IterativeDump(opts, output)
}

// Prefetcher methods
func (l *LazyStateDB) StartPrefetcher(namespace string, witness *stateless.Witness) {
	statedb, _ := l.getStateDB()
	statedb.StartPrefetcher(namespace, witness)
}
func (l *LazyStateDB) StopPrefetcher() { statedb, _ := l.getStateDB(); statedb.StopPrefetcher() }

// Recording methods
func (l *LazyStateDB) StartRecording() { statedb, _ := l.getStateDB(); statedb.StartRecording() }
func (l *LazyStateDB) RecordProgram(targets []ethdb.WasmTarget, moduleHash common.Hash) {
	statedb, _ := l.getStateDB()
	statedb.RecordProgram(targets, moduleHash)
}
func (l *LazyStateDB) UserWasms() state.UserWasms {
	statedb, _ := l.getStateDB()
	return statedb.UserWasms()
}

// Storage methods
func (l *LazyStateDB) SetStorage(addr common.Address, storage map[common.Hash]common.Hash) {
	statedb, _ := l.getStateDB()
	statedb.SetStorage(addr, storage)
}
