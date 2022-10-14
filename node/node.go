package node

import (
	"PBFT/consensus"
	"PBFT/message"
	"PBFT/service"
	"fmt"
)

const MaxMsgNO = 100

// [signal]:  a channel connects [node] with [consensus] and [service], deliver the exit message
// [srvChan]: a channel connects [node] with [service], deliver service message(request)
// [conChan]: a channel connects [node] with [consensus], deliver {message.RequestRecord} message to clients
// [directReplyChan]: a channel connects [node] with [consensus], deliver {message.Reply} message to clients
type Node struct {
	NodeID          int64
	signal          chan interface{}
	srvChan         chan interface{}
	conChan         <-chan *message.RequestRecord
	directReplyChan <-chan *message.Reply
	waitQueue       []*message.Request
	consensus       *consensus.StateEngine
	service         *service.Service
}

func NewNode(id int64) *Node {
	srvChan := make(chan interface{}, MaxMsgNO)
	conChan := make(chan *message.RequestRecord, MaxMsgNO)
	rChan := make(chan *message.Reply, MaxMsgNO)

	c := consensus.InitConsensus(id, conChan, rChan)
	sr := service.InitService(message.PortByID(id), srvChan)

	n := &Node{
		NodeID:          id,
		consensus:       c,
		service:         sr,
		srvChan:         srvChan,
		waitQueue:       make([]*message.Request, 0),
		signal:          make(chan interface{}),
		conChan:         conChan,
		directReplyChan: rChan,
	}
	return n
}

func (n *Node) Run() {
	fmt.Printf("\nConsensus node[%d] start primary[%t]......\n", n.NodeID, n.NodeID == n.consensus.PrimaryID)

	go n.consensus.StartConsensus(n.signal)
	go n.service.WaitRequest(n.signal, n.consensus)
	go n.Dispatch()

	s := <-n.signal
	fmt.Printf("Node[%d] exit because of:%s", n.NodeID, s)
}

func (n *Node) Dispatch() {
	for {
		select {
		// handle service message
		case srvMsg := <-n.srvChan:
			opMsg, ok := srvMsg.(*message.Request)
			if !ok {
				return
			}
			// a new service message invokes InspireConsensus()
			if err := n.consensus.InspireConsensus(opMsg); err != nil {
				fmt.Printf("consesus layer err:%s", err)
				n.waitQueue = append(n.waitQueue, opMsg)
			}

		// handle comitted message.RequestRecord
		case record := <-n.conChan:
			reply, err := n.service.Execute(record.ViewID, n.NodeID, record.SequenceID, record.Request)
			if err != nil {
				fmt.Printf("service layer err:%s", err)
				continue
			}
			n.consensus.ResetState(reply)

		// handle committed and replied message.Reply
		case reply := <-n.directReplyChan:
			if err := n.service.DirectReply(reply); err != nil {
				fmt.Println(err)
				continue
			}
		}
	}
}
