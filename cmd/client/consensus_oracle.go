package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/offchainlabs/nitro/arbnode"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
	"github.com/offchainlabs/nitro/arbstate/daprovider"
	"github.com/offchainlabs/nitro/cmd/chaininfo"
	"github.com/offchainlabs/nitro/solgen/go/bridgegen"
	"github.com/offchainlabs/nitro/util/headerreader"
)

type BatchHandlerType struct {
	ParentChainNodeURL    string                        `koanf:"parent-chain-node-url"`
	BatchSubmissionTxHash string                        `koanf:"parent-chain-submission-tx-hash"`
	ChildChainId          uint64                        `koanf:"child-chain-id"`
	BlobClient            headerreader.BlobClientConfig `koanf:"blob-client"`
	ChainInfoFile         string                        `koanf:"chain-info-file"`
}

type DasHandlerType struct {
	ParentChainNodeURL    string                        `koanf:"parent-chain-node-url"`
	BatchSubmissionTxHash string                        `koanf:"parent-chain-submission-tx-hash"`
	ChildChainId          uint64                        `koanf:"child-chain-id"`
	BlobClient            headerreader.BlobClientConfig `koanf:"blob-client"`
	ChainInfoFile         string                        `koanf:"chain-info-file"`
}

const defaultChainInfoFile = "./nitro/cmd/chaininfo/arbitrum_chain_info.json"

