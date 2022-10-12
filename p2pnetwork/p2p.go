package p2pnetwork

import (
	"PBFT/message"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

var nodeList = []int64{0, 1, 2, 3}

type P2pNetwork interface {
	BroadCast(v interface{}) error
}

// [SrvHub]: contains all TCP connections with other nodes
// [Peers]: map TCP connect to an int number
// [MsgChan]: a channel connects [p2p] with [state(consensus)], deliver consensus message, corresponding to [ch] in [state(consensus)]
type SimpleP2p struct {
	NodeId         int64
	SrvHub         *net.TCPListener
	Peers          map[string]*net.TCPConn
	PrivateKey     *ecdsa.PrivateKey
	PeerPublicKeys map[string]*ecdsa.PublicKey
	MsgChan        chan<- *message.ConMessage
}

// new simple P2P liarary
func NewSimpleP2pLib(id int64, msgChan chan<- *message.ConMessage) P2pNetwork {
	port := message.PortByID(id)
	s, err := net.ListenTCP("tcp4", &net.TCPAddr{
		Port: port,
	})
	if err != nil {
		panic(err)
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	sp := &SimpleP2p{
		NodeId:         id,
		SrvHub:         s,
		Peers:          make(map[string]*net.TCPConn),
		PrivateKey:     privateKey,
		PeerPublicKeys: make(map[string]*ecdsa.PublicKey),
		MsgChan:        msgChan,
	}
	go sp.monitor()

	for _, pid := range nodeList {
		if pid == id {
			continue
		}

		rPort := message.PortByID(pid)
		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: rPort})
		if err != nil {
			fmt.Printf("\nnode [%d] is not valid currently\n", pid)
			continue
		}
		sp.Peers[conn.RemoteAddr().String()] = conn
		fmt.Printf("node [%d] connected=[%s=>%s]\n", pid, conn.LocalAddr().String(), conn.RemoteAddr().String())

		go sp.waitData(conn)
	}

	// new public key message
	npkMsg := &message.NewPublicKey{
		NodeID: sp.NodeId,
		PK:     &privateKey.PublicKey,
	}
	cMsg := message.CreateConMsg(message.MTRequest, npkMsg)
	if err := sp.BroadCast(cMsg); err != nil {
		panic(err)
	}

	return sp
}

// add new node OR remove old node
func (sp *SimpleP2p) monitor() {
	fmt.Printf("===>P2p node is waiting at:%s\n", sp.SrvHub.Addr().String())
	for {
		conn, err := sp.SrvHub.AcceptTCP()
		if err != nil {
			fmt.Printf("P2p network accept err:%s\n", err)
			if err == io.EOF {
				fmt.Printf("Remove peer node%s\n", conn.RemoteAddr().String())
				delete(sp.Peers, conn.RemoteAddr().String())
			}
			continue
		}

		sp.Peers[conn.RemoteAddr().String()] = conn
		fmt.Printf("connection create [%s->%s]\n", conn.RemoteAddr().String(), conn.LocalAddr().String())

		// new public key message
		npkMsg := &message.NewPublicKey{
			NodeID: sp.NodeId,
			PK:     &sp.PrivateKey.PublicKey,
		}
		cMsg := message.CreateConMsg(message.MTRequest, npkMsg)
		if err := sp.BroadCast(cMsg); err != nil {
			panic(err)
		}

		go sp.waitData(conn)
	}
}

// remove old node AND deliver consensus mseeage by [MsgChan]
func (sp *SimpleP2p) waitData(conn *net.TCPConn) {
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("P2p network capture data err:%s\n", err)
			if err == io.EOF {
				fmt.Printf("Remove peer node%s\n", conn.RemoteAddr().String())
				delete(sp.Peers, conn.RemoteAddr().String())
				return
			}
			continue
		}

		conMsg := &message.ConMessage{}
		if err := json.Unmarshal(buf[:n], conMsg); err != nil {
			fmt.Println(string(buf[:n]))
			panic(err)
		}
		if conMsg.Typ == message.MTPublicKey {
			npk := &message.NewPublicKey{}
			if err := json.Unmarshal(conMsg.Payload, npk); err != nil {
				return fmt.Errorf("======>[procConsensusMsg] invalid[%s] didiViewChange message[%s]", err, conMsg)
			}
		}

		s.p2pWire.PeerPublicKeys[npk.NodeID] = npk.PK
		sp.MsgChan <- conMsg
	}
}

// BroadCast message to all connected nodes
func (sp *SimpleP2p) BroadCast(v interface{}) error {
	if v == nil {
		return fmt.Errorf("empty msg body")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	for name, conn := range sp.Peers {
		_, err := conn.Write(data)
		if err != nil {
			fmt.Printf("write to node[%s] err:%s\n", name, err)
		}
	}
	// time.Sleep(300 * time.Millisecond)
	return nil
}
