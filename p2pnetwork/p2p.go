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
	"sync"
	"time"
)

// const mutexLocked = 1

// var nodeList = []int64{0, 1, 2, 3}

// var nodeList = []int64{0, 1, 2, 3, 4, 5, 6}

var nodeList = []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

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
	NodeId int64
	SrvHub *net.TCPListener
	Peers  map[string]*net.TCPConn
	// PeersMutex       map[string]*sync.Mutex
	Ip2Id            map[string]int64
	PrivateKey       *ecdsa.PrivateKey
	PeerPublicKeys   map[int64]*ecdsa.PublicKey
	ClientPublicKeys map[string]*ecdsa.PublicKey
	MsgChan          chan<- *message.ConMessage
	mutex            sync.Mutex
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
		NodeId: id,
		SrvHub: s,
		Peers:  make(map[string]*net.TCPConn),
		// PeersMutex:       make(map[string]*sync.Mutex),
		Ip2Id:            make(map[string]int64),
		PrivateKey:       privateKey,
		PeerPublicKeys:   make(map[int64]*ecdsa.PublicKey),
		ClientPublicKeys: make(map[string]*ecdsa.PublicKey),
		MsgChan:          msgChan,
		mutex : sync.Mutex{},
	}
	go sp.monitor(id)

	for _, pid := range nodeList {
		if pid == id {
			continue
		}

		rPort := message.PortByID(pid)
		// mutex := sync.Mutex{}
		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: rPort})
		if err != nil {
			fmt.Printf("===>[Node%d] is not valid currently\n", pid)
			continue
		}

		sp.Peers[conn.RemoteAddr().String()] = conn
		// sp.PeersMutex[conn.RemoteAddr().String()] = &mutex
		sp.Ip2Id[conn.RemoteAddr().String()] = pid
		fmt.Printf("===>[Node%d]Connected=[%s=>%s]\n", pid, conn.LocalAddr().String(), conn.RemoteAddr().String())

		// new public key message
		// send key message to new node
		kMsg := message.CreateKeyMsg(message.MTPublicKey, sp.NodeId, sp.PrivateKey)
		// locker := &sync.RWMutex{}
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
				// delete(sp.PeersMutex, conn.RemoteAddr().String())
				fmt.Printf("===>[Node%d] Remove peer node%s's public key%s\n", id, conn.RemoteAddr().String(), sp.PeerPublicKeys[sp.Ip2Id[conn.RemoteAddr().String()]])
				delete(sp.PeerPublicKeys, sp.Ip2Id[conn.RemoteAddr().String()])
				delete(sp.Ip2Id, conn.RemoteAddr().String())
			}
			continue
		}

		// mutex := sync.Mutex{}
		sp.Peers[conn.RemoteAddr().String()] = conn
		// sp.PeersMutex[conn.RemoteAddr().String()] = &mutex
		fmt.Printf("===>[Node%d]New connection create [%s->%s]\n", id, conn.RemoteAddr().String(), conn.LocalAddr().String())

		// new public key message
		// send key message to new node
		kMsg := message.CreateKeyMsg(message.MTPublicKey, sp.NodeId, sp.PrivateKey)
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
				// delete(sp.PeersMutex, conn.RemoteAddr().String())
				fmt.Printf("===>Remove peer node%s's public key%s\n", conn.RemoteAddr().String(), sp.PeerPublicKeys[sp.Ip2Id[conn.RemoteAddr().String()]])
				delete(sp.PeerPublicKeys, sp.Ip2Id[conn.RemoteAddr().String()])
				delete(sp.Ip2Id, conn.RemoteAddr().String())
				return
			}
			continue
		}

		// fmt.Printf("===>Receive [%s->%s]\n", conn.RemoteAddr().String(), conn.LocalAddr().String())
		conMsg := &message.ConMessage{}
		if err := json.Unmarshal(buf[:n], conMsg); err != nil {
			fmt.Println(string(buf[:n]))
			panic(err)
		}
		time.Sleep(100 * time.Millisecond)
		
		// if MutexLocked(sp.PeersMutex[conn.RemoteAddr().String()]) {
		// 	sp.PeersMutex[conn.RemoteAddr().String()].Unlock()
		// }

		switch conMsg.Typ {
		// handle new public key message from backups
		case message.MTPublicKey:
			pub, err := x509.ParsePKIXPublicKey(conMsg.Payload)
			if err != nil {
				fmt.Printf("===>[ERROR]Key message parse err:%s\n", err)
				continue
			}
			newPublicKey := pub.(*ecdsa.PublicKey)
			verify := signature.VerifySig(conMsg.Payload, conMsg.Sig, newPublicKey)
			if !verify {
				fmt.Printf("===>[ERROR]Verify new public key Signature failed, From Node[%d], IP[%s]\n", conMsg.From, conn.RemoteAddr().String())
				break
			}

			if sp.PeerPublicKeys[conMsg.From] != newPublicKey {
				// mutex := sync.Mutex{}
				sp.mutex.Lock()
				sp.Ip2Id[conn.RemoteAddr().String()] = conMsg.From
				sp.PeerPublicKeys[conMsg.From] = newPublicKey
				sp.mutex.Unlock()
				// sp.PeersMutex[conn.RemoteAddr().String()] = &mutex

				fmt.Printf("===>Get new public key from Node[%d], IP[%s]\n", conMsg.From, conn.RemoteAddr().String())
				fmt.Printf("===>Node[%d]'s new public key is[%v]\n", conMsg.From, newPublicKey)
			}

			// if MutexLocked(sp.PeersMutex[conn.RemoteAddr().String()]) {
			// 	sp.PeersMutex[conn.RemoteAddr().String()].Unlock()
			// }
		// handle consensus message from backups
		default:
			sp.MsgChan <- conMsg
		}

	}
}

