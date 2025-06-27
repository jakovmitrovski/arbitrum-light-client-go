package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/joho/godotenv"
	"github.com/offchainlabs/nitro/arbos"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
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
	proverRpcURLs := strings.Split(os.Getenv("PROVERS"), ";")
	rollupCoreAddr := common.HexToAddress(os.Getenv("ROLLUP_CORE_ADDRESS"))

	// accountAddress := common.HexToAddress(os.Getenv("ACCOUNT_ADDRESS"))

	ethClient, err := NewEthereumClient(ethRpcURL, rollupCoreAddr)
	if err != nil {
		log.Fatalf("Failed to init Ethereum client: %v", err)
	}

	arbClients := make([]*ArbitrumClient, len(proverRpcURLs))
	for i, proverRpcURL := range proverRpcURLs {
		arbClients[i], err = NewArbitrumClient(proverRpcURL)
		if err != nil {
			log.Fatalf("Failed to init Arbitrum client: %v", err)
		}
	}

	// // 1. Fetch latest confirmed assertion
	latestAssertion, err := ethClient.GetLatestAssertion(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest assertion: %v", err)
	}
	fmt.Printf("Latest Confirmed Assertion: 0x%s\n", hex.EncodeToString(latestAssertion[:]))

	// // 2. Fetch assertion node details
	// assertionNode, err := ethClient.GetAssertionDetails(ctx, latestAssertion)
	// if err != nil {
	// 	log.Fatalf("Failed to get assertion details: %v", err)
	// }
	// fmt.Printf("Assertion: %+v\n", assertionNode)

	// // 3. Get AssertionConfirmed log
	// confirmedLog, err := ethClient.GetAssertionConfirmedLog(ctx)
	// if err != nil {
	// 	log.Fatalf("Failed to get AssertionConfirmed: %v", err)
	// }
	// fmt.Printf("AssertionConfirmed: %+v\n", confirmedLog)

	// // 4. Get AssertionCreated log
	// createdLog, err := ethClient.GetAssertionCreatedLog(ctx)
	// if err != nil {
	// 	log.Fatalf("Failed to get AssertionCreated: %v", err)
	// }
	// fmt.Printf("AssertionCreated: %+v\n", createdLog)

	// fmt.Printf("AVM State root: 0x%x\n", createdLog.Assertion.AfterState.EndHistoryRoot)

	// // 5. Fetch and verify block from L2
	// block, err := arbClients[0].GetBlockByHash(ctx, confirmedLog.BlockHash)
	// 2 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xe1ec95df182d2d335e72aae5dcac142210b30c24dbff2a695deb8c0bbf991533"))
	// 3 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0x66aa454cd2c6e38af14d44a1697a0c010da712bae577fdaaf299bd208532e134"))
	// 4 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xb9dc275d50509340a54e252ae6430d3053b4da5ac69c814c183e18f36810b7f0"))
	// if err != nil {
	// 	log.Fatalf("Failed to get L2 block by hash: %v", err)
	// }

	// 6. verify the block hash...
	// verified := arbClients[0].VerifyBlockHash(block.Header(), confirmedLog.BlockHash)
	// fmt.Printf("Block hash verified: %v\n", verified)

	// genesis block...
	genesisBlock, err := arbClients[0].GetBlockByNumber(ctx, big.NewInt(0))
	if err != nil {
		log.Fatalf("Failed to get genesis block: %v", err)
	}
	fmt.Printf("Genesis block: %v\n", genesisBlock.Header().Hash())

	// // 7. Get and verify state proof
	// proof, err := arbClient.GetProof(ctx, *block.Header().Number, accountAddress, []string{})
	// if err != nil {
	// 	log.Fatalf("Failed to get proof: %v", err)
	// }

	// stateVerified := arbClient.VerifyStateProof(block.Header().Root, proof)
	// fmt.Printf("Verified: %v\n", stateVerified)
	// fmt.Printf("State root: 0x%x\n", block.Header().Root)

	beaconRpcURL := os.Getenv("ETHEREUM_BEACON_RPC_URL")
	arbChainId, err := strconv.ParseUint(os.Getenv("ARBITRUM_ONE_CHAIN_ID"), 10, 64)

	if err != nil {
		log.Fatalf("Error parsing ARBITRUM_CHAIN_ID: %v", err)
	}

	tournament(ctx, arbClients, ethClient, ethRpcURL, arbChainId, beaconRpcURL)

	// parentRpcURL := os.Getenv("ETHEREUM_RPC_URL")
	// // txHash := "0x4e2c7ff3b04c8115834b6edc8d746ffb519d747461e2e0bdd89d102622ec6a5e"
	// txHash := "0x837aae2c9ee352bfdf1ebd74f3f5042e62ef78a3a81d8cf2d6106b0dd7d485dd"
	// txHash := "0xcaf788c60948076d6839a9c40524d1eba8a0b6eb4bf62f732d8c98272b7f430d"

	// msg, err := batchhandler.StartBatchHandler(ctx, parentRpcURL, txHash, arbChainId, beaconRpcURL)

	// if err != nil {
	// 	fmt.Println("err decompressing")
	// }

	// if err != nil {
	// 	fmt.Println(err)
	// }

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

	// latest_state := arbClient.GetLatestState(ctx)
	// latest_state = arbClient.GetStateAt(ctx, 22341904, arbChainId)
	// latest_state := arbClient.GetL1DataAt(ctx, 3, arbChainId)

	// fmt.Println("block number: ", latest_state.L2BlockNumber)
	// fmt.Println("block hash: ", latest_state.L2BlockHash.Hex())

	// state_at_prev := arbClient.GetFullDataAt(ctx, latest_state.L2BlockNumber-4, arbChainId)
	// state_at_curr := arbClient.GetFullDataAt(ctx, latest_state.L2BlockNumber-2, arbChainId)

	// 3 expected_block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xb9dc275d50509340a54e252ae6430d3053b4da5ac69c814c183e18f36810b7f0"))
	// 4 expected_block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xaa6549925f0a680e76a17c241377e8307accea6c975d5aef968a260062a5a255"))
	//expected_block, err := arbClient.GetBlockByHash(ctx, state_at_curr.L2BlockHash)
	// expected_block, err := arbClient.GetBlockByNumber(ctx, big.NewInt(2))
	// if err != nil {
	// 	log.Fatalf("Failed to get L2 block by hash: %v", err)
	// }
	// // block, err := arbClient.GetBlockByHash(ctx, state_at_prev.L2BlockHash)
	// block, err := arbClient.GetBlockByNumber(ctx, big.NewInt(1))
	// if err != nil {
	// 	log.Fatalf("Failed to get L2 block by hash: %v", err)
	// }

	// arbClient2, err := NewArbitrumClient("https://arb-mainnet.g.alchemy.com/v2/mv49jnDzg_B9G-JLBAPNs-MwW4nd3Pk3")
	// if err != nil {
	// 	log.Fatalf("Failed to init Arbitrum client: %v", err)
	// }
	// msg := &latest_state.Message
	// fmt.Println("msg.Kind: ", msg.Header.Kind)
	// replay_message(ctx, ethClient.provider, arbClient, block.Header(), msg, expected_block.Header(), arbChainId)

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

	// fmt.Println(block.Header().Root.Hex())
	// fmt.Println(expected_block.Header().Root.Hex())

}

