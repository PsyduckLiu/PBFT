package message

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
)

type ConMessage struct {
	Typ     MType  `json:"type"`
	Sig     string `json:"sig"`
	From    string `json:"from"`
	Payload []byte `json:"payload"`
}

func (cm *ConMessage) String() string {
	return fmt.Sprintf("\n======Consensus Messagetype======"+
		"\ntype:%40s"+
		"\nsig:%40s"+
		"\npayload:%d"+
		"\n<------------------>",
		cm.Typ.String(),
		cm.Sig,
		len(cm.Payload))
}

func (cm *ConMessage) Verify() bool {
	//hash := HASH(cm.Payload)
	//return cm.From == Revert(hash, cm.Sig)
	return true
}

func CreateConMsg(t MType, msg interface{}) *ConMessage {
	data, e := json.Marshal(msg)
	if e != nil {
		return nil
	}

	// TODO: modify signature
	sig := fmt.Sprintf("consensus message[%s]", t)
	consMsg := &ConMessage{
		Typ:     t,
		Sig:     sig,
		Payload: data,
	}
	return consMsg
}

type NewPublicKey struct {
	NodeID int64            `json:"nodeID"`
	PK     *ecdsa.PublicKey `json:"pk"`
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
