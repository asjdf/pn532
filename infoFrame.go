package pn532

import "errors"

//Normal information frame
//PREAMBLE  1 byte
//START CODE  2 bytes (0x00 and 0xFF)
//LEN     1 byte indicating the number of bytes in the data field       (TFI and PD0 to PDn)
//LCS     1 Packet Length Checksum LCS byte that satisfies the relation: Lower byte of [LEN + LCS] = 0x00
//TFI     1 byte frame identifier, the value of this byte depends
//		on the way of the message
//			- D4h in case of a frame from the host controller to the PN532,
//			- D5h in case of a frame from the PN532 to the host controller.
//DATA    LEN-1 bytes of Packet Data Information
//		The first byte PD0 is the Command Code
//DCS     1 Data Checksum DCS byte that satisfies the relation:
//		Lower byte of [TFI + PD0 + PD1 + … + PDn + DCS] = 0x00
//POSTAMBLE 1 byte2.

type InfoFrame struct {
	PreAmble  byte
	StartCode [2]byte
	Len       byte
	Lcs       byte
	Tfi       byte
	Data      []byte
	Dcs       byte
	PostAmble byte
}

func (f *InfoFrame) calcLcs() byte {
	return ^f.Len + 1
}

func (f *InfoFrame) calcDcs() byte {
	dcs := f.Tfi
	for _, b := range f.Data {
		dcs += b
	}
	return ^dcs + 1
}

func (f *InfoFrame) Gen() []byte {
	buf := make([]byte, 0, len(f.Data)+8)
	buf = append(buf, f.PreAmble)
	buf = append(buf, f.StartCode[:]...)
	buf = append(buf, f.Len)
	buf = append(buf, f.Lcs)
	buf = append(buf, f.Tfi)
	buf = append(buf, f.Data...)
	buf = append(buf, f.Dcs)
	buf = append(buf, f.PostAmble)
	return buf
}

func NewNormalFrame(data []byte) *InfoFrame {
	frame := &InfoFrame{
		PreAmble:  0x00,
		StartCode: [2]byte{0x00, 0xFF},
		Len:       byte(len(data) + 1),
		Tfi:       0xD4,
		Data:      data,
		PostAmble: 0x00,
	}
	frame.Lcs = frame.calcLcs()
	frame.Dcs = frame.calcDcs()
	return frame
}

// Decode decode normal frame
func Decode(raw []byte) (*InfoFrame, error) {
	if len(raw) < 8 {
		return nil, errors.New("invalid length")
	}
	frame := &InfoFrame{}
	frame.PreAmble = raw[0]
	frame.StartCode = [2]byte{raw[1], raw[2]}
	frame.Len = raw[3]
	frame.Lcs = raw[4]
	if frame.calcLcs() != frame.Lcs {
		return nil, errors.New("invalid lcs")
	}
	frame.Tfi = raw[5]
	frame.Data = raw[6 : 6+frame.Len-1]
	frame.Dcs = raw[6+frame.Len-1]
	if frame.calcDcs() != frame.Dcs {
		return nil, errors.New("invalid dcs")
	}
	frame.PostAmble = raw[6+frame.Len]
	return frame, nil
}
