package pn532

import (
	"bytes"
	"errors"
	"github.com/asjdf/pn532/command"
	"go.bug.st/serial"
	"log"
	"strings"
)

const (
	ISO14443A = 0x00 // 卡片类型
)

type Pn532 struct {
	port   serial.Port
	wakeup bool
	logger Logger

	respBuf chan byte
	Resp    chan *RespFrame
}

func QuickInit(port string) (*Pn532, error) {
	p, err := Init(port)
	if err != nil {
		return nil, err
	}
	if success, err := p.SAMConfiguration(command.NormalMode, 0x17); err != nil || !success {
		return nil, errors.New("SAMConfiguration failed")
	}
	return p, nil
}

func Init(port string) (*Pn532, error) {
	return InitWithConf(&Config{Port: port, Mode: &serial.Mode{BaudRate: 115200}, Logger: DefaultLogger})
}

type Config struct {
	Port string // 串口号 例如 COM1 或者 /dev/ttyUSB0
	*serial.Mode
	Logger Logger
}

func InitWithConf(conf *Config) (*Pn532, error) {
	p, err := serial.Open(conf.Port, conf.Mode)
	if err != nil {
		return nil, err
	}

	if conf.Logger == nil {
		conf.Logger = DefaultLogger
	}
	pn := &Pn532{port: p,
		respBuf: make(chan byte),
		Resp:    make(chan *RespFrame),
		logger:  conf.Logger,
	}
	pn.initSerialReader()
	return pn, nil
}

type RespFrame struct {
	Type FrameType
	Raw  []byte
}

type FrameType int

const (
	UnknownFrame FrameType = iota
	NormalFrame
	ExtFrame
	ACKFrame
	NACKFrame
	ErrorFrame
)

// 串口守护进程，专门处理响应
func (p *Pn532) initSerialReader() {
	go func() {
		tmp := make([]byte, 512)
		for {
			tmpLen, err := p.port.Read(tmp)
			if err != nil {
				panic(err)
			}
			for i := 0; i < tmpLen; i++ {
				p.respBuf <- tmp[i]
			}
		}
	}()
	// NormalFrame结构
	// 00 00 FF LEN LCS TFI PD0 PD1 ……... PDn DCS 00
	// Extended information frame
	// 00 00 FF FF FF LEN_M LEN_L LCS TFI PD0 PD1 ……... PDn DCS 00
	// ACK frame
	// 00 00 FF 00 FF 00
	// NACK frame
	// 00 00 FF FF 00 00
	// Error frame
	// 00 00 FF 01 FF 7F 81 00
	go func() {
		decoding := false // 是否处于正在接收响应的状态（当前resp尚未接收完）
		currentFrame := bytes.Buffer{}
		currentFrameType := UnknownFrame
		LEN := byte(0x00)
		LCS := byte(0x00)

		LenM := byte(0x00)
		LenL := byte(0x00)

		submitFrame := func() {
			decoding = false
			raw := make([]byte, len(currentFrame.Bytes()))
			copy(raw, currentFrame.Bytes())
			p.Resp <- &RespFrame{
				Type: currentFrameType,
				Raw:  raw,
			}
			currentFrame.Reset()
			currentFrameType = UnknownFrame
			p.logger.Debugf("receive frame: % #X", raw)
		}
		dropFrame := func() {
			decoding = false
			p.logger.Debugf("drop frame: % #X", currentFrame.Bytes())
			currentFrame.Reset()
			currentFrameType = UnknownFrame
		}

		for b := range p.respBuf {
			if !decoding {
				if b == []byte{0x00, 0x00, 0xFF}[currentFrame.Len()] { // 判断是否是开始
					currentFrame.Write([]byte{b})
					if currentFrame.Len() == 3 {
						decoding = true
					}
				} else {
					dropFrame()
				}
			} else {
				// 开始解析内部结构
				currentFrame.Write([]byte{b})
				if currentFrame.Len() == 6 {
					// 判断是哪种类型的帧
					if LEN == 0xFF && LCS == 0xFF {
						currentFrameType = ExtFrame
					} else if LEN == 0x00 && LCS == 0xFF {
						currentFrameType = ACKFrame
					} else if LEN == 0xFF && LCS == 0x00 {
						currentFrameType = NACKFrame
					} else if LEN == 0x01 && LCS == 0xFF {
						currentFrameType = ErrorFrame
					} else if sum := LEN + LCS; sum&0xFF == 0x00 {
						currentFrameType = NormalFrame
					} else { // 有脏东西！哼哼哼啊啊啊啊啊啊
						dropFrame()
					}
				}

				switch currentFrameType {
				case UnknownFrame:
					switch currentFrame.Len() {
					case 4:
						LEN = b
					case 5:
						LCS = b
					}
				case NormalFrame:
					if currentFrame.Len() == 7+int(LEN) {
						// 希望未来不要浪费这个decode
						if _, err := Decode(currentFrame.Bytes()); err != nil {
							p.logger.Errorf("decode error: %s", err)
							dropFrame()
						} else {
							submitFrame()
						}
					}
				case ExtFrame:
					// 这里应该也要有check的
					switch currentFrame.Len() {
					case 6:
						LenM = b
					case 7:
						LenL = b
					case 9 + int(LenM)<<8 + int(LenL):
						submitFrame()
					}
				case ACKFrame:
					if currentFrame.Len() == 6 {
						submitFrame()
					}
				case NACKFrame:
					if currentFrame.Len() == 6 {
						submitFrame()
					}
				case ErrorFrame:
					if currentFrame.Len() == 8 {
						submitFrame()
					}
				}
			}
		}
	}()
}

