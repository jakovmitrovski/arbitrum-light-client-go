package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/joho/godotenv"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
)

type ChallengeResult int

const (
	BothLose ChallengeResult = iota
	LargestLosesParticipantWins
	LargestWinsParticipantLoses
	BothWin
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	ctx := context.Background()

	ethRpcURL := os.Getenv("ETHEREUM_RPC_URL")
	proverRpcURLs := strings.Split(os.Getenv("PROVERS"), ",")
	rollupCoreAddr := common.HexToAddress(os.Getenv("ROLLUP_CORE_ADDRESS"))

	// accountAddress := common.HexToAddress(os.Getenv("ACCOUNT_ADDRESS"))

	ethClient, err := NewEthereumClient(ethRpcURL, rollupCoreAddr)
	if err != nil {
		log.Fatalf("Failed to init Ethereum client: %v", err)
	}

	arbClients := make([]*ArbitrumClient, len(proverRpcURLs))
	for i, proverRpcURL := range proverRpcURLs {
		fmt.Printf("Initializing prover %d: %s\n", i, proverRpcURL)
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

	// // // 3. Get AssertionConfirmed log
	// confirmedLog, err := ethClient.GetAssertionConfirmedLog(ctx)
	// if err != nil {
	// 	log.Fatalf("Failed to get AssertionConfirmed: %v", err)
	// }
	// fmt.Printf("AssertionConfirmed: %+v\n", confirmedLog)

	// // // 4. Get AssertionCreated log
	// createdLog, err := ethClient.GetAssertionCreatedLog(ctx)
	// if err != nil {
	// 	log.Fatalf("Failed to get AssertionCreated: %v", err)
	// }
	// fmt.Printf("AssertionCreated: %+v\n", createdLog)

	// fmt.Printf("AVM State root: 0x%x\n", createdLog.Assertion.AfterState.EndHistoryRoot)

	// // // 5. Fetch and verify block from L2
	// //block, err := arbClients[0].GetBlockByHash(ctx, confirmedLog.BlockHash)
	// // 2 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xe1ec95df182d2d335e72aae5dcac142210b30c24dbff2a695deb8c0bbf991533"))
	// // 3 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0x66aa454cd2c6e38af14d44a1697a0c010da712bae577fdaaf299bd208532e134"))
	// // 4 block, err := arbClient.GetBlockByHash(ctx, common.HexToHash("0xb9dc275d50509340a54e252ae6430d3053b4da5ac69c814c183e18f36810b7f0"))
	// // if err != nil {
	// // 	log.Fatalf("Failed to get L2 block by hash: %v", err)
	// // }

	// // discard all provers if they disagree on neon-genesis:
	// realArbClients := make([]*ArbitrumClient, 0)
	// for i := 0; i < len(arbClients); i++ {
	// 	block, err := arbClients[i].GetBlockByHash(ctx, confirmedLog.BlockHash)
	// 	if err != nil {
	// 		log.Fatalf("Failed to get L2 block by hash: %v", err)
	// 	}
	// 	verified := arbClients[0].VerifyBlockHash(block.Header(), confirmedLog.BlockHash)
	// 	fmt.Printf("Block number %d hash verified: %v\n", block.Header().Number.Uint64(), verified)
	// 	if verified {
	// 		realArbClients = append(realArbClients, arbClients[i])
	// 	}
	// }

	// for i := 0; i < len(realArbClients); i++ {
	// 	fmt.Printf("Agrees on genesis client %d: %p\n", i, realArbClients[i])
	// }

	// 6. verify the block hash...

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

	// test(arbClients[0], ctx, beaconRpcURL, ethRpcURL, arbChainId, ethClient)

	Tournament(ctx, *genesisBlock.Header(), arbClients, ethRpcURL, arbChainId, beaconRpcURL)

}

func test(arbClient *ArbitrumClient, ctx context.Context, beaconRpcURL string, ethClientUrl string, arbChainId uint64, ethClient *EthereumClient) {
	// index := state.L2BlockNumber - 1

	// state, err := arbClient.GetLatestState(ctx, arbChainId)
	// if err != nil {
	// 	log.Fatalf("Failed to get latest state: %v", err)
	// }

	for index := uint64(12); index < 16; index++ {
		fmt.Println("index", index)
		if index == 11 {
			continue
		}

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

		if prevTrackingL1Data.Message.Header.Kind == arbostypes.L1MessageType_Initialize {
			fmt.Println("init message encountered")
			executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevBlock.Header(), &prevTrackingL1Data.Message, currBlock.Header(), arbChainId, &currTrackingL1Data.Message)
			fmt.Println("executionOracleResult", executionOracleResult)
		} else {
			fmt.Println("currBlock.Header().Root", currBlock.Header().Root.Hex())

			consensusOracleResult, err := ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)

			if !consensusOracleResult || err != nil {
				fmt.Println("consensusOracleResult", consensusOracleResult)
				fmt.Println("transaction", currTrackingL1Data.L1TxHash.Hex())
				fmt.Println("error", err)
			}

			fmt.Printf("=== About to execute replay_message for L2 message ===\n")
			executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevBlock.Header(), &currTrackingL1Data.Message, currBlock.Header(), arbChainId)
			fmt.Println("executionOracleResult", executionOracleResult)
			if !executionOracleResult {
				fmt.Println("executionOracleResult", executionOracleResult)
				break
			}
		}
		// } else {
		// 	prevPrevBlock, err := arbClient.GetBlockByNumber(ctx, big.NewInt(int64(index-2)))
		// 	if err != nil {
		// 		log.Fatalf("Failed to get block: %v", err)
		// 	}
		// 	prevPrevTrackingL1Data := arbClient.GetL1DataAt(ctx, index-1, arbChainId)

		// 	consensusOracleResult1, err := ExecuteConsensusOracle(ctx, prevPrevTrackingL1Data, prevTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)
		// 	if err != nil {
		// 		fmt.Println("error", err)
		// 	}
		// 	consensusOracleResult2, err := ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)
		// 	if err != nil {
		// 		fmt.Println("error", err)
		// 	}
		// 	executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevPrevBlock.Header(), &prevTrackingL1Data.Message, prevBlock.Header(), arbChainId, &currTrackingL1Data.Message)

		// 	fmt.Println("consensusOracleResult1", consensusOracleResult1)
		// 	fmt.Println("consensusOracleResult2", consensusOracleResult2)
		// 	fmt.Println("executionOracleResult", executionOracleResult)
		// }

	}
}