func test(arbClient *ArbitrumClient, state MessageTrackingL2Data, ctx context.Context, beaconRpcURL string, ethClientUrl string, arbChainId uint64, ethClient *EthereumClient) {
	// index := state.L2BlockNumber - 1

	fmt.Println("state.L2BlockNumber", state.L2BlockNumber)

	for index := uint64(2); index < state.L2BlockNumber; index++ {

		fmt.Println("index", index)

		prevBlock, err := arbClient.GetBlockByNumber(ctx, big.NewInt(int64(index-1)))
		if err != nil {
			log.Fatalf("Failed to get block: %v", err)
		}

		currBlock, err := arbClient.GetBlockByNumber(ctx, big.NewInt(int64(index)))
		if err != nil {
			log.Fatalf("Failed to get block: %v", err)
		}

		fmt.Println("prevBlock", prevBlock.Header().Hash().Hex(), prevBlock.Header().Number)
		fmt.Println("prevBlock root", prevBlock.Header().Root.Hex())
		fmt.Println("currBlock", currBlock.Header().Hash().Hex(), currBlock.Header().Number)
		fmt.Println("currBlock root", currBlock.Header().Root.Hex())
		for i := 0; i < prevBlock.Transactions().Len(); i++ {
			tx := prevBlock.Transactions()[i]
			fmt.Println("P tx", tx.Hash().Hex())
		}

		for i := 0; i < currBlock.Transactions().Len(); i++ {
			tx := currBlock.Transactions()[i]
			fmt.Println("C tx", tx.Hash().Hex())
		}

		prevTrackingL1Data := arbClient.GetL1DataAt(ctx, index, arbChainId)
		currTrackingL1Data := arbClient.GetL1DataAt(ctx, index+1, arbChainId)

		fmt.Println("prevTrackingL1Data", prevTrackingL1Data.Message.Header.Kind)
		fmt.Println("currTrackingL1Data", currTrackingL1Data.Message.Header.Kind)

		if currTrackingL1Data.Message.Header.Kind == arbostypes.L1MessageType_L2Message {
			currTxes, err := arbos.ParseL2Transactions(&currTrackingL1Data.Message, big.NewInt(int64(arbChainId)))

			if err != nil {
				fmt.Println("error", err)
			}

			for i := 0; i < len(currTxes); i++ {
				fmt.Println("currTx", currTxes[i].Hash().Hex())
			}

			fmt.Println("currBlock.Header().Root", currBlock.Header().Root.Hex())

			consensusOracleResult, err := ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)

			fmt.Println("consensusOracleResult", consensusOracleResult)
			fmt.Println("error", err)

			replay_message(ctx, ethClient.provider, arbClient, prevBlock.Header(), &currTrackingL1Data.Message, currBlock.Header(), arbChainId)
		} else {
			prevPrevBlock, err := arbClient.GetBlockByNumber(ctx, big.NewInt(int64(index-2)))
			if err != nil {
				log.Fatalf("Failed to get block: %v", err)
			}
			prevPrevTrackingL1Data := arbClient.GetL1DataAt(ctx, index-1, arbChainId)

			consensusOracleResult, err := ExecuteConsensusOracle(ctx, prevPrevTrackingL1Data, prevTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)
			if !consensusOracleResult || err != nil {
				fmt.Println("consensusOracleResult", consensusOracleResult)
				fmt.Println("error", err)
			}
			consensusOracleResult, err = ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)
			if !consensusOracleResult || err != nil {
				fmt.Println("consensusOracleResult", consensusOracleResult)
				fmt.Println("error", err)
			}

			replay_message(ctx, ethClient.provider, arbClient, prevPrevBlock.Header(), &prevTrackingL1Data.Message, currBlock.Header(), arbChainId, &currTrackingL1Data.Message)
		}
	}
}

