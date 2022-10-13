package service

import (
	"PBFT/message"
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

func (s *Service) WaitRequest(sig chan interface{}) {
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

		bo := &message.Request{}
		if err := json.Unmarshal(buf[:n], bo); err != nil {
			fmt.Printf("\nService message parse err:%s", err)
			continue
		}
		fmt.Printf("\nService message from[%s], Length is [%d], Client id is[%s], Operation is [%s]\n", rAddr.String(), n, bo.ClientID, bo.Operation)

		go s.process(bo)
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
