package message

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
)

func CreateKeyMsg(t MType, id int64, pk *ecdsa.PublicKey) *ConMessage {
	data := elliptic.Marshal(pk.Curve, pk.X, pk.Y)

	// TODO: modify signature
	sig := fmt.Sprintf("consensus message[%s]", t)
	keyMsg := &ConMessage{
		Typ:     t,
		Sig:     sig,
		From:    id,
		Payload: data,
	}
	return keyMsg
}
