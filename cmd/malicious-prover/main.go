package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

type MaliciousProver struct {
	realProverURL    string
	port             string
	corruptionCutoff uint64
	corruptionChance float64
	blockHashMapping map[string]string // corrupted hash -> original hash
	ethClient        *ethclient.Client
}

type BlockResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result"`
}

type StateResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result"`
}

var corruptibleFields = []string{
	"stateRoot",
	"transactionsRoot",
	"receiptsRoot",
	"mixHash",
	"gasLimit",
	"gasUsed",
	"timestamp",
	"extraData",
	"logsBloom",
}

var corruptedValues = map[string]string{
	"stateRoot":        "0x" + strings.Repeat("dead", 16),
	"transactionsRoot": "0x" + strings.Repeat("beef", 16),
	"receiptsRoot":     "0x" + strings.Repeat("cafe", 16),
	"mixHash":          "0x" + strings.Repeat("babe", 16),
	"gasLimit":         "0x" + strings.Repeat("abcd", 1),
	"gasUsed":          "0x" + strings.Repeat("abcd", 1),
	"timestamp":        "0x" + strings.Repeat("1234", 1),
	"extraData":        "0x" + strings.Repeat("9999", 1),
	"logsBloom":        "0x" + strings.Repeat("aaaa", 128),
}

func NewMaliciousProver(realProverURL, port string, corruptionCutoff uint64, corruptionChance float64) *MaliciousProver {
	// Create eth client for getting blocks
	ethClient, err := ethclient.Dial(realProverURL)
	if err != nil {
		log.Printf("Warning: Could not create eth client: %v", err)
		ethClient = nil
	}

	return &MaliciousProver{
		realProverURL:    realProverURL,
		port:             port,
		corruptionCutoff: corruptionCutoff,
		corruptionChance: corruptionChance,
		blockHashMapping: make(map[string]string),
		ethClient:        ethClient,
	}
}

func (mp *MaliciousProver) shouldCorrupt() bool {
	return rand.Float64() < mp.corruptionChance
}

func (mp *MaliciousProver) getRandomCorruptedField() string {
	return corruptibleFields[rand.Intn(len(corruptibleFields))]
}

func (mp *MaliciousProver) getBlockHash(blockNumber uint64) (string, error) {
	if mp.ethClient == nil {
		return "", fmt.Errorf("eth client not available")
	}

	block, err := mp.ethClient.BlockByNumber(context.Background(), big.NewInt(int64(blockNumber)))
	if err != nil {
		return "", err
	}

	return block.Hash().Hex(), nil
}

func (mp *MaliciousProver) computeBlockHash(blockData map[string]interface{}) (string, error) {
	// Parse hex strings to proper types
	parentHash := common.HexToHash(blockData["parentHash"].(string))
	uncleHash := common.HexToHash(blockData["sha3Uncles"].(string))
	coinbase := common.HexToAddress(blockData["miner"].(string))
	root := common.HexToHash(blockData["stateRoot"].(string))
	txHash := common.HexToHash(blockData["transactionsRoot"].(string))
	receiptHash := common.HexToHash(blockData["receiptsRoot"].(string))

	// Parse logsBloom
	logsBloomHex := blockData["logsBloom"].(string)
	logsBloomBytes := common.FromHex(logsBloomHex)
	var bloom types.Bloom
	copy(bloom[:], logsBloomBytes)

	// Parse numeric fields
	difficultyStr := strings.TrimPrefix(blockData["difficulty"].(string), "0x")
	difficulty := new(big.Int)
	difficulty.SetString(difficultyStr, 16)

	numberStr := strings.TrimPrefix(blockData["number"].(string), "0x")
	number := new(big.Int)
	number.SetString(numberStr, 16)

	gasLimitStr := strings.TrimPrefix(blockData["gasLimit"].(string), "0x")
	gasLimit := new(big.Int)
	gasLimit.SetString(gasLimitStr, 16)

	gasUsedStr := strings.TrimPrefix(blockData["gasUsed"].(string), "0x")
	gasUsed := new(big.Int)
	gasUsed.SetString(gasUsedStr, 16)

	timeStr := strings.TrimPrefix(blockData["timestamp"].(string), "0x")
	time := new(big.Int)
	time.SetString(timeStr, 16)

	extraData := common.FromHex(blockData["extraData"].(string))
	mixDigest := common.HexToHash(blockData["mixHash"].(string))

	nonceStr := strings.TrimPrefix(blockData["nonce"].(string), "0x")
	nonce := new(big.Int)
	nonce.SetString(nonceStr, 16)

	// Create header with all required fields
	header := &types.Header{
		ParentHash:  parentHash,
		UncleHash:   uncleHash,
		Coinbase:    coinbase,
		Root:        root,
		TxHash:      txHash,
		ReceiptHash: receiptHash,
		Bloom:       bloom,
		Difficulty:  difficulty,
		Number:      number,
		GasLimit:    gasLimit.Uint64(),
		GasUsed:     gasUsed.Uint64(),
		Time:        time.Uint64(),
		Extra:       extraData,
		MixDigest:   mixDigest,
		Nonce:       types.EncodeNonce(nonce.Uint64()),
	}

	// Handle baseFeePerGas if present (this is in the response)
	if baseFeeStr, ok := blockData["baseFeePerGas"].(string); ok && baseFeeStr != "" {
		baseFeeStr = strings.TrimPrefix(baseFeeStr, "0x")
		baseFee := new(big.Int)
		baseFee.SetString(baseFeeStr, 16)
		header.BaseFee = baseFee
	}

	// Compute the hash
	hash := header.Hash()
	return hash.Hex(), nil
}

