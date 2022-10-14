package consensus

import (
	"PBFT/message"
	"PBFT/p2pnetwork"
	"PBFT/signature"
	"encoding/json"
	"fmt"
	"time"
)

type Consensus interface {
	StartConsensus()
	PrePrepare()
	Prepare()
	Commit()
}

type Stage int

const (
	Idle Stage = iota
	PrePrepared
	Prepared
	Committed
)

func (s Stage) String() string {
	switch s {
	case Idle:
		return "Idle"
	case PrePrepared:
		return "PrePrepared"
	case Prepared:
		return "Prepared"
	case Committed:
		return "Committed"
	}
	return "Unknown"
}

const StateTimerOut = 5 * time.Second
const MaxStateMsgNO = 100

type RequestTimer struct {
	*time.Ticker
	IsOk bool
}

func newRequestTimer() *RequestTimer {
	tick := time.NewTicker(StateTimerOut)
	tick.Stop()
	return &RequestTimer{
		Ticker: tick,
		IsOk:   false,
	}
}

func (rt *RequestTimer) tick() {
	if rt.IsOk {
		return
	}
	rt.Reset(StateTimerOut)
	rt.IsOk = true
}

func (rt *RequestTimer) tack() {
	rt.IsOk = false
	rt.Stop()
}

type EngineStatus int8

const (
	Serving EngineStatus = iota
	ViewChanging
)

func (es EngineStatus) String() string {
	switch es {
	case Serving:
		return "Server consensus......"
	case ViewChanging:
		return "Changing views......"
	}
	return "Unknown"
}

type StateEngine struct {
	NodeID      int64 `json:"nodeID"`
	CurViewID   int64 `json:"viewID"`
	CurSequence int64 `json:"curSeq"`
	PrimaryID   int64 `json:"primaryID"`
	nodeStatus  EngineStatus

	Timer           *RequestTimer
	P2pWire         p2pnetwork.P2pNetwork
	MsgChan         <-chan *message.ConMessage
	nodeChan        chan<- *message.RequestRecord
	directReplyChan chan<- *message.Reply

	MiniSeq   int64 `json:"miniSeq"`
	MaxSeq    int64 `json:"maxSeq"`
	msgLogs   map[int64]*NormalLog
	cliRecord map[string]*ClientRecord
	sCache    *VCCache
}

func InitConsensus(id int64, cChan chan<- *message.RequestRecord, rChan chan<- *message.Reply) *StateEngine {
	ch := make(chan *message.ConMessage, MaxStateMsgNO)
	p2p := p2pnetwork.NewSimpleP2pLib(id, ch)
	se := &StateEngine{
		NodeID:          id,
		CurViewID:       0,
		CurSequence:     0,
		MiniSeq:         0,
		MaxSeq:          64,
		Timer:           newRequestTimer(),
		P2pWire:         p2p,
		MsgChan:         ch,
		nodeChan:        cChan,
		directReplyChan: rChan,
		msgLogs:         make(map[int64]*NormalLog),
		cliRecord:       make(map[string]*ClientRecord),
		sCache:          NewVCCache(),
	}
	se.PrimaryID = se.CurViewID % message.TotalNodeNO
	return se
}

// receive and handle consensus message
func (s *StateEngine) StartConsensus(sig chan interface{}) {
	s.nodeStatus = Serving

	for {
		select {
		case <-s.Timer.C:
			fmt.Printf("======>[Node%d]Time is out and view change starts", s.NodeID)
			s.ViewChange()
		case conMsg := <-s.MsgChan:
			switch conMsg.Typ {
			case message.MTRequest,
				message.MTPrePrepare,
				message.MTPrepare,
				message.MTCommit:
				if s.nodeStatus != Serving {
					fmt.Println("node is not in service status now......")
					continue
				}
				if err := s.procConsensusMsg(conMsg); err != nil {
					fmt.Print(err)
				}
			case message.MTViewChange,
				message.MTNewView:
				if err := s.procManageMsg(conMsg); err != nil {
					fmt.Print(err)
				}
			}
		}
	}
}

// new client ==> record
// old client ==> decide whether to directly reply
func (s *StateEngine) checkClientRecord(request *message.Request) (*ClientRecord, error) {
	client, ok := s.cliRecord[request.ClientID]
	if !ok {
		client = NewClientRecord()
		s.cliRecord[request.ClientID] = client
		fmt.Printf("======>[Primary] New Client ID:%s\n", request.ClientID)
	}

	if request.TimeStamp < client.LastReplyTime {
		rp, ok := client.Reply[request.TimeStamp]
		if ok {
			fmt.Printf("======>[Primary] direct reply:%d\n", rp.SeqID)
			s.directReplyChan <- rp
			return nil, nil
		}
		return nil, fmt.Errorf("======>[Primary] it's a old operation Request")
	}
	return client, nil
}

