// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package rollupcore

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// RollupCoreAssertionInputs is an auto generated low-level Go binding around an user-defined struct.
type RollupCoreAssertionInputs struct {
	BeforeStateData RollupCoreBeforeStateData
	BeforeState     RollupCoreAssertionState
	AfterState      RollupCoreAssertionState
}

// RollupCoreAssertionNode is an auto generated low-level Go binding around an user-defined struct.
type RollupCoreAssertionNode struct {
	FirstChildBlock  uint64
	SecondChildBlock uint64
	CreatedAtBlock   uint64
	IsFirstChild     bool
	Status           uint8
	ConfigHash       [32]byte
}

// RollupCoreAssertionState is an auto generated low-level Go binding around an user-defined struct.
type RollupCoreAssertionState struct {
	GlobalState    RollupCoreGlobalState
	MachineStatus  uint8
	EndHistoryRoot [32]byte
}

// RollupCoreBeforeStateData is an auto generated low-level Go binding around an user-defined struct.
type RollupCoreBeforeStateData struct {
	PrevPrevAssertionHash [32]byte
	SequencerBatchAcc     [32]byte
	ConfigData            RollupCoreConfigData
}

// RollupCoreConfigData is an auto generated low-level Go binding around an user-defined struct.
type RollupCoreConfigData struct {
	WasmModuleRoot      [32]byte
	RequiredStake       *big.Int
	ChallengeManager    common.Address
	ConfirmPeriodBlocks uint64
	NextInboxPosition   uint64
}

// RollupCoreGlobalState is an auto generated low-level Go binding around an user-defined struct.
type RollupCoreGlobalState struct {
	Bytes32Vals [2][32]byte
	U64Vals     [2]uint64
}