func (mp *MaliciousProver) corruptL1Data(data map[string]interface{}) map[string]interface{} {
	corrupted := deepCopyMap(data)

	shouldCorrupt := mp.shouldCorrupt()

	// Single corruption decision
	if shouldCorrupt {
		// Randomly choose between corrupting message or txhash
		if rand.Float64() < 0.5 {
			// Corrupt the L2Msg within the Message
			if message, ok := corrupted["Message"].(map[string]interface{}); ok {
				if l2MsgHex, ok := message["l2Msg"].(string); ok {
					l2MsgBytes := common.FromHex(l2MsgHex)

					fmt.Println("l2MsgBytes", l2MsgBytes)

					// Corrupt some bytes in the middle
					if len(l2MsgBytes) > 10 {
						corruptionStart := 10
						for i := corruptionStart; i < corruptionStart+4 && i < len(l2MsgBytes); i++ {
							l2MsgBytes[i] = 0xDE // Corrupt with 0xDE
						}

						// Convert back to []interface{} for JSON
						corruptedL2Msg := make([]interface{}, len(l2MsgBytes))
						for i, b := range l2MsgBytes {
							corruptedL2Msg[i] = float64(b)
						}

						message["l2Msg"] = corruptedL2Msg
						fmt.Printf("  Corrupted L2Msg bytes at positions 10-13\n")
					}
				}
			}
		} else {
			// Corrupt L1 transaction hash
			corrupted["L1TxHash"] = "0x" + strings.Repeat("beef", 16)
			fmt.Printf("  Corrupted L1TxHash\n")
		}
	}

	return corrupted
}

