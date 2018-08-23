package blocktracker

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
)

const (
	blockPollInterval = 4 * time.Second
)

// EthClient is the required interface
type EthClient interface {
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
}

// BlockTracker keeps track of new blocks
type BlockTracker struct {
	logger  *log.Logger
	client  EthClient
	EventCh chan *types.Block
}

// NewBlockTracker starts a new tracker
func NewBlockTracker(logger *log.Logger, client EthClient) *BlockTracker {
	return &BlockTracker{
		logger: logger,
		client: client,
	}
}

// NewBlockTrackerWithEndpoint starts a new tracker for a specific endpoint
func NewBlockTrackerWithEndpoint(logger *log.Logger, endpoint string) (*BlockTracker, error) {
	rpc, err := ethrpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	return NewBlockTracker(logger, ethclient.NewClient(rpc)), nil
}

// Start tracking blocks
func (b *BlockTracker) Start(ctx context.Context) {
	var lastBlock *types.Block

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(blockPollInterval):
			block, err := b.client.BlockByNumber(ctx, nil)
			if err != nil {
				continue
			}

			if lastBlock != nil && lastBlock.Hash() == block.Hash() {
				continue
			}

			lastBlock = block
			b.EventCh <- block
		}
	}
}
