package pn532

import "testing"

func TestDecode(t *testing.T) {
	data := NewNormalFrame([]byte{0x4A, 0x02, 0x00})
	_, err := Decode(data.Gen())
	if err != nil {
		t.Error(err)
		return
	}
}