func (p *Pn532) Close() error {
	return p.port.Close()
}

func (p *Pn532) Write(data []byte) (int, error) {
	return p.port.Write(data)
}

func (p *Pn532) WriteFrame(data []byte) error {
	frame := NewNormalFrame(data).Gen()
	if !p.wakeup {
		frame = append(command.WakeUp, frame...)
		p.wakeup = true // 虽然这将会导致竞争问题 但是鉴于开发者不太会同时操作睡眠和唤醒 所以不做更多处理
	}
	p.logger.Debugf("write: % #X", frame)
	_, err := p.port.Write(frame)
	return err
}

// SendCommand 发送命令至pn532 如响应正确 返回true 否则返回false
func (p *Pn532) SendCommand(data []byte) (bool, error) {
	if err := p.WriteFrame(data); err != nil {
		return false, err
	}
	switch resp := <-p.Resp; resp.Type {
	case ACKFrame:
		p.logger.Debugf("send command success")
		return true, nil
	case NACKFrame:
		p.logger.Errorf("send command failed")
		return false, nil
	default:
		p.logger.Errorf("send command error")
		return false, errors.New("response error")
	}
}

// WaitInfoFrame 等待响应帧
func (p *Pn532) WaitInfoFrame() (*InfoFrame, error) {
	for {
		switch resp := <-p.Resp; resp.Type {
		case NormalFrame:
			i, err := Decode(resp.Raw)
			if err != nil {
				return nil, err
			}
			return i, nil
		default:
			p.logger.Debugf("receive unexpect frame: % #X", resp.Raw)
		}
	}
}

// FirmwareVersion 获取固件版本
func (p *Pn532) FirmwareVersion() ([]byte, error) {
	if success, err := p.SendCommand([]byte{command.GetFirmwareVersion}); err != nil {
		return nil, err
	} else if !success {
		return nil, errors.New("send command failed")
	}
	resp, err := p.WaitInfoFrame()
	if err != nil {
		return nil, err
	}

	if resp.Data[1] == 0x32 {
		p.logger.Debugf("IC: 532")
	} else {
		p.logger.Debugf("IC: unknown")
	}
	p.logger.Debugf("Ver: %d", resp.Data[2])
	p.logger.Debugf("Rev: %d", resp.Data[3])
	var support []string
	if resp.Data[4]&0x01 != 0 {
		support = append(support, "ISO14443A")
	}
	if resp.Data[4]&0x02 != 0 {
		support = append(support, "ISO14443B")
	}
	if resp.Data[4]&0x04 != 0 {
		support = append(support, "ISO18092")
	}
	p.logger.Debugf("Support: %s", strings.Join(support, ","))

	return resp.Data[1:], nil
}

// SAMConfiguration 通常传入command.NormalMode,0x17
func (p *Pn532) SAMConfiguration(mode byte, timeout byte) (bool, error) {
	if success, err := p.SendCommand([]byte{command.SAMConfiguration, mode, timeout, 0x00}); err != nil {
		return false, err
	} else if !success {
		return false, errors.New("send command failed")
	}
	resp, err := p.WaitInfoFrame()
	if err != nil {
		return false, err
	}
	p.logger.Debugf("SAMConfiguration: % #X", resp.Data)
	return resp.Data[0] == command.SAMConfiguration+1, nil
}

// SetParameters 此命令用于设置 PN532 的内部参数，然后配置其针对不同情况的行为。
// NADUsed: Use of the NAD information in case of initiator configuration (DEP and ISO/IEC14443-4 PCD).
// DIDUsed: Use of the DID information in case of initiator configuration (or CID in case of ISO/IEC14443-4 PCD configuration).
// AutoATR_RES: Automatic generation of the ATR_RES in case of target configuration.
// AutoRATS: Automatic generation of the RATS in case of ISO/IEC14443-4 PCD mode.
// ISO14443-4_PICC: The emulation of a ISO/IEC14443-4 PICC is enabled.
// RemovePrePostAmble: The PN532 does not send Preamble and Postamble.
func (p *Pn532) SetParameters(NADUsed, DIDUsed, AutoATR_RES, AutoRATS, ISO14443_4_PICC, RemovePrePostAmble bool) (bool, error) {
	var params byte
	if NADUsed {
		params |= 0x01
	}
	if DIDUsed {
		params |= 0x02
	}
	if AutoATR_RES {
		params |= 0x04
	}
	if AutoRATS {
		params |= 0x10
	}
	if ISO14443_4_PICC {
		params |= 0x20
	}
	if RemovePrePostAmble {
		params |= 0x40
	}
	if success, err := p.SendCommand([]byte{command.SetParameters, params}); err != nil {
		return false, err
	} else if !success {
		return false, errors.New("send command failed")
	}
	resp, err := p.WaitInfoFrame()
	if err != nil {
		return false, err
	}
	p.logger.Debugf("SetParameters: % #X", resp.Data)
	return resp.Data[0] == command.SetParameters+1, nil
}

