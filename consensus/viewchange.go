package consensus

import (
	"PBFT/message"
	"fmt"
)

type VCCache struct {
	vcMsg message.VMessage
	nvMsg map[int64]*message.NewView
}

func NewVCCache() *VCCache {
	return &VCCache{
		vcMsg: make(message.VMessage),
		nvMsg: make(map[int64]*message.NewView),
	}
}

func (vcc *VCCache) pushVC(vc *message.ViewChange) {
	vcc.vcMsg[vc.NodeID] = vc
}

func (vcc *VCCache) hasNewViewYet(vid int64) bool {
	if _, ok := vcc.nvMsg[vid]; ok {
		return true
	}
	return false
}

func (vcc *VCCache) addNewView(nv *message.NewView) {
	vcc.nvMsg[nv.NewViewID] = nv
}

func (s *StateEngine) computePMsg() map[int64]*message.PTuple {
	P := make(map[int64]*message.PTuple)
	for seq := s.MiniSeq; seq < s.MaxSeq; seq++ {
		log, ok := s.msgLogs[seq]
		if !ok || log.Stage < Prepared {
			continue
		}

		tuple := &message.PTuple{
			PPMsg: log.PrePrepare,
			PMsg:  log.Prepare,
		}
		P[seq] = tuple
	}

	return P
}

func (s *StateEngine) ViewChange() {
	// fmt.Printf("======>[ViewChange] (%d, %d).....\n", s.CurViewID, s.lastCP.Seq)
	fmt.Printf("======>[ViewChange] (%d).....\n", s.CurViewID)
	s.nodeStatus = ViewChanging
	s.Timer.tack()

	pMsg := s.computePMsg()

	vc := &message.ViewChange{
		NewViewID: s.CurViewID + 1,
		// LastCPSeq: s.lastCP.Seq,
		NodeID:    s.NodeID,
		// CMsg:      s.lastCP.CPMsg,
		PMsg:      pMsg,
	}

	nextPrimaryID := vc.NewViewID % message.TotalNodeNO
	if s.NodeID == nextPrimaryID {
		s.sCache.pushVC(vc) //[vc.NodeID] = vc
	}

	consMsg := message.CreateConMsg(message.MTViewChange, vc)
	if err := s.p2pWire.BroadCast(consMsg); err != nil {
		fmt.Println(err)
		return
	}
	s.CurViewID++
	s.msgLogs = make(map[int64]*NormalLog)
}

type Set map[interface{}]bool

func (s Set) put(key interface{}) {
	s[key] = true
}
func (s *StateEngine) checkViewChange(vc *message.ViewChange) error {
	if s.CurViewID > vc.NewViewID {
		return fmt.Errorf("it's[%d] not for me[%d] view change", vc.NewViewID, s.CurViewID)
	}
	if len(vc.CMsg) <= message.MaxFaultyNode {
		return fmt.Errorf("view message checking C message failed")
	}
	var counter = make(map[int64]Set)
	for id, cp := range vc.CMsg {
		if cp.ViewID >= vc.NewViewID {
			continue
		}

		if cp.SequenceID != vc.LastCPSeq {
			return fmt.Errorf("view change message C msg's n[]%d is different from vc's"+
				" h[%d]", cp.SequenceID, vc.LastCPSeq)
		}

		//TODO:: digest test
		//if cp.Digest != message.Digest(vc.LastCPSeq){
		//
		//}

		if counter[cp.ViewID] == nil {
			counter[cp.ViewID] = make(Set)
		}

		counter[cp.ViewID].put(id)
	}

	CMsgIsOK := false
	for vid, set := range counter {
		if len(set) > message.MaxFaultyNode {
			fmt.Printf("view change check C message success[%d]:\n", vid)
			CMsgIsOK = true
			break
		}
	}
	if !CMsgIsOK {
		return fmt.Errorf("no valid C message in view change msg")
	}

	counter = make(map[int64]Set)
	for seq, pt := range vc.PMsg {

		ppView := pt.PPMsg.ViewID
		//prePrimaryID :=  ppView % message.TotalNodeNO

		if seq <= vc.LastCPSeq || seq > vc.LastCPSeq+CheckPointK {
			return fmt.Errorf("view change message checking P message faild pre-prepare n=%d,"+
				" checkpoint h=%d", seq, vc.LastCPSeq)
		}
		if ppView >= vc.NewViewID {
			return fmt.Errorf("view change message checking P message faild pre-prepare view=%d,"+
				" new view id=%d", pt.PPMsg.ViewID, vc.NewViewID)
		}

		for nid, prepare := range pt.PMsg {
			if ppView != prepare.ViewID {
				return fmt.Errorf("view change message checking view id[%d] in pre-prepare is not "+
					"same as prepare's[%d]", ppView, prepare.ViewID)
			}
			if seq != prepare.SequenceID {
				return fmt.Errorf("view change message checking seq id[%d] in pre-prepare"+
					"is different from prepare's[%d]", seq, prepare.SequenceID)
			}
			counter[ppView].put(nid)
		}
	}

	PMsgIsOk := false
	for vid, set := range counter {
		if len(set) >= 2*message.MaxFaultyNode {
			fmt.Printf("view change check P message success[%d]:\n", vid)
			PMsgIsOk = true
			break
		}
	}
	if !PMsgIsOk {
		return fmt.Errorf("view change check p message failed")
	}

	return nil
}

