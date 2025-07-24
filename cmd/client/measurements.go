package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

type MeasurementConfig struct {
	NumIterations  int
	OutputDir      string
	MeasureNetwork bool
	MeasureSystem  bool
}

type MeasurementResult struct {
	NumProvers          int
	BlockNumber         uint64
	Iteration           int
	SyncTime            time.Duration
	ConsensusOracleTime time.Duration
	ExecutionOracleTime time.Duration
	MemoryUsage         uint64
	CPUUsage            float64
	NetworkBytesIn      uint64
	NetworkBytesOut     uint64
	Timestamp           time.Time
}

type MeasurementRunner struct {
	config       *MeasurementConfig
	results      []MeasurementResult
	arbClient    *ArbitrumClient
	ctx          context.Context
	beaconRpcURL string
	ethRpcURL    string
	arbChainId   uint64
}

func NewMeasurementRunner(config *MeasurementConfig, arbClient *ArbitrumClient, ctx context.Context, beaconRpcURL, ethRpcURL string, arbChainId uint64) *MeasurementRunner {
	return &MeasurementRunner{
		config:       config,
		results:      make([]MeasurementResult, 0),
		arbClient:    arbClient,
		ctx:          ctx,
		beaconRpcURL: beaconRpcURL,
		ethRpcURL:    ethRpcURL,
		arbChainId:   arbChainId,
	}
}

