package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ferranbt/blocktracker"
)

const defaultRPCEndpoint = "https://mainnet.infura.io"

func main() {
	endpoint := flag.String("endpoint", defaultRPCEndpoint, "RPC endpoint")

	logger := log.New(os.Stderr, "", log.LstdFlags)
	tracker, err := blocktracker.NewBlockTrackerWithEndpoint(logger, *endpoint, true)
	if err != nil {
		fmt.Printf("Failed to start the tracker: %v", err)
		return
	}

	eventCh := make(chan blocktracker.Event)
	tracker.EventCh = eventCh

	ctx, cancel := context.WithCancel(context.Background())
	tracker.Start(ctx)

	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case evnt := <-eventCh:
			fmt.Println("-------------------------------------")
			for _, b := range evnt.Added {
				block := b.(*types.Block)
				fmt.Printf("ADD %s: %s\n", block.Number().String(), block.Hash().String())
			}
			for _, b := range evnt.Removed {
				block := b.(*types.Block)
				fmt.Printf("DEL %s: %s\n", block.Number().String(), block.Hash().String())
			}
		case sig := <-signalCh:
			fmt.Printf("Caught signal: %v", sig)
			cancel()
			return
		}
	}
}