func TestOracles(arbClient *ArbitrumClient, index uint64, ctx context.Context, beaconRpcURL string, ethClientUrl string, arbChainId uint64) bool {
	prevBlock, err := arbClient.GetBlockByNumber(ctx, big.NewInt(int64(index-1)))
	if err != nil {
		log.Fatalf("Failed to get block: %v", err)
	}
	currBlock, err := arbClient.GetBlockByNumber(ctx, big.NewInt(int64(index)))
	if err != nil {
		log.Fatalf("Failed to get block: %v", err)
	}

	prevTrackingL1Data := arbClient.GetL1DataAt(ctx, index, arbChainId)
	currTrackingL1Data := arbClient.GetL1DataAt(ctx, index+1, arbChainId)

	if prevTrackingL1Data.Message.Header.Kind == arbostypes.L1MessageType_Initialize {
		fmt.Println("init message encountered")
		executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevBlock.Header(), &prevTrackingL1Data.Message, currBlock.Header(), arbChainId, &currTrackingL1Data.Message)
		fmt.Println("executionOracleResult", executionOracleResult)
		return executionOracleResult
	} else {
		fmt.Println("currBlock.Header().Root", currBlock.Header().Root.Hex())

		consensusOracleResult, err := ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)

		if !consensusOracleResult || err != nil {
			fmt.Println("consensusOracleResult", consensusOracleResult)
			fmt.Println("error", err)
			return false
		}

		executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevBlock.Header(), &currTrackingL1Data.Message, currBlock.Header(), arbChainId)
		fmt.Println("executionOracleResult", executionOracleResult)

		return consensusOracleResult && executionOracleResult
	}
}

func Tournament(ctx context.Context, neonGenesisBlock types.Header, arbClients []*ArbitrumClient, ethClientUrl string, arbChainId uint64, beaconRpcURL string) {
	sizes := make(map[*ArbitrumClient]MessageTrackingL2Data)

	for i := 0; i < len(arbClients); i++ {
		state, err := arbClients[i].GetLatestState(ctx, arbChainId)
		sizes[arbClients[i]] = *state
		if err != nil {
			log.Fatalf("Failed to get latest state: %v", err)
		}
	}

	sort.Slice(arbClients, func(i, j int) bool {
		return sizes[arbClients[i]].L2BlockNumber > sizes[arbClients[j]].L2BlockNumber
	})

	S := make(map[*ArbitrumClient]bool)
	S[arbClients[0]] = true

	largest := arbClients[0]

	for i := 1; i < len(arbClients); i++ {
		participant := arbClients[i]

		fmt.Println("S before", len(S))

		for {
			result := Challenge(neonGenesisBlock, largest, sizes[largest], participant, sizes[participant], ctx, ethClientUrl, arbChainId, beaconRpcURL)

			fmt.Println("result", result)

			if result == BothWin {
				S[participant] = true
				break
			} else if result == BothLose || result == LargestLosesParticipantWins {
				delete(S, largest)

				var newLargest *ArbitrumClient
				var maxSize uint64
				for survivor := range S {
					if sizes[survivor].L2BlockNumber > maxSize {
						maxSize = sizes[survivor].L2BlockNumber
						newLargest = survivor
					}
				}
				largest = newLargest

				if largest != nil {
					continue
				} else {
					break
				}
			} else {
				break
			}
		}

		if len(S) == 0 {
			S[participant] = true
			largest = participant
		}
	}

	fmt.Println("Final survivors:", len(S))
	for survivor := range S {
		fmt.Printf("Final survivor: %p, Size: %d, Hash: %s\n", survivor, sizes[survivor].L2BlockNumber, sizes[survivor].L2BlockHash.Hex())
	}
}