// get OR create log for request[seq]
func (s *StateEngine) getOrCreateLog(seq int64) *NormalLog {
	log, ok := s.msgLogs[seq]
	if !ok {
		log = NewNormalLog()
		s.msgLogs[seq] = log
	}
	return log
}

// new request comes
func (s *StateEngine) InspireConsensus(request *message.Request) error {
	newSeq := s.CurSequence
	s.CurSequence = (s.CurSequence + 1) % s.MaxSeq
	request.SeqID = newSeq
	client, err := s.checkClientRecord(request)
	if err != nil || client == nil {
		return err
	}
	client.saveRequest(request)

	dig := message.Digest(*request)
	ppMsg := &message.PrePrepare{
		ViewID:     s.CurViewID,
		SequenceID: newSeq,
		Digest:     json.RawMessage(dig),
	}

	log := s.getOrCreateLog(newSeq)
	log.PrePrepare = ppMsg
	log.clientID = request.ClientID
	log.Stage = PrePrepared

	sk := s.P2pWire.GetMySecretkey()
	cMsg := message.CreateConMsg(message.MTRequest, request, sk, s.NodeID)
	// fmt.Println(cMsg)
	if err := s.P2pWire.BroadCast(cMsg); err != nil {
		return err
	}
	cMsg = message.CreateConMsg(message.MTPrePrepare, ppMsg, sk, s.NodeID)
	if err := s.P2pWire.BroadCast(cMsg); err != nil {
		return err
	}

	fmt.Printf("======>[Primary]Consensus broadcast message(%d)\n", newSeq)
	s.Timer.tick()
	return nil
}

// Backups receive a new request
func (s *StateEngine) rawRequest(request *message.Request, msg *message.ConMessage) (err error) {
	fmt.Printf("======>[NewRequest]Receive a new request[%d]\n", request.SeqID)

	publicKey := s.P2pWire.GetPeerPublickey(msg.From)
	verify := signature.VerifySig(msg.Payload, msg.Sig, publicKey)
	if !verify {
		return fmt.Errorf("!===>Verify new request Signature failed, From Client[%s] and Node[%d]\n", request.ClientID, msg.From)
	}
	fmt.Printf("======>[NewRequest]Verify success\n")

	s.CurSequence = request.SeqID
	client, err := s.checkClientRecord(request)
	if err != nil || client == nil {
		return err
	}
	client.saveRequest(request)

	log := s.getOrCreateLog(request.SeqID)
	log.clientID = request.ClientID

	s.Timer.tick()
	return nil
}

// Backups receive a new Preprepare
func (s *StateEngine) idle2PrePrepare(ppMsg *message.PrePrepare, msg *message.ConMessage) (err error) {
	fmt.Printf("======>[idle2PrePrepare]Current sequence[%d]\n", ppMsg.SequenceID)

	publicKey := s.P2pWire.GetPeerPublickey(msg.From)
	verify := signature.VerifySig(msg.Payload, msg.Sig, publicKey)
	if !verify {
		return fmt.Errorf("!===>Verify pre-Prepare message failed, From Node[%d]\n", msg.From)
	}
	fmt.Printf("======>[idle2PrePrepare]Verify success\n")

	if ppMsg.ViewID != s.CurViewID {
		return fmt.Errorf("======>[idle2PrePrepare] invalid view id Msg=%d state=%d", ppMsg.ViewID, s.CurViewID)
	}
	if ppMsg.SequenceID > s.MaxSeq || ppMsg.SequenceID < s.MiniSeq {
		return fmt.Errorf("======>[idle2PrePrepare] sequence no[%d] invalid[%d~%d]", ppMsg.SequenceID, s.MiniSeq, s.MaxSeq)
	}

	log := s.getOrCreateLog(ppMsg.SequenceID)
	client, ok := s.cliRecord[log.clientID]
	if !ok {
		return fmt.Errorf("======>[idle2PrePrepare] haven't receive request for sequence no[%d]", ppMsg.SequenceID)
	}

	requestDigest := message.Digest(*(client.Request[ppMsg.SequenceID]))
	if string(requestDigest) != string(ppMsg.Digest) {
		return fmt.Errorf("digest in pre-Prepare message and digest for request[%d] are not the same", ppMsg.SequenceID)
	}
	if log.Stage != Idle {
		return fmt.Errorf("invalid stage[current %s] when to prePrepared", log.Stage)
	}
	if log.PrePrepare != nil {
		if string(log.PrePrepare.Digest) != string(ppMsg.Digest) {
			return fmt.Errorf("pre-Prepare message in same v-n but not same digest")
		} else {
			fmt.Println("======>[idle2PrePrepare] duplicate pre-Prepare message")
			return
		}
	}

	prepare := &message.Prepare{
		ViewID:     s.CurViewID,
		SequenceID: ppMsg.SequenceID,
		Digest:     ppMsg.Digest,
		NodeID:     s.NodeID,
	}
	sk := s.P2pWire.GetMySecretkey()
	cMsg := message.CreateConMsg(message.MTPrepare, prepare, sk, s.NodeID)
	if err := s.P2pWire.BroadCast(cMsg); err != nil {
		return err
	}

	log.PrePrepare = ppMsg
	log.Prepare[s.NodeID] = prepare
	log.Stage = PrePrepared

	fmt.Printf("======>[idle2PrePrepare] Consensus status is [%s] seq=%d\n", log.Stage, ppMsg.SequenceID)
	return nil
}

