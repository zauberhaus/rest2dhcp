package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zauberhaus/rest2dhcp/service"
)

func main() {

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT)

	ctx, cancel := context.WithCancel(context.Background())

	server := service.NewServer(":8080")
	server.Start(ctx)

	signal := <-done
	log.Printf("Got %v", signal.String())

	cancel()

	<-server.Done
	log.Printf("Done.")
}
