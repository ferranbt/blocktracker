package blocktracker

import (
	"context"
	"fmt"
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
	EventCh   chan Event
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

// Event is the reconcile event
type Event struct {
	Added   []Block
	Removed []Block
}

func (b *BlockTracker) handleReconcile(block Block) (*Event, error) {
	evnt := Event{}

	addBlock := func(block Block) {
		evnt.Added = append(evnt.Added, block)
		b.addBlock(block)
	}

	removeBlock := func(indx int) {
		for i := indx + 1; i < len(b.blocks); i++ {
			evnt.Removed = append(evnt.Removed, b.blocks[i])
		}
		b.blocks = b.blocks[:indx+1]
	}

	originalBlock := block

RECONCILE:
	// only one block in history
	if len(b.blocks) == 0 {
		addBlock(block)

	} else if b.exists(block) {
		// block already in history
		return nil, nil

	} else if b.blocks[len(b.blocks)-1].Hash() == block.ParentHash() {
		// normal sequence
		addBlock(block)

	} else if indx := b.parentHashInHistory(block.ParentHash()); indx != -1 {
		// parent in history
		removeBlock(indx)
		addBlock(block)

	} else {
		// backfill
		parent, err := b.client.BlockByHash(context.Background(), block.ParentHash())
		if err != nil {
			return nil, fmt.Errorf("Parent with hash %s not found", block.ParentHash().String())
		}

		block = parent
		goto RECONCILE
	}

	// When backfilling we have to reconcile the original block
	if originalBlock.Hash() != block.Hash() {
		block = originalBlock
		goto RECONCILE
	}

	return &evnt, nil
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
			evnt, err := b.handleReconcile(block)
			if err != nil {
				b.logger.Printf("Failed to reconcile: %v", err)
			} else if evnt != nil {
				b.EventCh <- *evnt
			}
		} else {
			b.EventCh <- Event{Added: []Block{block}}
		}
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