// Backups receive a new Prepare
func (s *StateEngine) prePrepare2Prepare(prepare *message.Prepare, msg *message.ConMessage) (err error) {
	fmt.Printf("======>[prePrepare2Prepare]Current Prepare Message from Node[%d]\n", prepare.NodeID)
	fmt.Printf("======>[prePrepare2Prepare]Current sequence[%d]\n", prepare.SequenceID)

	publicKey := s.P2pWire.GetPeerPublickey(msg.From)
	verify := signature.VerifySig(msg.Payload, msg.Sig, publicKey)
	if !verify {
		return fmt.Errorf("!===>Verify prePrepare2Prepare message failed, From Node[%d]\n", msg.From)
	}
	fmt.Printf("======>[prePrepare2Prepare]Verify success\n")

	if prepare.ViewID != s.CurViewID {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>invalid view id Msg=%d state=%d", prepare.ViewID, s.CurViewID)
	}
	if prepare.SequenceID > s.MaxSeq || prepare.SequenceID < s.MiniSeq {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>sequence no[%d] invalid[%d~%d]", prepare.SequenceID, s.MiniSeq, s.MaxSeq)
	}

	log, ok := s.msgLogs[prepare.SequenceID]
	if !ok {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>havn't got log for message(%d) yet", prepare.SequenceID)
	}
	if log.Stage != PrePrepared {
		return fmt.Errorf("======>[prePrepare2Prepare] current[seq=%d] state isn't PrePrepared:[%s]", prepare.SequenceID, log.Stage)
	}

	ppMsg := log.PrePrepare
	if ppMsg == nil {
		return fmt.Errorf("======>[prePrepare2Prepare]:=>havn't got pre-Prepare message(%d) yet", prepare.SequenceID)
	}

	if ppMsg.ViewID != prepare.ViewID ||
		ppMsg.SequenceID != prepare.SequenceID ||
		string(ppMsg.Digest) != string(prepare.Digest) {
		return fmt.Errorf("[Prepare]:=>not same with pre-Prepare message")
	}
	log.Prepare[prepare.NodeID] = prepare

	prepareNodeNum := 0
	for _, prepare := range log.Prepare {
		if string(prepare.Digest) == string(ppMsg.Digest) {
			prepareNodeNum++
		}
	}
	if prepareNodeNum < 2*message.MaxFaultyNode+1 {
		return nil
	}

	commit := &message.Commit{
		ViewID:     s.CurViewID,
		SequenceID: prepare.SequenceID,
		Digest:     prepare.Digest,
		NodeID:     s.NodeID,
	}
	sk := s.P2pWire.GetMySecretkey()
	cMsg := message.CreateConMsg(message.MTCommit, commit, sk, s.NodeID)
	if err := s.P2pWire.BroadCast(cMsg); err != nil {
		return err
	}

	log.Commit[s.NodeID] = commit
	log.Stage = Prepared

	fmt.Printf("======>[prePrepare2Prepare] Consensus status is [%s] seq=%d\n", log.Stage, prepare.SequenceID)
	return
}

