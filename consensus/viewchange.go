package consensus

import (
	"PBFT/message"
	"fmt"
)

// view change cache
// vcMsg = view change message   
// VMessage map[int64]*ViewChange
// nvMsg = new view message
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

// invoked by state.go when timeout
func (s *StateEngine) ViewChange() {
	// fmt.Printf("======>[ViewChange] (%d, %d).....\n", s.CurViewID, s.lastCP.Seq)
	fmt.Printf("======>[ViewChange] Current view is(%d).....\n", s.CurViewID)
	s.nodeStatus = ViewChanging
	s.Timer.tack()

	vc := &message.ViewChange{
		NewViewID: s.CurViewID + 1,
		NodeID:    s.NodeID,
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

// invoked by state.go when received a viewchage message
func (s *StateEngine) procViewChange(vc *message.ViewChange) error {
	nextPrimaryID := vc.NewViewID % message.TotalNodeNO
	if s.NodeID != nextPrimaryID {
		fmt.Printf("I'm Node[%d] not the new[%d] primary node\n", s.NodeID, nextPrimaryID)
		return nil
	}

	s.sCache.pushVC(vc)
	if len(s.sCache.vcMsg) < 2*message.MaxFaultyNode {
		return nil
	}
	if s.sCache.hasNewViewYet(vc.NewViewID) {
		fmt.Printf("view change[%d] is in processing......\n", vc.NewViewID)
		return nil
	}

	return s.createNewViewMsg(vc.NewViewID)
}

func (s *StateEngine) createNewViewMsg(newVID int64) error {
	s.CurViewID = newVID
	nv := &message.NewView{
		NewViewID: s.CurViewID,
		VMsg:      s.sCache.vcMsg,
	}

	s.sCache.addNewView(nv)
	s.CurSequence = 0
	s.PrimaryID = s.CurViewID % message.TotalNodeNO
	fmt.Printf("======>[ViewChange] New primary is me[%d].....\n", s.PrimaryID)

	msg := message.CreateConMsg(message.MTNewView, nv)
	if err := s.p2pWire.BroadCast(msg); err != nil {
		return err
	}

	// s.cleanRequest()
	s.cleanLogandRequest()
	return nil
}

// func (s *StateEngine) cleanRequest() {
// 	for cid, client := range s.cliRecord {
// 		for seq, req := range client.Request {
// 			if req.TimeStamp < client.LastReplyTime {
// 				delete(client.Request, seq)
// 				fmt.Printf("cleaning request[%d] when view changed for client[%s]\n", seq, cid)
// 			}
// 		}
// 	}
// 	return
// }

// invoked by state.go when received a newview message
func (s *StateEngine) didChangeView(nv *message.NewView) error {
	// TODO:verify the NewView message
	s.CurViewID = nv.NewViewID
	s.sCache.vcMsg = nv.VMsg
	s.sCache.addNewView(nv)
	s.CurSequence = 0
	s.PrimaryID = s.CurViewID % message.TotalNodeNO
	fmt.Printf("======>[NewView] New primary is(%d).....\n", s.PrimaryID)

	// s.cleanRequest()
	s.cleanLogandRequest()
	return nil
}

func (s *StateEngine) cleanLogandRequest() {
	s.msgLogs = make(map[int64]*NormalLog)
	s.cliRecord = make(map[string]*ClientRecord)
	fmt.Printf("cleaning msgLogs[%d] and cliRecord [%d] when view changed\n", len(s.msgLogs), len(s.cliRecord))
}