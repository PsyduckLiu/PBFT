package main

import (
	"PBFT/message"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

func request(conn *net.UDPConn, wg *sync.RWMutex, sk *ecdsa.PrivateKey) {
	for {
		wg.Lock()

		primaryID, _ := strconv.Atoi(os.Args[1])
		rAddr := net.UDPAddr{
			Port: message.PortByID(int64(primaryID)),
		}

		kMsg := message.CreateClientKeyMsg(sk)
		bs, err := json.Marshal(kMsg)
		if err != nil {
			panic(err)
		}

		n, err := conn.WriteToUDP(bs, &rAddr)
		if err != nil || n == 0 {
			panic(err)
		}
		fmt.Println("Send request success!:=>")
	}
}

func normalCaseOperation(roundSize int, sk *ecdsa.PrivateKey) {
	beginTime := time.Now()
	fmt.Println("start test.....")
	lclAddr := net.UDPAddr{
		Port: 8088,
	}
	conn, err := net.ListenUDP("udp4", &lclAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	locker := &sync.RWMutex{}
	// go sendPublickey2Primary(conn, locker, sk)
	go request(conn, locker, sk)

	waitBuffer := make([]byte, 1024)
	var counter = 0
	var curSeq int64 = 0
	for {
		n, _, err := conn.ReadFromUDP(waitBuffer)
		if err != nil {
			panic(err)
		}

		re := &message.Reply{}
		if err := json.Unmarshal(waitBuffer[:n], re); err != nil {
			panic(err)
		}

		if curSeq > re.SeqID {
			continue
		}

		fmt.Printf("Client Read[%d] Reply from[%d], Result is[%s]:\n", n, re.NodeID, re.Result)
		counter++
		if counter >= 4 {
			fmt.Printf("Consensus(seq=%d) operation(%d) success!\n", curSeq, roundSize)
			locker.Unlock()
			counter = 0
			roundSize--
			curSeq = re.SeqID + 1
		}
		if roundSize <= 0 {
			fmt.Println("Test case finished")
			endTime := time.Since(beginTime)
			fmt.Println("Total Run time is:", endTime)
			fmt.Println("Average Run time is:", endTime)
			os.Exit(0)
		}
	}
}

func main() {
	//normalCaseOperation(51)
	//normalCaseOperation(21)
	//normalCaseOperation(30)
	//normalCaseOperation(100)

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	fmt.Printf("===>[Client]my own key is: %v\n", privateKey)

	// beginTime := time.Now()
	normalCaseOperation(20, privateKey)
	// endTime := time.Since(beginTime)

	// fmt.Println("Run time is:", endTime)
}
