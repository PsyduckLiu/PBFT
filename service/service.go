package service

import (
	"PBFT/consensus"
	"PBFT/message"
	"PBFT/signature"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
)

// [SrvHub]: contians UDP connection with client
// [nodeChan]: a channel connects [service] with [node], deliver service message(request), corresponding to [srvChan] in [node]
type Service struct {
	SrvHub   *net.UDPConn
	nodeChan chan interface{}
}

// initialize node's service
func InitService(port int, msgChan chan interface{}) *Service {
	locAddr := net.UDPAddr{
		Port: port,
	}
	srv, err := net.ListenUDP("udp4", &locAddr)
	if err != nil {
		return nil
	}
	fmt.Printf("===>Service is Listening at[%d]\n", port)

	s := &Service{
		SrvHub:   srv,
		nodeChan: msgChan,
	}

	return s
}

// wait for request from client in UDP channel
func (s *Service) WaitRequest(sig chan interface{}, stateMachine *consensus.StateEngine) {
	defer func() {
		if r := recover(); r != nil {
			sig <- r
		}
	}()

	buf := make([]byte, 2048)
	for {
		n, rAddr, err := s.SrvHub.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("===>[ERROR]Service received err:%s\n", err)
			continue
		}

		// time.Sleep(100 * time.Millisecond)

		// get request message from client
		requestFromClient := &message.ClientMessage{}
		if err := json.Unmarshal(buf[:n], requestFromClient); err != nil {
			fmt.Printf("===>[ERROR]Service message parse err:%s\n", err)
			continue
		}

		// make a client request message without signature for verifying signature
		requestWithoutSig := &message.ClientMessage{
			TimeStamp: requestFromClient.TimeStamp,
			ClientID:  requestFromClient.ClientID,
			Operation: requestFromClient.Operation,
			PublicKey: requestFromClient.PublicKey,
		}

		// recover the public key and verify the signature
		pub, err := x509.ParsePKIXPublicKey(requestWithoutSig.PublicKey)
		if err != nil {
			fmt.Printf("===>[ERROR]Key message parse err:%s\n", err)
			continue
		}
		publicKey := pub.(*ecdsa.PublicKey)
		verify := signature.VerifySig([]byte(fmt.Sprintf("%v", requestWithoutSig)), requestFromClient.Sig, publicKey)
		if !verify {
			fmt.Printf("===>[ERROR]Verify new request Signature failed, From Client[%s]\n", requestFromClient.ClientID)
			continue
		}

		// record the new client's public key
		stateMachine.P2pWire.NewClientPublickey(requestFromClient.ClientID, publicKey)

		// create request message used in PBFT consensus
		requestInPBFT := &message.Request{
			TimeStamp: requestFromClient.TimeStamp,
			ClientID:  requestFromClient.ClientID,
			Operation: requestFromClient.Operation,
		}
		fmt.Printf("===>Service message from[%s], Length is [%d], Client id is[%s], Operation is [%s]\n", rAddr.String(), n, requestInPBFT.ClientID, requestInPBFT.Operation)

		// process the request message
		go s.process(requestInPBFT)
	}
}

// process the request message
// send the message by s.nodeChan(node.srvChan), then it will invokeconsensus.inspireConsensus()
func (s *Service) process(op *message.Request) {
	/*
		TODO:: Check operation
		1. if clientID is authorized
		2. if operation is valid
	*/
	s.nodeChan <- op
}

// After commit phase, execute the operation in the request message
func (s *Service) Execute(v, n, seq int64, o *message.Request) (reply *message.Reply, err error) {
	fmt.Printf("===>Service is executing opertion[%s]......\n", o.Operation)
	r := &message.Reply{
		SeqID:     seq,
		ViewID:    v,
		Timestamp: o.TimeStamp,
		ClientID:  o.ClientID,
		NodeID:    n,
		Result:    "success",
	}

	bs, _ := json.Marshal(r)
	cAddr := net.UDPAddr{
		Port: 8088,
	}
	no, err := s.SrvHub.WriteToUDP(bs, &cAddr)
	if err != nil {
		fmt.Printf("===>[ERROR]Reply client failed:%s\n", err)
		return nil, err
	}
	fmt.Printf("===>Reply Success! Seq is [%d], Result is [%s], Length is [%d]\n", seq, r.Result, no)

	return r, nil
}

// the same request asked twice, directly reply the result
func (s *Service) DirectReply(r *message.Reply) error {
	bs, _ := json.Marshal(r)
	cAddr := net.UDPAddr{
		Port: 8088,
	}
	no, err := s.SrvHub.WriteToUDP(bs, &cAddr)
	if err != nil {
		fmt.Printf("===>[ERROR]Reply client failed:%s\n", err)
		return err
	}
	fmt.Printf("===>Reply Success! Seq is [%d], Result is [%s], Length is [%d]\n", r.SeqID, r.Result, no)

	return nil
}