// RollupCoreMetaData contains all meta data concerning the RollupCore contract.
var RollupCoreMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"assertionHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"blockHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"sendRoot\",\"type\":\"bytes32\"}],\"name\":\"AssertionConfirmed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"assertionHash\",\"type\":\"bytes32\"},{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"parentAssertionHash\",\"type\":\"bytes32\"},{\"components\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"prevPrevAssertionHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"sequencerBatchAcc\",\"type\":\"bytes32\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"wasmModuleRoot\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"requiredStake\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"challengeManager\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"confirmPeriodBlocks\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"nextInboxPosition\",\"type\":\"uint64\"}],\"internalType\":\"structRollupCore.ConfigData\",\"name\":\"configData\",\"type\":\"tuple\"}],\"internalType\":\"structRollupCore.BeforeStateData\",\"name\":\"beforeStateData\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"bytes32[2]\",\"name\":\"bytes32Vals\",\"type\":\"bytes32[2]\"},{\"internalType\":\"uint64[2]\",\"name\":\"u64Vals\",\"type\":\"uint64[2]\"}],\"internalType\":\"structRollupCore.GlobalState\",\"name\":\"globalState\",\"type\":\"tuple\"},{\"internalType\":\"enumRollupCore.MachineStatus\",\"name\":\"machineStatus\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"endHistoryRoot\",\"type\":\"bytes32\"}],\"internalType\":\"structRollupCore.AssertionState\",\"name\":\"beforeState\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"bytes32[2]\",\"name\":\"bytes32Vals\",\"type\":\"bytes32[2]\"},{\"internalType\":\"uint64[2]\",\"name\":\"u64Vals\",\"type\":\"uint64[2]\"}],\"internalType\":\"structRollupCore.GlobalState\",\"name\":\"globalState\",\"type\":\"tuple\"},{\"internalType\":\"enumRollupCore.MachineStatus\",\"name\":\"machineStatus\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"endHistoryRoot\",\"type\":\"bytes32\"}],\"internalType\":\"structRollupCore.AssertionState\",\"name\":\"afterState\",\"type\":\"tuple\"}],\"indexed\":false,\"internalType\":\"structRollupCore.AssertionInputs\",\"name\":\"assertion\",\"type\":\"tuple\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"afterInboxBatchAcc\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"inboxMaxCount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"wasmModuleRoot\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"requiredStake\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"challengeManager\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"confirmPeriodBlocks\",\"type\":\"uint64\"}],\"name\":\"AssertionCreated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"assertionHash\",\"type\":\"bytes32\"}],\"name\":\"getAssertion\",\"outputs\":[{\"components\":[{\"internalType\":\"uint64\",\"name\":\"firstChildBlock\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"secondChildBlock\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"createdAtBlock\",\"type\":\"uint64\"},{\"internalType\":\"bool\",\"name\":\"isFirstChild\",\"type\":\"bool\"},{\"internalType\":\"enumRollupCore.AssertionStatus\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"configHash\",\"type\":\"bytes32\"}],\"internalType\":\"structRollupCore.AssertionNode\",\"name\":\"assertion\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"latestConfirmed\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"assertionHash\",\"type\":\"bytes32\"},{\"components\":[{\"components\":[{\"internalType\":\"bytes32[2]\",\"name\":\"bytes32Vals\",\"type\":\"bytes32[2]\"},{\"internalType\":\"uint64[2]\",\"name\":\"u64Vals\",\"type\":\"uint64[2]\"}],\"internalType\":\"structRollupCore.GlobalState\",\"name\":\"globalState\",\"type\":\"tuple\"},{\"internalType\":\"enumRollupCore.MachineStatus\",\"name\":\"machineStatus\",\"type\":\"uint8\"},{\"internalType\":\"bytes32\",\"name\":\"endHistoryRoot\",\"type\":\"bytes32\"}],\"internalType\":\"structRollupCore.AssertionState\",\"name\":\"state\",\"type\":\"tuple\"},{\"internalType\":\"bytes32\",\"name\":\"prevAssertionHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"inboxAcc\",\"type\":\"bytes32\"}],\"name\":\"validateAssertionHash\",\"outputs\":[],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"assertionHash\",\"type\":\"bytes32\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"wasmModuleRoot\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"requiredStake\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"challengeManager\",\"type\":\"address\"},{\"internalType\":\"uint64\",\"name\":\"confirmPeriodBlocks\",\"type\":\"uint64\"},{\"internalType\":\"uint64\",\"name\":\"nextInboxPosition\",\"type\":\"uint64\"}],\"internalType\":\"structRollupCore.ConfigData\",\"name\":\"configData\",\"type\":\"tuple\"}],\"name\":\"validateConfig\",\"outputs\":[],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// RollupCoreABI is the input ABI used to generate the binding from.
// Deprecated: Use RollupCoreMetaData.ABI instead.
var RollupCoreABI = RollupCoreMetaData.ABI

// RollupCore is an auto generated Go binding around an Ethereum contract.
type RollupCore struct {
	RollupCoreCaller     // Read-only binding to the contract
	RollupCoreTransactor // Write-only binding to the contract
	RollupCoreFilterer   // Log filterer for contract events
}

// RollupCoreCaller is an auto generated read-only Go binding around an Ethereum contract.
type RollupCoreCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RollupCoreTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RollupCoreTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RollupCoreFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RollupCoreFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RollupCoreSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RollupCoreSession struct {
	Contract     *RollupCore       // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RollupCoreCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RollupCoreCallerSession struct {
	Contract *RollupCoreCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts     // Call options to use throughout this session
}

// RollupCoreTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RollupCoreTransactorSession struct {
	Contract     *RollupCoreTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// RollupCoreRaw is an auto generated low-level Go binding around an Ethereum contract.
type RollupCoreRaw struct {
	Contract *RollupCore // Generic contract binding to access the raw methods on
}

// RollupCoreCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RollupCoreCallerRaw struct {
	Contract *RollupCoreCaller // Generic read-only contract binding to access the raw methods on
}

// RollupCoreTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RollupCoreTransactorRaw struct {
	Contract *RollupCoreTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRollupCore creates a new instance of RollupCore, bound to a specific deployed contract.