func tournament(ctx context.Context, arbClients []*ArbitrumClient, ethClient *EthereumClient, ethClientUrl string, arbChainId uint64, beaconRpcURL string) {
	sizes := make(map[*ArbitrumClient]MessageTrackingL2Data)

	for i := 0; i < len(arbClients); i++ {
		state, err := arbClients[i].GetLatestState(ctx, arbChainId)
		sizes[arbClients[i]] = *state
		if err != nil {
			log.Fatalf("Failed to get latest state: %v", err)
		}
	}

	test(arbClients[0], sizes[arbClients[0]], ctx, beaconRpcURL, ethClientUrl, arbChainId, ethClient)

	// S := make(map[*ArbitrumClient]bool)
	// S[arbClients[0]] = true

	// largest := arbClients[0]

	// for i := 1; i < len(arbClients); i++ {
	// 	participant := arbClients[i]

	// 	for {

	// 		// 0 (00): largest loses, participant loses
	// 		// 1 (01): largest loses, participant wins
	// 		// 2 (10): largest wins, participant loses
	// 		// 3 (11): largest wins, participant wins
	// 		result := challenge(largest, sizes[largest], participant, sizes[participant], ctx)

	// 		// Step 15-16: If "nested MMRs" (participant wins), add to survivors
	// 		if result == 3 {
	// 			S[participant] = true
	// 			break
	// 		} else if result == 0 || result == 1 {
	// 			delete(S, largest)

	// 			var newLargest *ArbitrumClient
	// 			var maxSize uint64
	// 			for survivor := range S {
	// 				if sizes[survivor].L2BlockNumber > maxSize {
	// 					maxSize = sizes[survivor].L2BlockNumber
	// 					newLargest = survivor
	// 				}
	// 			}
	// 			largest = newLargest

	// 			if largest != nil {
	// 				continue
	// 			} else {
	// 				break
	// 			}
	// 		}

	// 		if len(S) == 0 {
	// 			S[participant] = true
	// 			largest = participant
	// 		}
	// 		break
	// 	}

	// 	fmt.Println(S)
	// }
}
