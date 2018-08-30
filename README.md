
# BlockTracker

Based on [ethereumjs-blockstream](https://github.com/ethereumjs/ethereumjs-blockstream).

## Command Line Tool Usage

Install the command line tool with:

```
go get -u github.com/ferranbt/blocktracker/cmd/blocktracker
```

Then run it with:

```
blocktracker
```

## Library Usage

Install the library with:

```
go get -u github.com/ferranbt/blocktracker
```

```
tracker, err := blocktracker.NewBlockTrackerWithEndpoint(logger, rpcEndpoint, true)
if err != nil {
    panic(err)
}

eventCh := make(chan blocktracker.Event)
tracker.EventCh = eventCh

tracker.Start(context.Background())

for {
    evnt := <-eventCh:
	
	fmt.Println("-------------------------------------")
	for _, b := range evnt.Added {
		block := b.(*types.Block)
		fmt.Printf("ADD %s: %s\n", block.Number().String(), block.Hash().String())
	}
	for _, b := range evnt.Removed {
		block := b.(*types.Block)
		fmt.Printf("DEL %s: %s\n", block.Number().String(), block.Hash().String())
	}
}
```