func (mp *MaliciousProver) proxyRequest(w http.ResponseWriter, r *http.Request) {
	// Read the request body to check the method
	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		fmt.Printf("Error decoding request body: %v\n", err)
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusInternalServerError)
		return
	}

	method, _ := requestBody["method"].(string)

	// Handle debug_traceBlockByHash - map corrupted hash back to original
	if strings.Contains(method, "debug_traceBlockByHash") {
		params, ok := requestBody["params"].([]interface{})
		if ok && len(params) > 0 {
			requestedHash, ok := params[0].(string)
			if ok {
				// Check if this is a corrupted hash we need to map back
				if originalHash, exists := mp.blockHashMapping[requestedHash]; exists {
					fmt.Printf("Mapping corrupted hash %s back to original %s\n", requestedHash, originalHash)
					// Replace the hash in the request
					params[0] = originalHash
					requestBody["params"] = params
				} else {
					fmt.Printf("DEBUG: Hash %s not found in mapping, using as-is\n", requestedHash)
				}
			}
		}
	}

	// Re-encode the request body
	requestBytes, _ := json.Marshal(requestBody)

	// Forward the request to the real prover
	resp, err := http.Post(mp.realProverURL, "application/json", bytes.NewBuffer(requestBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("Error forwarding request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read the real response
	var realResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&realResponse); err != nil {
		http.Error(w, fmt.Sprintf("Error decoding response: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle lightclient_getL1DataAt with random corruption
	if strings.Contains(method, "lightclient_getL1DataAt") {
		// Extract the block number parameter
		params, ok := requestBody["params"].([]interface{})
		if ok && len(params) > 0 {
			blockNum, ok := params[0].(float64)
			if ok {
				if err == nil {
					fmt.Printf("Corrupting L1 data for block %f\n", blockNum)

					corruptedResponse := deepCopyMap(realResponse)
					if result, ok := corruptedResponse["result"].(map[string]interface{}); ok {
						corruptedResult := mp.corruptL1Data(result)
						corruptedResponse["result"] = corruptedResult
					}

					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(corruptedResponse)
					return
				}
			}
		}
	}

	// Add debug logging for lightclient_getLatestIndexL1
	if strings.Contains(method, "lightclient_getLatestIndexL1") {
		fmt.Printf("DEBUG: Forwarding lightclient_getLatestIndexL1 request\n")
	}

	// Corrupt eth_getBlockByNumber responses based on cutoff point
	if strings.Contains(method, "eth_getBlockByNumber") {
		// Extract the block number parameter
		params, ok := requestBody["params"].([]interface{})
		if ok && len(params) > 0 {
			blockNumStr, ok := params[0].(string)
			if ok {
				// Parse the block number (remove "0x" prefix if present)
				blockNumStr = strings.TrimPrefix(blockNumStr, "0x")
				blockNum, err := strconv.ParseUint(blockNumStr, 16, 64)
				if err == nil && blockNum >= mp.corruptionCutoff {
					fmt.Printf("Corrupting block data for block %d (cutoff is %d)\n", blockNum, mp.corruptionCutoff)

					// Get the original hash using eth client
					originalHash, err := mp.getBlockHash(blockNum)
					if err != nil {
						fmt.Printf("  Error getting original hash: %v\n", err)
						// Continue with real response if we can't get the hash
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(realResponse)
						return
					}
					fmt.Printf("originalHash: %s\n", originalHash)

					// Create a deep copy of the response to avoid modifying the original
					corruptedResponse := deepCopyMap(realResponse)

					// Randomly corrupt different fields
					if result, ok := corruptedResponse["result"].(map[string]interface{}); ok {
						// Decide how many fields to corrupt (1-3 fields)
						numFieldsToCorrupt := 1

						for i := 0; i < numFieldsToCorrupt; i++ {
							fieldToCorrupt := mp.getRandomCorruptedField()
							if corruptedValue, exists := corruptedValues[fieldToCorrupt]; exists {
								result[fieldToCorrupt] = corruptedValue
							}
						}

						// Generate a fake corrupted hash
						corruptedHash, err := mp.computeBlockHash(result)
						if err == nil {
							// Set the corrupted hash in the result
							// result["hash"] = corruptedHash
							fmt.Printf("corruptedHash: %s\n", corruptedHash)
							fmt.Printf("result[hash]: %s\n", result["hash"])

							// Record the mapping from corrupted hash to original hash
							mp.blockHashMapping[corruptedHash] = originalHash
							fmt.Printf("corruptedHash: %s\n", corruptedHash)
							fmt.Printf("  Hash mapping: %s -> %s\n", corruptedHash, originalHash)
						} else {
							fmt.Printf("  Error computing corrupted hash: %v\n", err)
							// Continue with real response if hash computation fails
							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(realResponse)
							return
						}
					}

					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(corruptedResponse)
					return
				}
			}
		}
	}

	// Return the real response for all other cases
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(realResponse)
}

func deepCopyMap(original map[string]interface{}) map[string]interface{} {
	copied := make(map[string]interface{})
	for key, value := range original {
		switch v := value.(type) {
		case map[string]interface{}:
			copied[key] = deepCopyMap(v)
		case []interface{}:
			copied[key] = deepCopySlice(v)
		default:
			copied[key] = value
		}
	}
	return copied
}

func deepCopySlice(original []interface{}) []interface{} {
	copied := make([]interface{}, len(original))
	for i, value := range original {
		switch v := value.(type) {
		case map[string]interface{}:
			copied[i] = deepCopyMap(v)
		case []interface{}:
			copied[i] = deepCopySlice(v)
		default:
			copied[i] = value
		}
	}
	return copied
}

func (mp *MaliciousProver) startServer() {
	http.HandleFunc("/", mp.proxyRequest)

	fmt.Printf("Malicious prover starting on port %s\n", mp.port)
	fmt.Printf("Proxying to: %s\n", mp.realProverURL)

	log.Fatal(http.ListenAndServe(":"+mp.port, nil))
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Environment variables with defaults
	realProverURL := os.Getenv("REAL_PROVER_URL")
	if realProverURL == "" {
		realProverURL = "http://18.184.43.124:8547"
	}
	port := "8548"

	// Parse corruption cutoff
	corruptionCutoffStr := os.Getenv("CORRUPTION_CUTOFF")
	corruptionCutoff := uint64(13) // Default cutoff
	if corruptionCutoffStr != "" {
		if parsed, err := strconv.ParseUint(corruptionCutoffStr, 10, 64); err == nil {
			corruptionCutoff = parsed
		} else {
			log.Printf("Warning: Invalid CORRUPTION_CUTOFF, using default %d", corruptionCutoff)
		}
	}

	corruptionChance := 0.5 // Default 50% chance

	maliciousProver := NewMaliciousProver(realProverURL, port, corruptionCutoff, corruptionChance)
	maliciousProver.startServer()
}