func (mr *MeasurementRunner) RunTournamentMeasurements(arbClients []*ArbitrumClient) error {
	fmt.Printf("Starting Tournament measurements: %d iterations\n", mr.config.NumIterations)

	if err := os.MkdirAll(mr.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	genesisBlock, err := mr.arbClient.GetBlockByNumber(mr.ctx, big.NewInt(0))
	if err != nil {
		return fmt.Errorf("failed to get genesis block: %v", err)
	}

	blockStart := uint64(100)
	blockEnd := uint64(200)
	blockStep := uint64(100)

	for provers := 0; provers < len(arbClients); provers++ {
		for iteration := 0; iteration < mr.config.NumIterations; iteration++ {
			for blockNumber := blockStart; blockNumber <= blockEnd; blockNumber += blockStep {
				currentIteration := uint64(blockNumber/blockStep) + (uint64(iteration) * uint64(blockEnd/blockStep)) + (uint64(provers) * uint64(mr.config.NumIterations))

				fmt.Printf("Tournament iteration %d/%d\n", currentIteration, uint64(mr.config.NumIterations)*uint64(len(arbClients))*((blockEnd-blockStart+blockStep)/blockStep))
				var inStart, outStart uint64
				if mr.config.MeasureNetwork {
					inStart, outStart, _ = mr.getNetworkBytes()
				}

				startTime := time.Now()

				Tournament(mr.ctx, *genesisBlock.Header(), arbClients[:provers+1], mr.ethRpcURL, mr.arbChainId, mr.beaconRpcURL, blockNumber)

				result := MeasurementResult{
					NumProvers:  provers,
					BlockNumber: blockNumber,
					Iteration:   iteration,
					SyncTime:    time.Since(startTime),
					Timestamp:   time.Now(),
				}

				if mr.config.MeasureSystem {
					mr.addSystemMeasurements(&result)
				}

				if mr.config.MeasureNetwork {
					inBytes, outBytes, err := mr.getNetworkBytes()
					if err != nil {
						log.Printf("Failed to get network bytes: %v", err)
					}
					result.NetworkBytesIn = inBytes - inStart
					result.NetworkBytesOut = outBytes - outStart
				}

				mr.results = append(mr.results, result)

				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	return mr.saveResults("tournament_measurements.csv")
}

func (mr *MeasurementRunner) addSystemMeasurements(result *MeasurementResult) {
	if vmstat, err := mem.VirtualMemory(); err == nil {
		result.MemoryUsage = vmstat.Used
	}

	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}
}

func (mr *MeasurementRunner) getNetworkBytes() (inBytes, outBytes uint64, err error) {
	counters, err := net.IOCounters(true)
	if err != nil {
		return 0, 0, err
	}

	var totalIn, totalOut uint64
	for _, c := range counters {
		totalIn += c.BytesRecv
		totalOut += c.BytesSent
	}
	return totalIn, totalOut, nil
}

func (mr *MeasurementRunner) saveResults(filename string) error {
	filepath := fmt.Sprintf("%s/%s", mr.config.OutputDir, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filepath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"num_provers", "block_number", "iteration", "sync_time_ms", "consensus_oracle_time_ms",
		"execution_oracle_time_ms", "memory_bytes", "cpu_percent",
		"network_bytes_in", "network_bytes_out", "timestamp",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	for _, result := range mr.results {
		row := []string{
			strconv.Itoa(result.NumProvers),
			strconv.FormatUint(result.BlockNumber, 10),
			strconv.Itoa(result.Iteration),
			strconv.FormatInt(int64(result.SyncTime.Milliseconds()), 10),
			strconv.FormatInt(int64(result.ConsensusOracleTime.Milliseconds()), 10),
			strconv.FormatInt(int64(result.ExecutionOracleTime.Milliseconds()), 10),
			strconv.FormatUint(result.MemoryUsage, 10),
			strconv.FormatFloat(result.CPUUsage, 'f', 2, 64),
			strconv.FormatUint(result.NetworkBytesIn, 10),
			strconv.FormatUint(result.NetworkBytesOut, 10),
			result.Timestamp.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %v", err)
		}
	}

	fmt.Printf("Results saved to %s\n", filepath)
	return nil
}

func (mr *MeasurementRunner) RunConsensusOracleMeasurements() error {
	fmt.Printf("Starting consensus oracle measurements: %d iterations\n", mr.config.NumIterations)

	if err := os.MkdirAll(mr.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	testBlock := uint64(1350)

	for iteration := 1; iteration <= mr.config.NumIterations; iteration++ {
		fmt.Printf("Consensus oracle iteration %d/%d\n", iteration, mr.config.NumIterations)

		result := MeasurementResult{
			BlockNumber: testBlock,
			Iteration:   iteration,
			Timestamp:   time.Now(),
		}

		prevTrackingL1Data := mr.arbClient.GetL1DataAt(mr.ctx, testBlock, mr.arbChainId)
		currTrackingL1Data := mr.arbClient.GetL1DataAt(mr.ctx, testBlock+1, mr.arbChainId)

		var inStart, outStart uint64
		if mr.config.MeasureNetwork {
			inStart, outStart, _ = mr.getNetworkBytes()
		}

		start := time.Now()
		_, err := ExecuteConsensusOracle(mr.ctx, prevTrackingL1Data, currTrackingL1Data, mr.ethRpcURL, mr.arbChainId, mr.beaconRpcURL)
		result.ConsensusOracleTime = time.Since(start)

		if err != nil {
			log.Printf("Consensus oracle error: %v", err)
		}

		if mr.config.MeasureSystem {
			mr.addSystemMeasurements(&result)
		}

		if mr.config.MeasureNetwork {
			inBytes, outBytes, err := mr.getNetworkBytes()
			if err != nil {
				log.Printf("Failed to get network bytes: %v", err)
			}
			result.NetworkBytesIn = inBytes - inStart
			result.NetworkBytesOut = outBytes - outStart
		}

		mr.results = append(mr.results, result)

		time.Sleep(100 * time.Millisecond)
	}

	return mr.saveResults("consensus_oracle_measurements.csv")
}

func (mr *MeasurementRunner) RunExecutionOracleMeasurements() error {
	fmt.Printf("Starting execution oracle measurements: %d iterations\n", mr.config.NumIterations)

	if err := os.MkdirAll(mr.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	testBlock := uint64(1350)

	for iteration := 1; iteration <= mr.config.NumIterations; iteration++ {
		fmt.Printf("Execution oracle iteration %d/%d\n", iteration, mr.config.NumIterations)

		result := MeasurementResult{
			BlockNumber: testBlock,
			Iteration:   iteration,
			Timestamp:   time.Now(),
		}

		prevBlock, err := mr.arbClient.GetBlockByNumber(mr.ctx, big.NewInt(int64(testBlock-1)))
		if err != nil {
			log.Printf("Failed to get prev block: %v", err)
			continue
		}

		currBlock, err := mr.arbClient.GetBlockByNumber(mr.ctx, big.NewInt(int64(testBlock)))
		if err != nil {
			log.Printf("Failed to get curr block: %v", err)
			continue
		}

		currTrackingL1Data := mr.arbClient.GetL1DataAt(mr.ctx, testBlock+1, mr.arbChainId)

		var inStart, outStart uint64
		if mr.config.MeasureNetwork {
			inStart, outStart, _ = mr.getNetworkBytes()
		}

		start := time.Now()
		executionResult := ExecuteExecutionOracle(mr.ctx, mr.arbClient, prevBlock.Header(), &currTrackingL1Data.Message, currBlock.Header(), mr.arbChainId)
		result.ExecutionOracleTime = time.Since(start)

		if !executionResult {
			log.Printf("Execution oracle failed for iteration %d", iteration)
		}

		if mr.config.MeasureSystem {
			mr.addSystemMeasurements(&result)
		}

		if mr.config.MeasureNetwork {
			inBytes, outBytes, err := mr.getNetworkBytes()
			if err != nil {
				log.Printf("Failed to get network bytes: %v", err)
			}
			result.NetworkBytesIn = inBytes - inStart
			result.NetworkBytesOut = outBytes - outStart
		}

		mr.results = append(mr.results, result)

		time.Sleep(100 * time.Millisecond)
	}

	return mr.saveResults("execution_oracle_measurements.csv")
}

func (mr *MeasurementRunner) PrintSummary() {
	if len(mr.results) == 0 {
		fmt.Println("No results to summarize")
		return
	}

	fmt.Println("\n=== Measurement Summary ===")
	fmt.Printf("Total measurements: %d\n", len(mr.results))

	// Calculate statistics for each metric
	if len(mr.results) > 0 {
		// Consensus oracle times
		var consensusTimes []float64
		for _, result := range mr.results {
			if result.ConsensusOracleTime > 0 {
				consensusTimes = append(consensusTimes, float64(result.ConsensusOracleTime.Milliseconds()))
			}
		}
		if len(consensusTimes) > 0 {
			avg, min, max := calculateStats(consensusTimes)
			fmt.Printf("Consensus Oracle (ms): avg=%.2f, min=%.2f, max=%.2f\n", avg, min, max)
		}

		// Execution oracle times
		var executionTimes []float64
		for _, result := range mr.results {
			if result.ExecutionOracleTime > 0 {
				executionTimes = append(executionTimes, float64(result.ExecutionOracleTime.Milliseconds()))
			}
		}
		if len(executionTimes) > 0 {
			avg, min, max := calculateStats(executionTimes)
			fmt.Printf("Execution Oracle (ms): avg=%.2f, min=%.2f, max=%.2f\n", avg, min, max)
		}

		// Sync times
		var syncTimes []float64
		for _, result := range mr.results {
			if result.SyncTime > 0 {
				syncTimes = append(syncTimes, float64(result.SyncTime.Milliseconds()))
			}
		}
		if len(syncTimes) > 0 {
			avg, min, max := calculateStats(syncTimes)
			fmt.Printf("Sync Time (ms): avg=%.2f, min=%.2f, max=%.2f\n", avg, min, max)
		}

		// Memory usage
		var memoryUsage []float64
		for _, result := range mr.results {
			if result.MemoryUsage > 0 {
				memoryUsage = append(memoryUsage, float64(result.MemoryUsage))
			}
		}
		if len(memoryUsage) > 0 {
			avg, min, max := calculateStats(memoryUsage)
			fmt.Printf("Memory Usage (bytes): avg=%.0f, min=%.0f, max=%.0f\n", avg, min, max)
		}

		// CPU usage
		var cpuUsage []float64
		for _, result := range mr.results {
			if result.CPUUsage > 0 {
				cpuUsage = append(cpuUsage, result.CPUUsage)
			}
		}
		if len(cpuUsage) > 0 {
			avg, min, max := calculateStats(cpuUsage)
			fmt.Printf("CPU Usage (%%): avg=%.2f, min=%.2f, max=%.2f\n", avg, min, max)
		}
	}

	fmt.Println("===========================")
}

func calculateStats(values []float64) (avg, min, max float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}

	min = values[0]
	max = values[0]
	sum := 0.0

	for _, v := range values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	avg = sum / float64(len(values))
	return avg, min, max
}
