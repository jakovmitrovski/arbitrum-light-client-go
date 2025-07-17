package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type MaliciousProver struct {
	realProverURL string
	requestCount  int
	port          string
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

func NewMaliciousProver(realProverURL, port string) *MaliciousProver {
	return &MaliciousProver{
		realProverURL: realProverURL,
		requestCount:  0,
		port:          port,
	}
}

func (mp *MaliciousProver) proxyRequest(w http.ResponseWriter, r *http.Request) {
	mp.requestCount++

	fmt.Printf("Received request #%d: %s %s\n", mp.requestCount, r.Method, r.URL.Path)

	// Read the request body to check the method
	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		fmt.Printf("Error decoding request body: %v\n", err)
		http.Error(w, fmt.Sprintf("Error decoding request: %v", err), http.StatusInternalServerError)
		return
	}

	method, _ := requestBody["method"].(string)

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

	if strings.Contains(method, "lightclient_getLatestIndexL1") {
		hardcodedResponse := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      requestBody["id"],
			"result": map[string]interface{}{
				"StateIndex": uint64(19), // Hardcoded index
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hardcodedResponse)
		return
	}

	// Only corrupt eth_getBlockByNumber responses based on block number parameter
	if strings.Contains(method, "eth_getBlockByNumber") {
		// Extract the block number parameter
		params, ok := requestBody["params"].([]interface{})
		if ok && len(params) > 0 {
			blockNumStr, ok := params[0].(string)
			if ok {
				// Parse the block number (remove "0x" prefix if present)
				blockNumStr = strings.TrimPrefix(blockNumStr, "0x")
				blockNum, err := strconv.ParseUint(blockNumStr, 16, 64)
				if err == nil && blockNum%3 == 0 && blockNum != 0 {
					fmt.Printf("Corrupting state root for block %d (request #%d)\n", blockNum, mp.requestCount)

					// Create a deep copy of the response to avoid modifying the original
					corruptedResponse := deepCopyMap(realResponse)

					// Corrupt only the state root in the copied response
					if result, ok := corruptedResponse["result"].(map[string]interface{}); ok {
						// Change the state root to a different hash
						result["stateRoot"] = "0x" + strings.Repeat("dead", 16)
					}

					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(corruptedResponse)
					return
				}
			}
		}
	}

	// Return the real response for all other cases
	fmt.Printf("Returning real data for request #%d (%s)\n", mp.requestCount, method)
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

	realProverURL := os.Getenv("REAL_PROVER_URL")
	if realProverURL == "" {
		realProverURL = "http://localhost:8547" // Default to localhost
	}

	port := os.Getenv("MALICIOUS_PROVER_PORT")
	if port == "" {
		port = "8548" // Default port
	}

	maliciousProver := NewMaliciousProver(realProverURL, port)
	maliciousProver.startServer()
}