func Challenge(neonGenesisBlock types.Header, largest *ArbitrumClient, largestState MessageTrackingL2Data, participant *ArbitrumClient, participantState MessageTrackingL2Data, ctx context.Context, ethClientUrl string, arbChainId uint64, beaconRpcURL string) ChallengeResult {
	fmt.Printf("=== Challenge between largest (size: %d) and participant (size: %d) ===\n",
		largestState.L2BlockNumber, participantState.L2BlockNumber)

	largestStateLower, err := largest.GetBlockByNumber(ctx, big.NewInt(int64(participantState.L2BlockNumber)))
	if err != nil {
		fmt.Printf("Largest client failed to get block %d: %v\n", participantState.L2BlockNumber, err)
		return LargestLosesParticipantWins
	}

	if largestStateLower.Header().Hash() == participantState.L2BlockHash {
		fmt.Println("Both clients agree on common prefix")

		// If they agree on common prefix, participant is not losing
		// Now test the remaining blocks of the larger client
		fmt.Printf("Testing remaining blocks %d to %d\n", participantState.L2BlockNumber+1, largestState.L2BlockNumber)

		for index := participantState.L2BlockNumber + 1; index < min(largestState.L2BlockNumber, participantState.L2BlockNumber+10); index++ {
			fmt.Printf("Testing block %d\n", index)

			result := TestOracles(largest, index, ctx, beaconRpcURL, ethClientUrl, arbChainId)
			if !result {
				return LargestLosesParticipantWins
			}
		}

		fmt.Println("Largest client passed all tests")
		return BothWin

	} else {
		fmt.Printf("Disagreement found! largest=%s, participant=%s\n",
			largestState.L2BlockHash.Hex(), participantState.L2BlockHash.Hex())

		if largestStateLower.Header().Number.Uint64() != participantState.L2BlockNumber {
			return LargestLosesParticipantWins
		}

		// Perform bisection to find a point of disagreement
		return PerformBisection(neonGenesisBlock, largest, participant, participantState, ctx, ethClientUrl, arbChainId, beaconRpcURL)
	}
}

func PerformBisection(neonGenesisBlock types.Header, largest *ArbitrumClient, participant *ArbitrumClient, participantState MessageTrackingL2Data, ctx context.Context, ethClientUrl string, arbChainId uint64, beaconRpcURL string) ChallengeResult {
	left := neonGenesisBlock.Number.Uint64()
	right := participantState.L2BlockNumber

	for left < right-1 {
		mid := (left + right) / 2
		largestState, err := largest.GetBlockByNumber(ctx, big.NewInt(int64(mid)))
		if err != nil {
			fmt.Printf("Largest client failed to get block %d: %v\n", mid, err)
			return LargestLosesParticipantWins
		}
		participantState, err := participant.GetBlockByNumber(ctx, big.NewInt(int64(mid)))
		if err != nil {
			fmt.Printf("Participant client failed to get block %d: %v\n", mid, err)
			return LargestWinsParticipantLoses
		}
		if largestState.Header().Hash() == participantState.Header().Hash() {
			fmt.Println("updating left to", mid)
			left = mid
		} else {
			fmt.Println("updating right to", mid)
			right = mid
		}
	}

	// Now test the disagreement point
	fmt.Printf("Testing disagreement at block %d\n", right)

	largestResult := TestOracles(largest, right, ctx, beaconRpcURL, ethClientUrl, arbChainId)
	participantResult := TestOracles(participant, right, ctx, beaconRpcURL, ethClientUrl, arbChainId)

	if largestResult && !participantResult {
		fmt.Println("Largest wins, participant loses")
		return LargestWinsParticipantLoses
	} else if !largestResult && participantResult {
		fmt.Println("Largest loses, participant wins")
		return LargestLosesParticipantWins
	} else if !largestResult && !participantResult {
		fmt.Println("Both lose")
		return BothLose
	} else {
		fmt.Println("Both win (shouldn't happen with disagreement)")
		return BothWin
	}
}
