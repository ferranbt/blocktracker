
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
tracker, err := blocktracker.NewBlockTrackerWithEndpoint(logger, rpcEndpoint)
if err != nil {
    panic(err)
}

eventCh := make(chan *types.Block)
tracker.EventCh = eventCh

go tracker.Start(context.Background())

for {
    block := <-eventCh:
    fmt.Printf("%s: %s\n", block.Number().String(), block.Hash().String())
}
```
