package message

import (
	"PBFT/signature"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
)

type ConMessage struct {
	Typ     MType  `json:"type"`
	Sig     []byte `json:"sig"`
	From    int64  `json:"from"`
	Payload []byte `json:"payload"`
}

func (cm *ConMessage) String() string {
	return fmt.Sprintf("\n======Consensus Messagetype======"+
		"\ntype:%s"+
		"\nsig:%s"+
		"\nFrom:%s"+
		"\npayload:%d"+
		"\n<------------------>",
		cm.Typ.String(),
		cm.Sig,
		string(rune(cm.From)),
		len(cm.Payload))
}

func (cm *ConMessage) Verify() bool {
	//hash := HASH(cm.Payload)
	//return cm.From == Revert(hash, cm.Sig)
	return true
}

func CreateConMsg(t MType, msg interface{}, sk *ecdsa.PrivateKey, id int64) *ConMessage {
	data, e := json.Marshal(msg)
	if e != nil {
		return nil
	}
	sig := signature.GenerateSig(data, sk)
	consMsg := &ConMessage{
		Typ:     t,
		Sig:     sig,
		From:    id,
		Payload: data,
	}
	// fmt.Println(consMsg)
	return consMsg
}

type RequestRecord struct {
	*PrePrepare
	*Request
}

type PrePrepare struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     []byte `json:"digest"`
}

type PrepareMsg map[int64]*Prepare
type Prepare struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     []byte `json:"digest"`
	NodeID     int64  `json:"nodeID"`
}

type Commit struct {
	ViewID     int64  `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`
	Digest     []byte `json:"digest"`
	NodeID     int64  `json:"nodeID"`
}

type VMessage map[int64]*ViewChange
type ViewChange struct {
	NewViewID int64 `json:"newViewID"`
	NodeID    int64 `json:"nodeID"`
}

func (vc *ViewChange) Digest() string {
	// TODO: modify digest
	// return fmt.Sprintf("this is digest for[%d-%d]", vc.NewViewID, vc.LastCPSeq)
	return fmt.Sprintf("this is digest for[%d]", vc.NewViewID)
}

type NewView struct {
	NewViewID int64    `json:"newViewID"`
	VMsg      VMessage `json:"vMSG"`
}
