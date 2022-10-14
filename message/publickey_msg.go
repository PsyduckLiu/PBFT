package message

import (
	"PBFT/signature"
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"time"
)

func CreateKeyMsg(t MType, id int64, sk *ecdsa.PrivateKey) *ConMessage {
	publicKey := sk.PublicKey
	marshalledKey, err := x509.MarshalPKIXPublicKey(&publicKey)
	if err != nil {
		panic(err)
	}
	sig := signature.GenerateSig(marshalledKey, sk)
	keyMsg := &ConMessage{
		Typ:     t,
		Sig:     sig,
		From:    id,
		Payload: marshalledKey,
	}
	return keyMsg
}

func CreateClientKeyMsg(sk *ecdsa.PrivateKey) *ClientMessage {
	publicKey := sk.PublicKey
	marshalledKey, err := x509.MarshalPKIXPublicKey(&publicKey)
	if err != nil {
		panic(err)
	}
	keyMsg := &ClientMessage{
		TimeStamp: time.Now().Unix(),
		ClientID:  "Client's address",
		Operation: "<READ TX FROM POOL>",
		PublicKey: marshalledKey,
	}

	sig := signature.GenerateSig([]byte(fmt.Sprintf("%v", keyMsg)), sk)
	// fmt.Println(sig)
	keyMsg.Sig = sig
	return keyMsg
}
