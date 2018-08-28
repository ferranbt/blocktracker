package blocktracker

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/ethereum/go-ethereum/common"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
)

// WrappedClient For some reasons, we cannot use ethclient directly without the wrapper
type WrappedClient struct {
	c *ethclient.Client
}

// BlockByHash returns a block by hash
func (w *WrappedClient) BlockByHash(ctx context.Context, hash common.Hash) (Block, error) {
	return w.c.BlockByHash(ctx, hash)
}

// BlockByNumber returns the block by number
func (w *WrappedClient) BlockByNumber(ctx context.Context, number *big.Int) (Block, error) {
	return w.c.BlockByNumber(ctx, number)
}

const (
	blockPollInterval  = 4 * time.Second
	maxReconcileBlocks = 10
)

// Block reference
type Block interface {
	Hash() common.Hash
	ParentHash() common.Hash
}

// EthClient is the required interface
type EthClient interface {
	BlockByHash(ctx context.Context, hash common.Hash) (Block, error)
	BlockByNumber(ctx context.Context, number *big.Int) (Block, error)
}

// BlockTracker keeps track of new blocks
type BlockTracker struct {
	logger    *log.Logger
	client    EthClient
	EventCh   chan Block
	blocks    []Block
	reconcile bool
}

// NewBlockTracker starts a new tracker
func NewBlockTracker(logger *log.Logger, client EthClient, reconcile bool) *BlockTracker {
	return &BlockTracker{
		logger:    logger,
		client:    client,
		reconcile: reconcile,
		blocks:    []Block{},
	}
}

// NewBlockTrackerWithEndpoint starts a new tracker for a specific endpoint
func NewBlockTrackerWithEndpoint(logger *log.Logger, endpoint string, reconcile bool) (*BlockTracker, error) {
	rpc, err := ethrpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	client := ethclient.NewClient(rpc)
	return NewBlockTracker(logger, &WrappedClient{client}, reconcile), nil
}

func (b *BlockTracker) addBlock(block Block) {
	if len(b.blocks) == maxReconcileBlocks {
		b.blocks = b.blocks[1:]
	}
	b.blocks = append(b.blocks, block)
}

func (b *BlockTracker) exists(block Block) bool {
	for _, b := range b.blocks {
		if b.Hash() == block.Hash() {
			return true
		}
	}
	return false
}

func (b *BlockTracker) handleReconcile(block Block) {

	// only one
	if len(b.blocks) == 0 {
		b.addBlock(block)
		return
	}

	// block already in history
	if b.exists(block) {
		return
	}

	// normal sequence
	if b.blocks[len(b.blocks)-1].Hash() == block.ParentHash() {
		b.addBlock(block)
		return
	}

	// parent in history
	if indx := b.parentHashInHistory(block.ParentHash()); indx != -1 {
		b.blocks = b.blocks[:indx+1]
		b.addBlock(block)
		return
	}

	b.backfill(block)
}

// TODO. use backfill with the goto
// TODO. retention of blocks
func (b *BlockTracker) backfill(block Block) {
	parent, err := b.client.BlockByHash(context.Background(), block.ParentHash())
	if err != nil {
		panic("Father not found")
	}

	b.handleReconcile(parent)
	b.handleReconcile(block)
}

func (b *BlockTracker) parentHashInHistory(hash common.Hash) int {
	for indx, b := range b.blocks {
		if b.Hash() == hash {
			return indx
		}
	}
	return -1
}

// Start tracking blocks
func (b *BlockTracker) Start(ctx context.Context) {
	b.polling(ctx, func(block Block) {
		if b.reconcile {
			b.handleReconcile(block)
		}

		b.EventCh <- block
	})
}

func (b *BlockTracker) polling(ctx context.Context, callback func(Block)) {
	go func() {
		var lastBlock Block

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
				callback(block)
			}
		}
	}()
}
