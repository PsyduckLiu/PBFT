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

func InitService(port int, msgChan chan interface{}) *Service {
	// 0.0.0.0:port
	locAddr := net.UDPAddr{
		Port: port,
	}
	srv, err := net.ListenUDP("udp4", &locAddr)
	if err != nil {
		return nil
	}
	fmt.Printf("\n===>Service Listening at[%d]\n", port)

	s := &Service{
		SrvHub:   srv,
		nodeChan: msgChan,
	}
	return s
}

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
			fmt.Printf("Service received err:%s\n", err)
			continue
		}

		requestFromClient := &message.ClientMessage{}
		if err := json.Unmarshal(buf[:n], requestFromClient); err != nil {
			fmt.Printf("Service message parse err:%s\n", err)
			continue
		}

		requestWithoutSig := &message.ClientMessage{
			TimeStamp: requestFromClient.TimeStamp,
			ClientID:  requestFromClient.ClientID,
			Operation: requestFromClient.Operation,
			PublicKey: requestFromClient.PublicKey,
		}
		pub, err := x509.ParsePKIXPublicKey(requestWithoutSig.PublicKey)
		if err != nil {
			fmt.Printf("Key message parse err:%s\n", err)
			continue
		}
		publicKey := pub.(*ecdsa.PublicKey)
		verify := signature.VerifySig([]byte(fmt.Sprintf("%v", requestWithoutSig)), requestFromClient.Sig, publicKey)
		if !verify {
			fmt.Printf("!===>Verify new request Signature failed, From Client[%s]\n", requestFromClient.ClientID)
			continue
		}
		stateMachine.P2pWire.NewClientPublickey(requestFromClient.ClientID, publicKey)

		requestInPBFT := &message.Request{
			TimeStamp: requestFromClient.TimeStamp,
			ClientID:  requestFromClient.ClientID,
			Operation: requestFromClient.Operation,
		}

		fmt.Printf("\nService message from[%s], Length is [%d], Client id is[%s], Operation is [%s]\n", rAddr.String(), n, requestInPBFT.ClientID, requestInPBFT.Operation)
		go s.process(requestInPBFT)
	}
}

func (s *Service) process(op *message.Request) {
	/*
		TODO:: Check operation
		1. if clientID is authorized
		2. if operation is valid
	*/
	s.nodeChan <- op
}

func (s *Service) Execute(v, n, seq int64, o *message.Request) (reply *message.Reply, err error) {
	fmt.Printf("\nService is executing opertion[%s]......\n", o.Operation)
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
		fmt.Printf("Reply client failed:%s\n", err)
		return nil, err
	}
	fmt.Printf("Reply Success! Seq is [%d], Result is [%s], Length is [%d]\n", seq, r.Result, no)
	return r, nil
}

func (s *Service) DirectReply(r *message.Reply) error {
	bs, _ := json.Marshal(r)
	cAddr := net.UDPAddr{
		Port: 8088,
	}
	no, err := s.SrvHub.WriteToUDP(bs, &cAddr)
	if err != nil {
		fmt.Printf("Reply client failed:%s\n", err)
		return err
	}
	fmt.Printf("Reply Success! Seq is [%d], Result is [%s], Length is [%d]\n", r.SeqID, r.Result, no)
	return nil
}
