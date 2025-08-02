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

	var genesisBlock *types.Block

	// discard all provers if they disagree on neon-genesis:
	realArbClients := make([]*ArbitrumClient, 0)
	for i := 0; i < len(arbClients); i++ {
		block, err := arbClients[i].GetBlockByHash(ctx, confirmedLog.BlockHash)
		if err != nil {
			log.Fatalf("Failed to get L2 block by hash: %v", err)
		}
		verified := arbClients[i].VerifyBlockHash(block.Header(), confirmedLog.BlockHash)
		fmt.Printf("Block number %d hash verified: %v\n", block.Header().Number.Uint64(), verified)
		if verified {
			realArbClients = append(realArbClients, arbClients[i])
		}
		if genesisBlock == nil && verified {
			genesisBlock = block
		}
	}

	for i := 0; i < len(realArbClients); i++ {
		fmt.Printf("Agrees on genesis client %d: %p\n", i, realArbClients[i])
	}

	beaconRpcURL := os.Getenv("ETHEREUM_BEACON_RPC_URL")
	arbChainId, err := strconv.ParseUint(os.Getenv("ARBITRUM_ONE_CHAIN_ID"), 10, 64)

	if err != nil {
		log.Fatalf("Error parsing ARBITRUM_CHAIN_ID: %v", err)
	}

	// RunMeasurements(ctx, arbClients, ethRpcURL, arbChainId, beaconRpcURL)

	Tournament(ctx, *genesisBlock.Header(), arbClients, ethRpcURL, arbChainId, beaconRpcURL, 0)

}

func RunMeasurements(ctx context.Context, arbClients []*ArbitrumClient, ethRpcURL string, arbChainId uint64, beaconRpcURL string) {
	// Example 1: Tournament measurements
	fmt.Println("\n1. Running Tournament measurements...")
	config1 := &MeasurementConfig{
		NumIterations:  5,
		OutputDir:      "./measurements/tournament",
		MeasureSystem:  true,
		MeasureNetwork: true,
	}

	runner1 := NewMeasurementRunner(config1, arbClients[0], ctx, beaconRpcURL, ethRpcURL, arbChainId)
	if err := runner1.RunTournamentMeasurements(arbClients); err != nil {
		log.Printf("Tournament measurements failed: %v", err)
	}
	runner1.PrintSummary()

	// Example 2: Consensus oracle measurements
	fmt.Println("\n2. Running Consensus Oracle measurements...")
	config2 := &MeasurementConfig{
		NumIterations:  10,
		OutputDir:      "./measurements/consensus",
		MeasureSystem:  true,
		MeasureNetwork: true,
	}

	runner2 := NewMeasurementRunner(config2, arbClients[0], ctx, beaconRpcURL, ethRpcURL, arbChainId)
	if err := runner2.RunConsensusOracleMeasurements(); err != nil {
		log.Printf("Consensus oracle measurements failed: %v", err)
	}
	runner2.PrintSummary()

	// Example 3: Execution oracle measurements
	fmt.Println("\n3. Running Execution Oracle measurements...")
	config3 := &MeasurementConfig{
		NumIterations:  10,
		OutputDir:      "./measurements/execution",
		MeasureSystem:  true,
		MeasureNetwork: true,
	}

	runner3 := NewMeasurementRunner(config3, arbClients[0], ctx, beaconRpcURL, ethRpcURL, arbChainId)
	if err := runner3.RunExecutionOracleMeasurements(); err != nil {
		log.Printf("Execution oracle measurements failed: %v", err)
	}
	runner3.PrintSummary()
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
		consensussOracleResult, err := ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)
		if err != nil || !consensussOracleResult {
			return false
		}

		executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevBlock.Header(), &prevTrackingL1Data.Message, currBlock.Header(), arbChainId, &currTrackingL1Data.Message)

		return executionOracleResult
	} else {
		consensusOracleResult, err := ExecuteConsensusOracle(ctx, prevTrackingL1Data, currTrackingL1Data, ethClientUrl, arbChainId, beaconRpcURL)

		if !consensusOracleResult || err != nil {
			return false
		}

		executionOracleResult := ExecuteExecutionOracle(ctx, arbClient, prevBlock.Header(), &currTrackingL1Data.Message, currBlock.Header(), arbChainId)

		return executionOracleResult
	}
}

func Tournament(ctx context.Context, neonGenesisBlock types.Header, arbClients []*ArbitrumClient, ethClientUrl string, arbChainId uint64, beaconRpcURL string, n uint64) {
	sizes := make(map[*ArbitrumClient]MessageTrackingL2Data)

	for i := 0; i < len(arbClients); i++ {
		var state *MessageTrackingL2Data
		var err error
		if n == 0 {
			state, err = arbClients[i].GetLatestState(ctx, arbChainId)
		} else {
			state, err = arbClients[i].GetStateAt(ctx, n, arbChainId)
		}
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

		for {
			result := Challenge(neonGenesisBlock, largest, sizes[largest], participant, sizes[participant], ctx, ethClientUrl, arbChainId, beaconRpcURL)

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