// BroadCast message to all connected nodes
func (sp *SimpleP2p) BroadCast(v interface{}) error {
	if v == nil {
		return fmt.Errorf("===>[ERROR]empty msg body")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	for name, conn := range sp.Peers {
		go WriteTCP(conn, data, name)
		// sp.PeersMutex[conn.RemoteAddr().String()].Lock()
		// _, err := conn.Write(data)
		// if err != nil {
		// 	fmt.Printf("===>[ERROR]write to node[%s] err:%s\n", name, err)
		// }
	}

	// time.Sleep(1000 * time.Millisecond)
	return nil
}

// BroadCast message to all connected nodes
func (sp *SimpleP2p) SendUniqueNode(conn *net.TCPConn, v interface{}) error {
	if v == nil {
		return fmt.Errorf("===>[ERROR]empty msg body")
	}

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	go WriteTCP(conn, data, conn.RemoteAddr().String())
	// _, err = conn.Write(data)
	// if err != nil {
	// 	return fmt.Errorf("===>===>[ERROR]write to node[%s] err:%s\n", conn.RemoteAddr().String(), err)
	// }

	// time.Sleep(200 * time.Millisecond)
	return nil
}

func WriteTCP(conn *net.TCPConn, v []byte, name string) {
	_, err := conn.Write(v)
	if err != nil {
		fmt.Printf("===>[ERROR]write to node[%s] err:%s\n", name, err)
		panic(err)
	}
}

// Get Peer Publickey
func (sp *SimpleP2p) GetPeerPublickey(peerId int64) *ecdsa.PublicKey {
	return sp.PeerPublicKeys[peerId]
}

// Get Client Publickey
func (sp *SimpleP2p) GetClientPublickey(clientId string) *ecdsa.PublicKey {
	return sp.ClientPublicKeys[clientId]
}

// Get My Secret key
func (sp *SimpleP2p) GetMySecretkey() *ecdsa.PrivateKey {
	return sp.PrivateKey
}

// New Client Public key
func (sp *SimpleP2p) NewClientPublickey(clientId string, pk *ecdsa.PublicKey) {
	sp.ClientPublicKeys[clientId] = pk
}

// func MutexLocked(m *sync.Mutex) bool {
// 	state := reflect.ValueOf(m).Elem().FieldByName("state")
// 	return state.Int()&mutexLocked == mutexLocked
// }