func (s *StateEngine) procViewChange(vc *message.ViewChange) error {
	nextPrimaryID := vc.NewViewID % message.TotalNodeNO
	if s.NodeID != nextPrimaryID {
		fmt.Printf("im[%d] not the new[%d] primary node\n", s.NodeID, nextPrimaryID)
		return nil
	}
	if err := s.checkViewChange(vc); err != nil {
		return err
	}

	s.sCache.pushVC(vc)
	if len(s.sCache.vcMsg) < message.MaxFaultyNode*2 {
		return nil
	}
	if s.sCache.hasNewViewYet(vc.NewViewID) {
		fmt.Printf("view change[%d] is in processing......\n", vc.NewViewID)
		return nil
	}

	return s.createNewViewMsg(vc.NewViewID)
}

func (s *StateEngine) GetON(newVID int64) (int64, int64, message.OMessage, message.OMessage, *message.ViewChange) {
	mergeP := make(map[int64]*message.PTuple)
	var maxNinV int64 = 0
	var maxNinO int64 = 0

	var cpVC *message.ViewChange = nil
	for _, vc := range s.sCache.vcMsg {
		if vc.LastCPSeq > maxNinV {
			maxNinV = vc.LastCPSeq
			cpVC = vc
		}
		for seq, pMsg := range vc.PMsg {
			if _, ok := mergeP[seq]; ok {
				continue
			}
			mergeP[seq] = pMsg
			if seq > maxNinO {
				maxNinO = seq
			}
		}
	}

	O := make(message.OMessage)
	N := make(message.OMessage)
	for i := maxNinV + 1; i <= maxNinO; i++ {
		pt, ok := mergeP[i]
		if ok {
			O[i] = pt.PPMsg
			O[i].ViewID = newVID
		} else {
			N[i] = &message.PrePrepare{
				ViewID:     newVID,
				SequenceID: i,
				//Digest:     nil,
				Digest: string(rune(-1)),
			}
		}
	}

	return maxNinV, maxNinO, O, N, cpVC
}

func (s *StateEngine) createNewViewMsg(newVID int64) error {

	s.CurViewID = newVID
	newCP, newSeq, o, n, cpVC := s.GetON(newVID)
	nv := &message.NewView{
		NewViewID: s.CurViewID,
		VMsg:      s.sCache.vcMsg,
		OMsg:      o,
		NMsg:      n,
	}

	s.sCache.addNewView(nv)

	s.CurSequence = newSeq

	msg := message.CreateConMsg(message.MTNewView, nv)
	if err := s.p2pWire.BroadCast(msg); err != nil {
		return err
	}
	s.updateStateNV(newCP, cpVC)
	s.cleanRequest()
	return nil
}

func (s *StateEngine) updateStateNV(maxNV int64, vc *message.ViewChange) {

	if maxNV > s.lastCP.Seq {
		cp := NewCheckPoint(maxNV, s.CurViewID)
		cp.CPMsg = vc.CMsg
		s.checks[maxNV] = cp
		s.runCheckPoint(maxNV)

		s.createCheckPoint(maxNV)
	}

	if maxNV > s.LasExeSeq {
		//TODO:: last reply last reply time
		s.LasExeSeq = maxNV
	}

	return
}

func (s *StateEngine) cleanRequest() {
	for cid, client := range s.cliRecord {
		for seq, req := range client.Request {
			if req.TimeStamp < client.LastReplyTime {
				delete(client.Request, seq)
				fmt.Printf("cleaning request[%d] when view changed for client[%s]\n", seq, cid)
			}
		}
	}
	return
}

func (s *StateEngine) didChangeView(nv *message.NewView) error {

	newVID := nv.NewViewID
	s.sCache.vcMsg = nv.VMsg
	newCP, newSeq, O, N, cpVC := s.GetON(newVID)
	if !O.EQ(nv.OMsg) {
		return fmt.Errorf("new view checking O message faliled")
	}
	if !N.EQ(nv.NMsg) {
		return fmt.Errorf("new view checking N message faliled")
	}

	for _, ppMsg := range O {
		if e := s.idle2PrePrepare(ppMsg); e != nil {
			return e
		}
	}

	for _, ppMsg := range N {
		if e := s.idle2PrePrepare(ppMsg); e != nil {
			return e
		}
	}

	s.sCache.addNewView(nv)
	s.CurSequence = newSeq
	s.updateStateNV(newCP, cpVC)
	s.cleanRequest()
	return nil
}
