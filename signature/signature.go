package signature

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
)

func Digest(msg []byte) []byte {
	h := sha256.New()
	h.Write(msg)
	digest := h.Sum(nil)

	return digest
}

func GenerateSig(msg []byte, sk *ecdsa.PrivateKey) []byte {
	digest := Digest(msg)
	// fmt.Println(digest)
	sig, err := ecdsa.SignASN1(rand.Reader, sk, digest)
	if err != nil {
		panic(err)
	}
	return sig
}

func VerifySig(msg []byte, sig []byte, pk *ecdsa.PublicKey) bool {
	digest := Digest(msg)
	// fmt.Println(digest)
	// fmt.Println(sig)
	// fmt.Println("VerifySig", pk)
	valid := ecdsa.VerifyASN1(pk, digest, sig)
	// fmt.Println(valid)
	return valid
}