// ReadPassiveTarget 读卡 并返回读到的uid
func (p *Pn532) ReadPassiveTarget(cardBaud byte) ([]byte, error) {
	// 532最多一次可以识读2张卡 但如果是Jewel卡 一次只能读一张 因为读两张意义不大 所以这里直接写死一张
	if success, err := p.SendCommand([]byte{command.InListPassiveTarget, 0x01, cardBaud}); err != nil {
		return nil, err
	} else if !success {
		return nil, errors.New("send command failed")
	}

	resp, err := p.WaitInfoFrame()
	if err != nil {
		return nil, err
	}
	if resp.Data[1] != 0x01 {
		return nil, errors.New("more than one passive target detected")
	}
	if resp.Data[6] > 0x07 {
		return nil, errors.New("found card with unexpected long uid length")
	}
	return resp.Data[7 : 7+resp.Data[6]], nil
}

// InAutoPoll 读卡 并返回读到的uid
// PollNr specifies the number of polling (one polling is a polling for each Type j).
// period (0x01-0x0F) indicates the polling period in units of 150 ms.
// Type 1 indicates the mandatory target type to be polled at the 1st time.
func (p *Pn532) InAutoPoll(PollNr, Period byte, Type ...byte) ([]byte, error) { // 实际上可以考虑返回[][]byte
	if PollNr < 0x01 {
		return nil, errors.New("poll number must be greater than 0x01")
	}
	if Period < 0x01 || Period > 0x0F {
		return nil, errors.New("period must be between 0x01 and 0x0F")
	}
	if len(Type) > 254 {
		return nil, errors.New("type length must be less than 254")
	}
	if success, err := p.SendCommand(append([]byte{
		command.InAutoPoll,
		PollNr,
		Period,
		byte(len(Type))},
		Type...)); err != nil {
		return nil, err
	} else if !success {
		return nil, errors.New("send command failed")
	}

	resp, err := p.WaitInfoFrame()
	if err != nil {
		return nil, err
	}
	if resp.Data[0] != command.InAutoPoll+1 {
		return nil, errors.New("command resp error")
	}
	if resp.Data[1] != 0x01 {
		return nil, errors.New("more than one passive target detected")
	}
	return resp.Data[9 : 9+resp.Data[8]], nil
}

// MifareClassicAuthenticateBlock 验证区块密码  keyType 为设置验证A密码或B密码 blockNum为块号
func (p *Pn532) MifareClassicAuthenticateBlock(uid []byte, blockNum byte, keyType byte, key []byte) (bool, error) {
	if len(key) != 6 {
		log.Fatalf("key length must be 6")
	}
	if !(len(uid) >= 4 && len(uid) <= 7) {
		log.Fatalf("uid length must be more than 3 and less than 8")
	}
	if keyType != command.MifareCmdAuthA && keyType != command.MifareCmdAuthB {
		log.Fatalf("keyType must be 0x60 or 0x61")
	}
	cmd := []byte{
		command.InDataExchange,
		0x01, // 卡最多数量
		keyType,
		blockNum,
	}
	cmd = append(cmd, key...)
	cmd = append(cmd, uid...)

	if success, err := p.SendCommand(cmd); err != nil {
		return false, err
	} else if !success {
		return false, errors.New("send command failed")
	}
	resp, err := p.WaitInfoFrame()
	if err != nil {
		return false, err
	}
	if resp.Data[1] == 0x00 {
		return true, nil
	} else {
		return false, nil
	}
}

func (p *Pn532) MifareClassicReadBlock(blockNum byte) ([]byte, error) {
	if success, err := p.SendCommand([]byte{command.InDataExchange, 0x01, command.MifareCmdRead, blockNum}); err != nil {
		return nil, err
	} else if !success {
		return nil, errors.New("send command failed")
	}
	resp, err := p.WaitInfoFrame()
	if err != nil {
		return nil, err
	}
	if resp.Data[1] == 0x00 {
		return resp.Data[2:], nil
	} else {
		return nil, errors.New("read block failed")
	}
}

func (p *Pn532) MifareClassicWriteBlock(blockNum byte, data []byte) (bool, error) {
	if len(data) != 16 {
		log.Fatalf("data length must be 16")
	}
	if success, err := p.SendCommand(append([]byte{command.InDataExchange, 0x01, command.MifareCmdWrite, blockNum}, data...)); err != nil {
		return false, err
	} else if !success {
		return false, errors.New("send command failed")
	}
	resp, err := p.WaitInfoFrame()
	if err != nil {
		return false, err
	}
	if resp.Data[1] == 0x00 {
		return true, nil
	} else {
		return false, nil
	}
}
