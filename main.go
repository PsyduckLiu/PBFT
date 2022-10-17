package main

import (
	"PBFT/node"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	// Input: node id
	// Usage: go run main.go [id]
	if len(os.Args) < 2 {
		panic("usage: input id")
	}

	id, _ := strconv.Atoi(os.Args[1])
	node := node.NewNode(int64(id))
	go node.Run()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	pid := strconv.Itoa(os.Getpid())
	fmt.Printf("===>PBFT demo is running at PID[%s]\n", pid)
	fmt.Println()
	fmt.Println("===============================================")
	fmt.Println("*                                             *")
	fmt.Println("*     Practical Byzantine Fault Tolerance     *")
	fmt.Println("*                                             *")
	fmt.Println("===============================================")
	fmt.Println()

	sig := <-sigCh
	fmt.Printf("===>Finish by signal[%s]\n", sig.String())
}
