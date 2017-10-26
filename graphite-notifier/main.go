// main.go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/UltraNemesis/graphitenotifier"
)

var conf graphitenotifier.Configuration

func main() {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	graphitenotifier.LoadConfig([]string{"./conf", "../conf"}, "config", &conf)

	server := graphitenotifier.NewServer(conf)

	fmt.Println("Starting Graphite Notifier Services...")

	go server.Start()

	<-sigs
}
