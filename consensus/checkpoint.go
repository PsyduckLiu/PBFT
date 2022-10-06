package consensus

// import (
// 	"PBFT/message"
// 	"fmt"
// )

// type CheckPoint struct {
// 	Seq      int64                         `json:"sequence"`
// 	Digest   string                        `json:"digest"`
// 	IsStable bool                          `json:"isStable"`
// 	ViewID   int64                         `json:"viewID"`
// 	CPMsg    map[int64]*message.CheckPoint `json:"checks"`
// }

// func NewCheckPoint(sq, vi int64) *CheckPoint {
// 	cp := &CheckPoint{
// 		Seq:      sq,
// 		IsStable: false,
// 		ViewID:   vi,
// 		CPMsg:    make(map[int64]*message.CheckPoint),
// 	}
// 	return cp
// }

// func (s *StateEngine) ResetState(reply *message.Reply) {
// 	s.msgLogs[reply.SeqID].Stage = Idle
// 	s.LasExeSeq = reply.SeqID

// 	if s.CurSequence%CheckPointInterval == 0 {
// 		fmt.Printf("======>[ResetState]Need to create check points(%d)\n", s.CurSequence)
// 		go s.createCheckPoint(s.CurSequence)
// 	}
// 	s.cliRecord[reply.ClientID].saveReply(reply)
// }

// func (s *StateEngine) createCheckPoint(sequence int64) {
// 	msg := &message.CheckPoint{
// 		SequenceID: sequence,
// 		NodeID:     s.NodeID,
// 		ViewID:     s.CurViewID,
// 		Digest:     fmt.Sprintf("checkpoint message for [seq(%d)]", sequence),
// 	}

// 	cp := NewCheckPoint(sequence, s.CurViewID)
// 	cp.Digest = fmt.Sprintf("check point message<%d, %d>", s.NodeID, sequence)
// 	cp.CPMsg[s.NodeID] = msg
// 	s.checks[sequence] = cp

// 	fmt.Printf("======>[createCheckPoint] Broadcast check point message<%d, %d>\n", s.NodeID, sequence)
// 	consMsg := message.CreateConMsg(message.MTCommit, msg)
// 	err := s.p2pWire.BroadCast(consMsg)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// }

// func (s *StateEngine) checkingPoint(msg *message.CheckPoint) error {
// 	cp, ok := s.checks[msg.SequenceID]
// 	if !ok {
// 		cp = NewCheckPoint(msg.SequenceID, s.CurViewID)
// 		s.checks[msg.SequenceID] = cp
// 	}
// 	cp.CPMsg[msg.NodeID] = msg
// 	s.runCheckPoint(msg.SequenceID)
// 	return nil
// }

// func (s *StateEngine) runCheckPoint(seq int64) {
// 	cp, ok := s.checks[seq]
// 	if !ok {
// 		return

// 	}
// 	if len(cp.CPMsg) < 2*message.MaxFaultyNode+1 {
// 		fmt.Printf("======>[checkingPoint] message counter:[%d]\n", len(cp.CPMsg))
// 		return
// 	}
// 	if cp.IsStable {
// 		fmt.Printf("======>[checkingPoint] Check Point for [%d] has confirmed\n", cp.Seq)
// 		return
// 	}

// 	fmt.Println("======>[checkingPoint] Start to clean the old message data......")
// 	cp.IsStable = true
// 	for id, log := range s.msgLogs {
// 		if id > cp.Seq {
// 			continue
// 		}
// 		log.PrePrepare = nil
// 		log.Commit = nil
// 		delete(s.msgLogs, id)
// 		fmt.Printf("======>[checkingPoint] Delete log message:CPseq=%d  clientID=%s\n", id, log.clientID)
// 	}

// 	for id, cps := range s.checks {
// 		if id >= cp.Seq {
// 			continue
// 		}
// 		cps.CPMsg = nil
// 		delete(s.checks, id)
// 		fmt.Printf("======>[checkingPoint] Delete Checkpoint:seq=%d stable=%t\n", id, cps.IsStable)
// 	}

// 	s.MiniSeq = cp.Seq
// 	s.MaxSeq = s.MiniSeq + CheckPointK
// 	s.lastCP = cp
// 	fmt.Printf("======>[checkingPoint] Success in Checkpoint forwarding[(%d, %d)]......\n", s.MiniSeq, s.MaxSeq)
// }