func NewRollupCore(address common.Address, backend bind.ContractBackend) (*RollupCore, error) {
	contract, err := bindRollupCore(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &RollupCore{RollupCoreCaller: RollupCoreCaller{contract: contract}, RollupCoreTransactor: RollupCoreTransactor{contract: contract}, RollupCoreFilterer: RollupCoreFilterer{contract: contract}}, nil
}

// NewRollupCoreCaller creates a new read-only instance of RollupCore, bound to a specific deployed contract.
func NewRollupCoreCaller(address common.Address, caller bind.ContractCaller) (*RollupCoreCaller, error) {
	contract, err := bindRollupCore(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RollupCoreCaller{contract: contract}, nil
}

// NewRollupCoreTransactor creates a new write-only instance of RollupCore, bound to a specific deployed contract.
func NewRollupCoreTransactor(address common.Address, transactor bind.ContractTransactor) (*RollupCoreTransactor, error) {
	contract, err := bindRollupCore(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RollupCoreTransactor{contract: contract}, nil
}

// NewRollupCoreFilterer creates a new log filterer instance of RollupCore, bound to a specific deployed contract.
func NewRollupCoreFilterer(address common.Address, filterer bind.ContractFilterer) (*RollupCoreFilterer, error) {
	contract, err := bindRollupCore(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RollupCoreFilterer{contract: contract}, nil
}

// bindRollupCore binds a generic wrapper to an already deployed contract.
func bindRollupCore(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := RollupCoreMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RollupCore *RollupCoreRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RollupCore.Contract.RollupCoreCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RollupCore *RollupCoreRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RollupCore.Contract.RollupCoreTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RollupCore *RollupCoreRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RollupCore.Contract.RollupCoreTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_RollupCore *RollupCoreCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _RollupCore.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_RollupCore *RollupCoreTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _RollupCore.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_RollupCore *RollupCoreTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _RollupCore.Contract.contract.Transact(opts, method, params...)
}

// GetAssertion is a free data retrieval call binding the contract method 0x88302884.
//
// Solidity: function getAssertion(bytes32 assertionHash) view returns((uint64,uint64,uint64,bool,uint8,bytes32) assertion)
func (_RollupCore *RollupCoreCaller) GetAssertion(opts *bind.CallOpts, assertionHash [32]byte) (RollupCoreAssertionNode, error) {
	var out []interface{}
	err := _RollupCore.contract.Call(opts, &out, "getAssertion", assertionHash)

	if err != nil {
		return *new(RollupCoreAssertionNode), err
	}

	out0 := *abi.ConvertType(out[0], new(RollupCoreAssertionNode)).(*RollupCoreAssertionNode)

	return out0, err

}

// GetAssertion is a free data retrieval call binding the contract method 0x88302884.
//
// Solidity: function getAssertion(bytes32 assertionHash) view returns((uint64,uint64,uint64,bool,uint8,bytes32) assertion)
func (_RollupCore *RollupCoreSession) GetAssertion(assertionHash [32]byte) (RollupCoreAssertionNode, error) {
	return _RollupCore.Contract.GetAssertion(&_RollupCore.CallOpts, assertionHash)
}

// GetAssertion is a free data retrieval call binding the contract method 0x88302884.
//
// Solidity: function getAssertion(bytes32 assertionHash) view returns((uint64,uint64,uint64,bool,uint8,bytes32) assertion)
func (_RollupCore *RollupCoreCallerSession) GetAssertion(assertionHash [32]byte) (RollupCoreAssertionNode, error) {
	return _RollupCore.Contract.GetAssertion(&_RollupCore.CallOpts, assertionHash)
}

// LatestConfirmed is a free data retrieval call binding the contract method 0x65f7f80d.
//
// Solidity: function latestConfirmed() view returns(bytes32)
func (_RollupCore *RollupCoreCaller) LatestConfirmed(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _RollupCore.contract.Call(opts, &out, "latestConfirmed")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// LatestConfirmed is a free data retrieval call binding the contract method 0x65f7f80d.
//
// Solidity: function latestConfirmed() view returns(bytes32)
func (_RollupCore *RollupCoreSession) LatestConfirmed() ([32]byte, error) {
	return _RollupCore.Contract.LatestConfirmed(&_RollupCore.CallOpts)
}

// LatestConfirmed is a free data retrieval call binding the contract method 0x65f7f80d.
//
// Solidity: function latestConfirmed() view returns(bytes32)
func (_RollupCore *RollupCoreCallerSession) LatestConfirmed() ([32]byte, error) {
	return _RollupCore.Contract.LatestConfirmed(&_RollupCore.CallOpts)
}

// ValidateAssertionHash is a free data retrieval call binding the contract method 0xe51019a6.
//
// Solidity: function validateAssertionHash(bytes32 assertionHash, ((bytes32[2],uint64[2]),uint8,bytes32) state, bytes32 prevAssertionHash, bytes32 inboxAcc) pure returns()
func (_RollupCore *RollupCoreCaller) ValidateAssertionHash(opts *bind.CallOpts, assertionHash [32]byte, state RollupCoreAssertionState, prevAssertionHash [32]byte, inboxAcc [32]byte) error {
	var out []interface{}
	err := _RollupCore.contract.Call(opts, &out, "validateAssertionHash", assertionHash, state, prevAssertionHash, inboxAcc)

	if err != nil {
		return err
	}

	return err

}

// ValidateAssertionHash is a free data retrieval call binding the contract method 0xe51019a6.
//
// Solidity: function validateAssertionHash(bytes32 assertionHash, ((bytes32[2],uint64[2]),uint8,bytes32) state, bytes32 prevAssertionHash, bytes32 inboxAcc) pure returns()
func (_RollupCore *RollupCoreSession) ValidateAssertionHash(assertionHash [32]byte, state RollupCoreAssertionState, prevAssertionHash [32]byte, inboxAcc [32]byte) error {
	return _RollupCore.Contract.ValidateAssertionHash(&_RollupCore.CallOpts, assertionHash, state, prevAssertionHash, inboxAcc)
}

// ValidateAssertionHash is a free data retrieval call binding the contract method 0xe51019a6.
//
// Solidity: function validateAssertionHash(bytes32 assertionHash, ((bytes32[2],uint64[2]),uint8,bytes32) state, bytes32 prevAssertionHash, bytes32 inboxAcc) pure returns()
func (_RollupCore *RollupCoreCallerSession) ValidateAssertionHash(assertionHash [32]byte, state RollupCoreAssertionState, prevAssertionHash [32]byte, inboxAcc [32]byte) error {
	return _RollupCore.Contract.ValidateAssertionHash(&_RollupCore.CallOpts, assertionHash, state, prevAssertionHash, inboxAcc)
}

// ValidateConfig is a free data retrieval call binding the contract method 0x04972af9.
//
// Solidity: function validateConfig(bytes32 assertionHash, (bytes32,uint256,address,uint64,uint64) configData) view returns()
func (_RollupCore *RollupCoreCaller) ValidateConfig(opts *bind.CallOpts, assertionHash [32]byte, configData RollupCoreConfigData) error {
	var out []interface{}
	err := _RollupCore.contract.Call(opts, &out, "validateConfig", assertionHash, configData)

	if err != nil {
		return err
	}

	return err

}

// ValidateConfig is a free data retrieval call binding the contract method 0x04972af9.
//
// Solidity: function validateConfig(bytes32 assertionHash, (bytes32,uint256,address,uint64,uint64) configData) view returns()
func (_RollupCore *RollupCoreSession) ValidateConfig(assertionHash [32]byte, configData RollupCoreConfigData) error {
	return _RollupCore.Contract.ValidateConfig(&_RollupCore.CallOpts, assertionHash, configData)
}

// ValidateConfig is a free data retrieval call binding the contract method 0x04972af9.
//
// Solidity: function validateConfig(bytes32 assertionHash, (bytes32,uint256,address,uint64,uint64) configData) view returns()
func (_RollupCore *RollupCoreCallerSession) ValidateConfig(assertionHash [32]byte, configData RollupCoreConfigData) error {
	return _RollupCore.Contract.ValidateConfig(&_RollupCore.CallOpts, assertionHash, configData)
}

// RollupCoreAssertionConfirmedIterator is returned from FilterAssertionConfirmed and is used to iterate over the raw logs and unpacked data for AssertionConfirmed events raised by the RollupCore contract.
type RollupCoreAssertionConfirmedIterator struct {
	Event *RollupCoreAssertionConfirmed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RollupCoreAssertionConfirmedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RollupCoreAssertionConfirmed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RollupCoreAssertionConfirmed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RollupCoreAssertionConfirmedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RollupCoreAssertionConfirmedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RollupCoreAssertionConfirmed represents a AssertionConfirmed event raised by the RollupCore contract.
type RollupCoreAssertionConfirmed struct {
	AssertionHash [32]byte
	BlockHash     [32]byte
	SendRoot      [32]byte
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterAssertionConfirmed is a free log retrieval operation binding the contract event 0xfc42829b29c259a7370ab56c8f69fce23b5f351a9ce151da453281993ec0090c.
//
// Solidity: event AssertionConfirmed(bytes32 indexed assertionHash, bytes32 blockHash, bytes32 sendRoot)
func (_RollupCore *RollupCoreFilterer) FilterAssertionConfirmed(opts *bind.FilterOpts, assertionHash [][32]byte) (*RollupCoreAssertionConfirmedIterator, error) {

	var assertionHashRule []interface{}
	for _, assertionHashItem := range assertionHash {
		assertionHashRule = append(assertionHashRule, assertionHashItem)
	}

	logs, sub, err := _RollupCore.contract.FilterLogs(opts, "AssertionConfirmed", assertionHashRule)
	if err != nil {
		return nil, err
	}
	return &RollupCoreAssertionConfirmedIterator{contract: _RollupCore.contract, event: "AssertionConfirmed", logs: logs, sub: sub}, nil
}

// WatchAssertionConfirmed is a free log subscription operation binding the contract event 0xfc42829b29c259a7370ab56c8f69fce23b5f351a9ce151da453281993ec0090c.
//
// Solidity: event AssertionConfirmed(bytes32 indexed assertionHash, bytes32 blockHash, bytes32 sendRoot)
func (_RollupCore *RollupCoreFilterer) WatchAssertionConfirmed(opts *bind.WatchOpts, sink chan<- *RollupCoreAssertionConfirmed, assertionHash [][32]byte) (event.Subscription, error) {

	var assertionHashRule []interface{}
	for _, assertionHashItem := range assertionHash {
		assertionHashRule = append(assertionHashRule, assertionHashItem)
	}

	logs, sub, err := _RollupCore.contract.WatchLogs(opts, "AssertionConfirmed", assertionHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RollupCoreAssertionConfirmed)
				if err := _RollupCore.contract.UnpackLog(event, "AssertionConfirmed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAssertionConfirmed is a log parse operation binding the contract event 0xfc42829b29c259a7370ab56c8f69fce23b5f351a9ce151da453281993ec0090c.
//
// Solidity: event AssertionConfirmed(bytes32 indexed assertionHash, bytes32 blockHash, bytes32 sendRoot)
func (_RollupCore *RollupCoreFilterer) ParseAssertionConfirmed(log types.Log) (*RollupCoreAssertionConfirmed, error) {
	event := new(RollupCoreAssertionConfirmed)
	if err := _RollupCore.contract.UnpackLog(event, "AssertionConfirmed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// RollupCoreAssertionCreatedIterator is returned from FilterAssertionCreated and is used to iterate over the raw logs and unpacked data for AssertionCreated events raised by the RollupCore contract.
type RollupCoreAssertionCreatedIterator struct {
	Event *RollupCoreAssertionCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *RollupCoreAssertionCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(RollupCoreAssertionCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(RollupCoreAssertionCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *RollupCoreAssertionCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *RollupCoreAssertionCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// RollupCoreAssertionCreated represents a AssertionCreated event raised by the RollupCore contract.
type RollupCoreAssertionCreated struct {
	AssertionHash       [32]byte
	ParentAssertionHash [32]byte
	Assertion           RollupCoreAssertionInputs
	AfterInboxBatchAcc  [32]byte
	InboxMaxCount       *big.Int
	WasmModuleRoot      [32]byte
	RequiredStake       *big.Int
	ChallengeManager    common.Address
	ConfirmPeriodBlocks uint64
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterAssertionCreated is a free log retrieval operation binding the contract event 0x901c3aee23cf4478825462caaab375c606ab83516060388344f0650340753630.
//
// Solidity: event AssertionCreated(bytes32 indexed assertionHash, bytes32 indexed parentAssertionHash, ((bytes32,bytes32,(bytes32,uint256,address,uint64,uint64)),((bytes32[2],uint64[2]),uint8,bytes32),((bytes32[2],uint64[2]),uint8,bytes32)) assertion, bytes32 afterInboxBatchAcc, uint256 inboxMaxCount, bytes32 wasmModuleRoot, uint256 requiredStake, address challengeManager, uint64 confirmPeriodBlocks)
func (_RollupCore *RollupCoreFilterer) FilterAssertionCreated(opts *bind.FilterOpts, assertionHash [][32]byte, parentAssertionHash [][32]byte) (*RollupCoreAssertionCreatedIterator, error) {

	var assertionHashRule []interface{}
	for _, assertionHashItem := range assertionHash {
		assertionHashRule = append(assertionHashRule, assertionHashItem)
	}
	var parentAssertionHashRule []interface{}
	for _, parentAssertionHashItem := range parentAssertionHash {
		parentAssertionHashRule = append(parentAssertionHashRule, parentAssertionHashItem)
	}

	logs, sub, err := _RollupCore.contract.FilterLogs(opts, "AssertionCreated", assertionHashRule, parentAssertionHashRule)
	if err != nil {
		return nil, err
	}
	return &RollupCoreAssertionCreatedIterator{contract: _RollupCore.contract, event: "AssertionCreated", logs: logs, sub: sub}, nil
}

// WatchAssertionCreated is a free log subscription operation binding the contract event 0x901c3aee23cf4478825462caaab375c606ab83516060388344f0650340753630.
//
// Solidity: event AssertionCreated(bytes32 indexed assertionHash, bytes32 indexed parentAssertionHash, ((bytes32,bytes32,(bytes32,uint256,address,uint64,uint64)),((bytes32[2],uint64[2]),uint8,bytes32),((bytes32[2],uint64[2]),uint8,bytes32)) assertion, bytes32 afterInboxBatchAcc, uint256 inboxMaxCount, bytes32 wasmModuleRoot, uint256 requiredStake, address challengeManager, uint64 confirmPeriodBlocks)
func (_RollupCore *RollupCoreFilterer) WatchAssertionCreated(opts *bind.WatchOpts, sink chan<- *RollupCoreAssertionCreated, assertionHash [][32]byte, parentAssertionHash [][32]byte) (event.Subscription, error) {

	var assertionHashRule []interface{}
	for _, assertionHashItem := range assertionHash {
		assertionHashRule = append(assertionHashRule, assertionHashItem)
	}
	var parentAssertionHashRule []interface{}
	for _, parentAssertionHashItem := range parentAssertionHash {
		parentAssertionHashRule = append(parentAssertionHashRule, parentAssertionHashItem)
	}

	logs, sub, err := _RollupCore.contract.WatchLogs(opts, "AssertionCreated", assertionHashRule, parentAssertionHashRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(RollupCoreAssertionCreated)
				if err := _RollupCore.contract.UnpackLog(event, "AssertionCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAssertionCreated is a log parse operation binding the contract event 0x901c3aee23cf4478825462caaab375c606ab83516060388344f0650340753630.
//
// Solidity: event AssertionCreated(bytes32 indexed assertionHash, bytes32 indexed parentAssertionHash, ((bytes32,bytes32,(bytes32,uint256,address,uint64,uint64)),((bytes32[2],uint64[2]),uint8,bytes32),((bytes32[2],uint64[2]),uint8,bytes32)) assertion, bytes32 afterInboxBatchAcc, uint256 inboxMaxCount, bytes32 wasmModuleRoot, uint256 requiredStake, address challengeManager, uint64 confirmPeriodBlocks)
func (_RollupCore *RollupCoreFilterer) ParseAssertionCreated(log types.Log) (*RollupCoreAssertionCreated, error) {
	event := new(RollupCoreAssertionCreated)
	if err := _RollupCore.contract.UnpackLog(event, "AssertionCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