// Backups receive a new commit
func (s *StateEngine) prepare2Commit(commit *message.Commit, msg *message.ConMessage) (err error) {
	fmt.Printf("======>[prepare2Commit]Current Commit Message from Node[%d]\n", commit.NodeID)
	fmt.Printf("======>[prepare2Commit]Current sequence[%d]\n", commit.SequenceID)

	publicKey := s.P2pWire.GetPeerPublickey(msg.From)
	verify := signature.VerifySig(msg.Payload, msg.Sig, publicKey)
	if !verify {
		return fmt.Errorf("!===>Verify prepare2Commit message failed, From Node[%d]\n", msg.From)
	}
	fmt.Printf("======>[prepare2Commit]Verify success\n")

	if commit.ViewID != s.CurViewID {
		return fmt.Errorf("======>[prepare2Commit]  invalid view id Msg=%d state=%d", commit.ViewID, s.CurViewID)
	}
	if commit.SequenceID > s.MaxSeq || commit.SequenceID < s.MiniSeq {
		return fmt.Errorf("======>[prepare2Commit] sequence no[%d] invalid[%d~%d]",
			commit.SequenceID, s.MiniSeq, s.MaxSeq)
	}

	log, ok := s.msgLogs[commit.SequenceID]
	if !ok {
		return fmt.Errorf("======>[prepare2Commit]:=>havn't got log for message(%d) yet", commit.SequenceID)
	}
	if log.Stage != Prepared {
		return fmt.Errorf("======>[prepare2Commit] current[seq=%d] state isn't Prepared:[%s]", commit.SequenceID, log.Stage)
	}

	ppMsg := log.PrePrepare
	if ppMsg == nil {
		return fmt.Errorf("======>[prepare2Commit]:=>havn't got pre-Prepare message(%d) yet", commit.SequenceID)
	}
	if ppMsg.ViewID != commit.ViewID ||
		ppMsg.SequenceID != commit.SequenceID ||
		string(ppMsg.Digest) != string(commit.Digest) {
		return fmt.Errorf("[Prepare]:=>not same with pre-Prepare message")
	}
	log.Commit[commit.NodeID] = commit

	commitNodeNum := 0
	for _, commit := range log.Commit {
		if string(commit.Digest) == string(ppMsg.Digest) {
			commitNodeNum++
		}
	}
	if commitNodeNum < 2*message.MaxFaultyNode+1 {
		return nil
	}

	log.Stage = Committed
	s.Timer.tack()
	fmt.Printf("======>[prepare2Commit]Consensus status is [%s] seq=%d and timer stop\n", log.Stage, commit.SequenceID)

	request, ok := s.cliRecord[log.clientID].Request[commit.SequenceID]
	if !ok {
		return fmt.Errorf("no raw request for such seq[%d]", commit.SequenceID)
	}
	exeParam := &message.RequestRecord{
		Request:    request,
		PrePrepare: ppMsg,
	}

	s.nodeChan <- exeParam
	return
}

func (s *StateEngine) procConsensusMsg(msg *message.ConMessage) (err error) {
	fmt.Printf("\n======>[procConsensusMsg]Consesus message type:[%s] from Node[%d]\n", msg.Typ, msg.From)

	switch msg.Typ {
	case message.MTRequest:
		request := &message.Request{}
		if err := json.Unmarshal(msg.Payload, request); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] Invalid[%s] request message[%s]", err, msg)
		}
		return s.rawRequest(request, msg)

	case message.MTPrePrepare:
		prePrepare := &message.PrePrepare{}
		if err := json.Unmarshal(msg.Payload, prePrepare); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] Invalid[%s] pre-Prepare message[%s]", err, msg)
		}
		return s.idle2PrePrepare(prePrepare, msg)

	case message.MTPrepare:
		prepare := &message.Prepare{}
		if err := json.Unmarshal(msg.Payload, prepare); err != nil {
			return fmt.Errorf("======>[procConsensusMsg]invalid[%s] Prepare message[%s]", err, msg)
		}
		return s.prePrepare2Prepare(prepare, msg)

	case message.MTCommit:
		commit := &message.Commit{}
		if err := json.Unmarshal(msg.Payload, commit); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s] Commit message[%s]", err, msg)
		}
		return s.prepare2Commit(commit, msg)
	}
	return
}

func (s *StateEngine) procManageMsg(msg *message.ConMessage) (err error) {
	switch msg.Typ {
	case message.MTViewChange:
		vc := &message.ViewChange{}
		if err := json.Unmarshal(msg.Payload, vc); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s]ViewChange message[%s]", err, msg)
		}
		return s.procViewChange(vc, msg)

	case message.MTNewView:
		vc := &message.NewView{}
		if err := json.Unmarshal(msg.Payload, vc); err != nil {
			return fmt.Errorf("======>[procConsensusMsg] invalid[%s] didiViewChange message[%s]", err, msg)
		}
		return s.didChangeView(vc, msg)
	}
	return nil
}

func (s *StateEngine) ResetState(reply *message.Reply) {
	s.msgLogs[reply.SeqID].Stage = Idle
	s.msgLogs[reply.SeqID].PrePrepare = nil
	s.msgLogs[reply.SeqID].Commit = nil
	delete(s.msgLogs, reply.SeqID)
}
