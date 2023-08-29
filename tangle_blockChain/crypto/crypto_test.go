package crypto

import (
	"blockChain_consensus/tangleChain/common"
	"bytes"
	"testing"
)

type testReader struct {
}

func (tr *testReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(0)
	}
	return len(p), nil
}

func TestGenerateSpecificKey(t *testing.T) {
	reader := &testReader{}
	testhash := common.StringToHash("test")
	key1, err := GenerateSpecificKey(reader)
	if err != nil {
		t.Fatalf("fail in generate key: %v", err)
	}
	signedByte, err := SignHash(testhash, key1)
	if err != nil {
		t.Fatalf("fail in generate key: %v", err)
	}
	t.Logf("signByte is: %s", common.ToHex(signedByte))
	shouldBe := common.FromHex("0x35992fcea5d682d0b408a2db5a7ce16071662ef7db48d9d44aa11fd19895ae5045a08369c1d7052406b4ff9495023773bbf62fd56de5905316ea52b26e2e917601")
	if !bytes.Equal(shouldBe, signedByte) {
		t.Fatalf("fail in GenerateSpecificKey, got signed %x, but should be %x", signedByte, shouldBe)
	}
}
