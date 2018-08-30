package blocktracker

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

type DummyEthClient struct {
	blocks map[string]Block
}

func newDummyEthClient() *DummyEthClient {
	return &DummyEthClient{
		blocks: map[string]Block{},
	}
}

func (d *DummyEthClient) addBlocks(bb []Block) {
	for _, b := range bb {
		d.addBlock(b)
	}
}

func (d *DummyEthClient) addBlock(b Block) {
	d.blocks[b.Hash().String()] = b
}

func (d *DummyEthClient) getBlock(id string) (Block, error) {
	block, ok := d.blocks[id]
	if !ok {
		return nil, fmt.Errorf("Block %s not found", id)
	}
	return block, nil
}

func (d *DummyEthClient) BlockByHash(ctx context.Context, hash common.Hash) (Block, error) {
	return d.getBlock(hash.String())
}

func (d *DummyEthClient) BlockByNumber(ctx context.Context, number *big.Int) (Block, error) {
	return d.getBlock(number.String())
}

func byteToHash(x byte) common.Hash {
	return common.BytesToHash([]byte{x})
}

type block struct {
	hash   common.Hash
	parent common.Hash
}

func (b *block) Hash() common.Hash {
	return b.hash
}

func (b *block) ParentHash() common.Hash {
	return b.parent
}

func (b *block) Parent(parent byte) *block {
	b.parent = byteToHash(parent)
	return b
}

func (b *block) Eq(bb *block) bool {
	return b.hash == bb.hash || b.parent == bb.parent
}

func mock(number byte) *block {
	return &block{
		hash:   byteToHash(number),
		parent: byteToHash(number - 1),
	}
}

type blocks []Block

func TestReconcile(t *testing.T) {

	type Event struct {
		Added   blocks
		Removed blocks
	}

	type Reconcile struct {
		Block Block
		Event Event
	}

	cases := []struct {
		Name      string
		Scenario  blocks
		History   blocks
		Reconcile []Reconcile
		Expected  blocks
	}{
		{
			Name: "Empty history",
			Reconcile: []Reconcile{
				{
					Block: mock(0x1),
					Event: Event{
						Added: blocks{
							mock(0x1),
						},
					},
				},
			},
			Expected: blocks{
				mock(0x1),
			},
		},
		{
			Name: "Repeated header",
			History: blocks{
				mock(0x1),
			},
			Reconcile: []Reconcile{
				{
					Block: mock(0x1),
				},
			},
			Expected: blocks{
				mock(0x1),
			},
		},
		{
			Name: "New head",
			History: blocks{
				mock(0x1),
			},
			Reconcile: []Reconcile{
				{
					Block: mock(0x2),
					Event: Event{
						Added: blocks{
							mock(0x2),
						},
					},
				},
			},
			Expected: blocks{
				mock(0x1),
				mock(0x2),
			},
		},
		{
			Name: "Ignore block already on history",
			History: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x3),
			},
			Reconcile: []Reconcile{
				{
					Block: mock(0x2),
				},
			},
			Expected: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x3),
			},
		},
		{
			Name: "Multi Roll back",
			History: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x3),
				mock(0x4),
			},
			Reconcile: []Reconcile{
				{
					Block: mock(0x30).Parent(0x2),
					Event: Event{
						Added: blocks{
							mock(0x30).Parent(0x2),
						},
						Removed: blocks{
							mock(0x3),
							mock(0x4),
						},
					},
				},
			},
			Expected: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x30).Parent(0x2),
			},
		},
		{
			Name: "Backfills missing blocks",
			Scenario: blocks{
				mock(0x3),
				mock(0x4),
			},
			History: blocks{
				mock(0x1),
				mock(0x2),
			},
			Reconcile: []Reconcile{
				{
					Block: mock(0x5),
					Event: Event{
						Added: blocks{
							mock(0x3),
							mock(0x4),
							mock(0x5),
						},
					},
				},
			},
			Expected: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x3),
				mock(0x4),
				mock(0x5),
			},
		},
		{
			Name: "Rolls back and backfills",
			Scenario: blocks{
				mock(0x30).Parent(0x2),
				mock(0x40).Parent(0x30),
			},
			History: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x3),
				mock(0x4),
			},
			Reconcile: []Reconcile{
				{
					Block: mock(0x50).Parent(0x40),
					Event: Event{
						Added: blocks{
							mock(0x30).Parent(0x2),
							mock(0x40).Parent(0x30),
							mock(0x50).Parent(0x40),
						},
						Removed: blocks{
							mock(0x3),
							mock(0x4),
						},
					},
				},
			},
			Expected: blocks{
				mock(0x1),
				mock(0x2),
				mock(0x30).Parent(0x2),
				mock(0x40).Parent(0x30),
				mock(0x50).Parent(0x40),
			},
		},
	}

	for _, cc := range cases {
		t.Run(cc.Name, func(tt *testing.T) {
			client := newDummyEthClient()
			tracker := NewBlockTracker(nil, client, false)

			// Add scenario
			client.addBlocks(cc.Scenario)

			// bootstrap history
			for _, b := range cc.History {
				tracker.addBlock(b)
			}

			// start reconcile
			for _, r := range cc.Reconcile {
				evnt, err := tracker.handleReconcile(r.Block)
				if err != nil {
					t.Fatal(err)
				}

				if evnt != nil {
					if err := compareBlocks(r.Event.Added, evnt.Added); err != nil {
						tt.Fatalf("Failed in added events: %v", err)
					}

					if err := compareBlocks(r.Event.Removed, evnt.Removed); err != nil {
						tt.Fatalf("Failed in removed events: %v", err)
					}
				}
			}

			if err := compareBlocks(cc.Expected, tracker.blocks); err != nil {
				tt.Fatal(err)
			}
		})
	}
}

func compareBlocks(one blocks, two blocks) error {
	if len(one) != len(two) {
		return fmt.Errorf("Expected length failed")
	}

	for indx, b := range one {
		if !(b).(*block).Eq(two[indx].(*block)) {
			return fmt.Errorf("Hash failed at indx: %d", indx)
		}
	}

	return nil
}