func getSha256(msg []byte) string {
	hasher := sha256.New()
	hasher.Write(msg)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func ExecuteConsensusOracle(ctx context.Context, prevL1Data MessageTrackingL1Data, currL1Data MessageTrackingL1Data, parentChainURL string, childChainId uint64, beaconRPCURL string) (bool, error) {
	if prevL1Data.Message.Header.Kind == arbostypes.L1MessageType_Initialize {
		messages, _, err := StartBatchHandler(ctx, prevL1Data, parentChainURL, childChainId, beaconRPCURL)
		if err != nil {
			return false, err
		}

		return getSha256(messages[0].L2msg) == getSha256(currL1Data.Message.L2msg), nil
	}

	if prevL1Data.L1TxHash == currL1Data.L1TxHash {
		messages, _, err := StartBatchHandler(ctx, prevL1Data, parentChainURL, childChainId, beaconRPCURL)
		if err != nil {
			return false, err
		}

		for i := 0; i < len(messages); i++ {
			if getSha256(messages[i].L2msg) == getSha256(prevL1Data.Message.L2msg) && getSha256(messages[i+1].L2msg) == getSha256(currL1Data.Message.L2msg) {
				return true, nil
			}
		}
		return false, errors.New("messages are not consecutive")
	}

	// if the tx hash is different, we need to get the messages from two batches
	messages1, targetBatchNum1, err := StartBatchHandler(ctx, prevL1Data, parentChainURL, childChainId, beaconRPCURL)
	if err != nil {
		return false, err
	}
	messages2, targetBatchNum2, err := StartBatchHandler(ctx, currL1Data, parentChainURL, childChainId, beaconRPCURL)

	if err != nil {
		return false, err
	}

	if targetBatchNum2-targetBatchNum1 != 1 {
		return false, errors.New("target batch numbers are not consecutive")
	}

	last := messages1[len(messages1)-1]
	first := messages2[0]

	if getSha256(last.L2msg) == getSha256(prevL1Data.Message.L2msg) && getSha256(first.L2msg) == getSha256(currL1Data.Message.L2msg) {
		return true, nil
	} else {
		return false, errors.New("messages are not consecutive")
	}
}

func StartBatchHandler(ctx context.Context, L1Data MessageTrackingL1Data, parentChainURL string, childChainId uint64, beaconRPCURL string) ([]*arbostypes.L1IncomingMessage, uint64, error) {
	txHash := L1Data.L1TxHash.Hex()

	config := &BatchHandlerType{
		ParentChainNodeURL:    parentChainURL,
		BatchSubmissionTxHash: txHash,
		ChildChainId:          childChainId,
		BlobClient: headerreader.BlobClientConfig{
			BeaconUrl: beaconRPCURL,
		},
		ChainInfoFile: defaultChainInfoFile,
	}

	chainInfoFiles := []string{defaultChainInfoFile}
	if config.ChainInfoFile != "" {
		chainInfoFiles = append(chainInfoFiles, config.ChainInfoFile)
	}

	chainConfig, err := chaininfo.GetRollupAddressesConfig(config.ChildChainId, "", chainInfoFiles, "")
	if err != nil {
		return nil, 0, err
	}

	var parentChainClient *ethclient.Client
	parentChainClient, _ = ethclient.DialContext(ctx, config.ParentChainNodeURL)

	submissionTxReceipt, err := parentChainClient.TransactionReceipt(ctx, common.HexToHash(config.BatchSubmissionTxHash))

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	seqFilter, err := bridgegen.NewSequencerInboxFilterer(chainConfig.SequencerInbox, parentChainClient)

	if err != nil {
		return nil, 0, err
	}

	batchMap := make(map[uint64]*arbnode.SequencerInboxBatch)

	seqInbox, err := arbnode.NewSequencerInbox(parentChainClient, chainConfig.SequencerInbox, 0)

	if err != nil {
		return nil, 0, err
	}

	batches, err := seqInbox.LookupBatchesInRange(ctx, submissionTxReceipt.BlockNumber, submissionTxReceipt.BlockNumber)

	if err != nil {
		return nil, 0, err
	}

	targetBatchNum, err := getBatchSeqNumFromSubmission(submissionTxReceipt, seqFilter)

	if err != nil {
		return nil, 0, err
	}

	var batch *arbnode.SequencerInboxBatch

	for _, subBatch := range batches {
		batchMap[subBatch.SequenceNumber] = subBatch
		if subBatch.SequenceNumber == targetBatchNum {
			batch = subBatch
		}
	}

	if batch == nil {
		return nil, 0, ErrBatchNotFound
	}

	backend := &MultiplexerBackend{
		batchSeqNum:     targetBatchNum,
		batches:         batchMap,
		delayedMessages: nil,
		ctx:             ctx,
		client:          parentChainClient,
	}

	batchFetcher := func(batchNum uint64) ([]byte, error) {
		batchData, err := backend.GetBatchDataByNum(batchNum)
		if err != nil {
			if err == ErrUnknownBatch {
				return nil, nil
			} else {
				return nil, err
			}
		}
		return batchData, nil
	}

	lastBatchDelayedCount, err := getAfterDelayedBySeqNum(int64(batch.SequenceNumber)-1, seqFilter)

	err = setDelayedToBackendByIndexRange(ctx, parentChainClient, chainConfig.SequencerInbox, chainConfig.Bridge, int64(lastBatchDelayedCount), int64(batch.AfterDelayedCount)-1, backend)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get delayed msg: %w", err)
	}

	err = getPostingReportBatchAndfillin(ctx, parentChainClient, seqInbox, backend, batchFetcher)

	if err != nil {
		return nil, 0, err
	}

	blobClient, err := headerreader.NewBlobClient(config.BlobClient, parentChainClient)
	blobClient.Initialize(ctx)
	if err != nil {
		fmt.Println("failed to initialize blob client", "err", err)
		return nil, 0, err
	}

	var dapReaders []daprovider.Reader

	dapReaders = append(dapReaders, daprovider.NewReaderForBlobReader(blobClient))

	bytes, batchBlockHash, err := backend.PeekSequencerInbox()

	if err != nil {
		return nil, 0, err
	}

	parsedSequencerMsg, err := ParseSequencerMessage(ctx, backend.batchSeqNum, batchBlockHash, bytes, dapReaders, daprovider.KeysetPanicIfInvalid)

	if err != nil {
		return nil, 0, err
	}

	messages, err := LoadMessages(parsedSequencerMsg, lastBatchDelayedCount, backend, config.ChildChainId)

	if err != nil {
		return nil, 0, err
	}

	return messages, targetBatchNum, nil
}
