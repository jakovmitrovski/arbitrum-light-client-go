package batchhandler

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/offchainlabs/nitro/arbnode"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
	"github.com/offchainlabs/nitro/arbstate/daprovider"
	"github.com/offchainlabs/nitro/cmd/chaininfo"
	"github.com/offchainlabs/nitro/cmd/util/confighelpers"
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

const defaultChainInfoFile = "../nitro/cmd/chaininfo/arbitrum_chain_info.json"

func parseBatchHandlerType(args []string) (*BatchHandlerType, error) {
	f := flag.NewFlagSet("batchHandler", flag.ContinueOnError)
	f.String("parent-chain-node-url", "", "URL for parent chain node")
	f.String("parent-chain-submission-tx-hash", "", "The batch submission transaction hash")
	f.Uint64("child-chain-id", 0, "Child chain id")
	f.String("chain-info-file", "", "Chain info file")
	headerreader.BlobClientAddOptions("blob-client", f)

	k, err := confighelpers.BeginCommonParse(f, args)
	if err != nil {
		println("error 1")
		return nil, err
	}

	var config BatchHandlerType
	if err := confighelpers.EndCommonParse(k, &config); err != nil {
		println("error 2")
		return nil, err
	}
	return &config, nil
}

func parseDasHandlerType(args []string) (*BatchHandlerType, error) {
	f := flag.NewFlagSet("batchHandler", flag.ContinueOnError)
	f.String("parent-chain-node-url", "", "URL for parent chain node")
	f.String("parent-chain-submission-tx-hash", "", "The batch submission transaction hash")
	f.Uint64("child-chain-id", 0, "Child chain id")
	f.String("chain-info-file", "", "Chain info file")

	k, err := confighelpers.BeginCommonParse(f, args)
	if err != nil {
		return nil, err
	}

	var config BatchHandlerType
	if err := confighelpers.EndCommonParse(k, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func StartBatchHandler(ctx context.Context, parentChainURL, txHash string, childChainId uint64, beaconRPCURL string) (*arbostypes.L1IncomingMessage, error) {
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
		return nil, err
	}

	var parentChainClient *ethclient.Client
	parentChainClient, err = ethclient.DialContext(ctx, config.ParentChainNodeURL)

	submissionTxReceipt, err := parentChainClient.TransactionReceipt(ctx, common.HexToHash(config.BatchSubmissionTxHash))

	if err != nil {
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	seqFilter, err := bridgegen.NewSequencerInboxFilterer(chainConfig.SequencerInbox, parentChainClient)

	if err != nil {
		return nil, err
	}

	batchMap := make(map[uint64]*arbnode.SequencerInboxBatch)

	seqInbox, err := arbnode.NewSequencerInbox(parentChainClient, chainConfig.SequencerInbox, 0)

	if err != nil {
		return nil, err
	}

	batches, err := seqInbox.LookupBatchesInRange(ctx, submissionTxReceipt.BlockNumber, submissionTxReceipt.BlockNumber)

	if err != nil {
		return nil, err
	}

	targetBatchNum, err := getBatchSeqNumFromSubmission(submissionTxReceipt, seqFilter)

	fmt.Println("targetBatchNum", targetBatchNum)

	if err != nil {
		return nil, err
	}

	var batch *arbnode.SequencerInboxBatch

	for _, subBatch := range batches {
		batchMap[subBatch.SequenceNumber] = subBatch
		if subBatch.SequenceNumber == targetBatchNum {
			batch = subBatch
		}
	}

	if batch == nil {
		return nil, ErrBatchNotFound
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
		return nil, fmt.Errorf("failed to get delayed msg: %w", err)
	}

	err = getPostingReportBatchAndfillin(ctx, parentChainClient, seqInbox, backend, batchFetcher)

	if err != nil {
		return nil, err
	}

	blobClient, err := headerreader.NewBlobClient(config.BlobClient, parentChainClient)
	blobClient.Initialize(ctx)
	if err != nil {
		fmt.Println("failed to initialize blob client", "err", err)
		return nil, err
	}

	var dapReaders []daprovider.Reader

	dapReaders = append(dapReaders, daprovider.NewReaderForBlobReader(blobClient))

	bytes, batchBlockHash, err := backend.PeekSequencerInbox()

	if err != nil {
		return nil, err
	}

	fmt.Println("batchBlockHash", batchBlockHash)

	parsedSequencerMsg, err := ParseSequencerMessage(ctx, backend.batchSeqNum, batchBlockHash, bytes, dapReaders, daprovider.KeysetPanicIfInvalid)

	if err != nil {
		return nil, err
	}

	// txes, err := getTxHash(parsedSequencerMsg, lastBatchDelayedCount, backend)
	// if err != nil {
	// 	fmt.Println("failed to get tx hash")
	// 	return err
	// }
	// for i := 0; i < len(txes); i++ {
	// 	fmt.Println(txes[i].Hash().Hex())
	// }

	// fmt.Println("Found tx numbder: ", len(txes))

	msg, err := getMessage(parsedSequencerMsg, 4, backend, lastBatchDelayedCount)

	if err != nil {
		return nil, err
	}

	batchData, err := batchFetcher(targetBatchNum)

	if err != nil {
		return nil, err
	}

	gas := arbostypes.ComputeBatchGasCost(batchData)
	msg.BatchGasCost = &gas

	if err != nil {
		fmt.Println("error", err)
		panic(err)
	}

	return msg, nil
}

func startDASHandler(args []string) error {
	_, err := parseDasHandlerType(args)
	if err != nil {
		return err
	}

	fmt.Printf("retrieveFromDAS is not supported now")
	return nil
}
