package p2pnetwork

import (
	"PBFT/message"
	"PBFT/signature"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

var nodeList = []int64{0, 1, 2, 3}

type P2pNetwork interface {
	GetPeerPublickey(peerId int64) *ecdsa.PublicKey
	GetClientPublickey(clientId string) *ecdsa.PublicKey
	GetMySecretkey() *ecdsa.PrivateKey
	NewClientPublickey(clientId string, pk *ecdsa.PublicKey)
	BroadCast(v interface{}) error
}

// [SrvHub]: contains all TCP connections with other nodes
// [Peers]: map TCP connect to an int number
// [MsgChan]: a channel connects [p2p] with [state(consensus)], deliver consensus message, corresponding to [ch] in [state(consensus)]
type SimpleP2p struct {
	NodeId           int64
	SrvHub           *net.TCPListener
	Peers            map[string]*net.TCPConn
	Ip2Id            map[string]int64
	PrivateKey       *ecdsa.PrivateKey
	PeerPublicKeys   map[int64]*ecdsa.PublicKey
	ClientPublicKeys map[string]*ecdsa.PublicKey
	MsgChan          chan<- *message.ConMessage
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
	fmt.Printf("===>[Node%d] my own key is: %v\n", id, privateKey)

	sp := &SimpleP2p{
		NodeId:           id,
		SrvHub:           s,
		Peers:            make(map[string]*net.TCPConn),
		Ip2Id:            make(map[string]int64),
		PrivateKey:       privateKey,
		PeerPublicKeys:   make(map[int64]*ecdsa.PublicKey),
		ClientPublicKeys: make(map[string]*ecdsa.PublicKey),
		MsgChan:          msgChan,
	}
	go sp.monitor(id)

	for _, pid := range nodeList {
		if pid == id {
			continue
		}

		rPort := message.PortByID(pid)
		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: rPort})
		if err != nil {
			fmt.Printf("===>[Node%d] is not valid currently\n", pid)
			continue
		}
		sp.Peers[conn.RemoteAddr().String()] = conn
		sp.Ip2Id[conn.RemoteAddr().String()] = pid
		fmt.Printf("===>[Node%d] connected=[%s=>%s]\n", pid, conn.LocalAddr().String(), conn.RemoteAddr().String())

		// new public key message
		kMsg := message.CreateKeyMsg(message.MTPublicKey, sp.NodeId, sp.PrivateKey)
		// fmt.Println(kMsg)
		if err := sp.SendUniqueNode(conn, kMsg); err != nil {
			panic(err)
		}

		go sp.waitData(conn)
	}

	return sp
}

// add new node OR remove old node
func (sp *SimpleP2p) monitor(id int64) {
	fmt.Printf("===>P2p [Node%d] is waiting at:%s\n", id, sp.SrvHub.Addr().String())

	for {
		conn, err := sp.SrvHub.AcceptTCP()
		if err != nil {
			fmt.Printf("===>P2p network accept err:%s\n", err)
			if err == io.EOF {
				fmt.Printf("===>[Node%d] Remove peer node%s\n", id, conn.RemoteAddr().String())
				delete(sp.Peers, conn.RemoteAddr().String())
				fmt.Printf("===>[Node%d] Remove peer node%s's public key%s\n", id, conn.RemoteAddr().String(), sp.PeerPublicKeys[sp.Ip2Id[conn.RemoteAddr().String()]])
				delete(sp.PeerPublicKeys, sp.Ip2Id[conn.RemoteAddr().String()])
				delete(sp.Ip2Id, conn.RemoteAddr().String())
			}
			continue
		}

		sp.Peers[conn.RemoteAddr().String()] = conn
		fmt.Printf("===>[Node%d] connection create [%s->%s]\n", id, conn.RemoteAddr().String(), conn.LocalAddr().String())

		// new public key message
		kMsg := message.CreateKeyMsg(message.MTPublicKey, sp.NodeId, sp.PrivateKey)
		// fmt.Println(kMsg)
		if err := sp.SendUniqueNode(conn, kMsg); err != nil {
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
			fmt.Printf("===>P2p network capture data err:%s\n", err)
			if err == io.EOF {
				fmt.Printf("===>Remove peer node%s\n", conn.RemoteAddr().String())
				delete(sp.Peers, conn.RemoteAddr().String())
				fmt.Printf("===>Remove peer node%s's public key%s\n", conn.RemoteAddr().String(), sp.PeerPublicKeys[sp.Ip2Id[conn.RemoteAddr().String()]])
				delete(sp.PeerPublicKeys, sp.Ip2Id[conn.RemoteAddr().String()])
				delete(sp.Ip2Id, conn.RemoteAddr().String())
				return
			}
			continue
		}

		conMsg := &message.ConMessage{}
		// fmt.Println("Con", string(buf[:n]))
		if err := json.Unmarshal(buf[:n], conMsg); err != nil {
			panic(err)
		}
		// fmt.Println("Con", conMsg)
		switch conMsg.Typ {
		case message.MTPublicKey:
			pub, err := x509.ParsePKIXPublicKey(conMsg.Payload)
			if err != nil {
				fmt.Printf("Key message parse err:%s\n", err)
				continue
			}
			newPublicKey := pub.(*ecdsa.PublicKey)
			verify := signature.VerifySig(conMsg.Payload, conMsg.Sig, newPublicKey)
			if !verify {
				fmt.Printf("!===>Verify new public key Signature failed, From Node[%d], IP[%s]\n", conMsg.From, conn.RemoteAddr().String())
				break
			}

			if sp.PeerPublicKeys[conMsg.From] != newPublicKey {
				sp.Ip2Id[conn.RemoteAddr().String()] = conMsg.From
				sp.PeerPublicKeys[conMsg.From] = newPublicKey

				fmt.Printf("===>Get new public key from Node[%d], IP[%s]\n", conMsg.From, conn.RemoteAddr().String())
				fmt.Printf("===>Node[%d]'s new public key is[%s]\n", conMsg.From, newPublicKey)
			}
		default:
			sp.MsgChan <- conMsg
		}
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
			fmt.Printf("===>write to node[%s] err:%s\n", name, err)
		}
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

// BroadCast message to all connected nodes
func (sp *SimpleP2p) SendUniqueNode(conn *net.TCPConn, v interface{}) error {
	if v == nil {
		return fmt.Errorf("empty msg body")
	}
	time.Sleep(200 * time.Millisecond)
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("===>write to node[%s] err:%s\n", conn.RemoteAddr().String(), err)
	}
	return nil
}

func (sp *SimpleP2p) GetPeerPublickey(peerId int64) *ecdsa.PublicKey {
	return sp.PeerPublicKeys[peerId]
}

func (sp *SimpleP2p) GetClientPublickey(clientId string) *ecdsa.PublicKey {
	return sp.ClientPublicKeys[clientId]
}

func (sp *SimpleP2p) GetMySecretkey() *ecdsa.PrivateKey {
	return sp.PrivateKey
}

func (sp *SimpleP2p) NewClientPublickey(clientId string, pk *ecdsa.PublicKey) {
	sp.ClientPublicKeys[clientId] = pk
}
