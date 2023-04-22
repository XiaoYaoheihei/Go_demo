//将服务端的所有服务全部都启动

package main

import (
	"Minymq/message"
	"context"
	"flag"
	"os"
	"os/signal"
)

var memQueueSize = flag.Int("mem-queue-size", 10000, "number of message to keep in memory (per topic)")

func main() {
	flag.Parse()

	endChan := make(chan struct{})
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	ctx, fn := context.WithCancel(context.Background())
	go func() {
		<-signalChan
		fn()
	}()

	go message.TopicFactory(ctx, *memQueueSize)
}
