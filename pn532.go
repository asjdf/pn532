package pn532

import (
	"bytes"
	"context"
	"errors"
	"github.com/asjdf/pn532/command"
	"github.com/tarm/serial"
	"log"
	"strings"
	"time"
)

const (
	Debug   = "DEBUG"
	Release = "RELEASE"

	ISO14443A = 0x00 // 卡片类型
)

var Mode string // running mode

func init() {
	Mode = Debug // default mode
}

type Pn532 struct {
	port   *serial.Port
	buf    *bytes.Buffer
	wakeup bool
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
	return InitWithConf(&serial.Config{Name: port, Baud: 115200})
}

func InitWithConf(conf *serial.Config) (*Pn532, error) {
	p, err := serial.OpenPort(conf)
	if err != nil {
		return nil, err
	}
	return &Pn532{port: p, buf: bytes.NewBuffer(nil)}, nil
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
	if Mode == Debug {
		log.Printf("write: % #x", frame)
	}
	_, err := p.port.Write(frame)
	return err
}

// ReadFrame 从串口读包 先从缓存区读 看看是不是还有包没解完 再从串口读
func (p *Pn532) ReadFrame(minLen int) ([]byte, error) {
	buf := make([]byte, 0, 512)
	totalLen := 0
	t := make([]byte, 512)
	n, _ := p.buf.Read(t)
	buf = append(buf, t[:n]...)
	totalLen += n

	if totalLen >= minLen {
		if Mode == Debug {
			log.Printf("read: % #x", buf[:totalLen])
		}
		return buf[:totalLen], nil
	}

	var err error
	var tmpLen int
	tmp := make([]byte, 512)
	tmpLen, err = p.port.Read(tmp)
	if err != nil {
		return nil, err
	}
	buf = append(buf, tmp[:tmpLen]...)
	totalLen += tmpLen

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	go func() {
		for totalLen < minLen {
			tmpLen, err = p.port.Read(tmp)
			if err != nil {
				return
			}
			buf = append(buf, tmp[:tmpLen]...)
			totalLen += tmpLen
		}
		cancel()
	}()

	select {
	case <-ctx.Done():
		if err != nil {
			return nil, err
		}
	case <-time.After(50 * time.Millisecond):
		cancel()
		return nil, errors.New("timeout")
	}

	if Mode == Debug {
		log.Printf("read: % #x", buf[:totalLen])
	}
	return buf[:totalLen], nil
}

// SendCommand 发送命令至pn532 如响应 返回true 否则返回false
func (p *Pn532) SendCommand(data []byte) (bool, error) {
	if err := p.WriteFrame(data); err != nil {
		return false, err
	}
	resp, err := p.ReadFrame(6)
	if err != nil {
		return false, err
	}
	if bytes.Equal(resp[:6], command.ACKFrame) {
		p.buf.Write(resp[6:])
		if Mode == Debug {
			log.Printf("send command success")
		}
		return true, nil
	} else if bytes.Equal(resp[:6], command.NACKFrame) {
		p.buf.Write(resp[6:])
		if Mode == Debug {
			log.Printf("send command failed")
		}
		return false, nil
	} else {
		p.buf.Write(resp)
		return false, errors.New("response error")
	}
}

func (p *Pn532) WaitInfoFrame() (*InfoFrame, error) {
	resp, err := p.ReadFrame(8)
	if err != nil {
		return nil, err
	}
	i, err := Decode(resp)
	if err != nil {
		return nil, err
	}
	p.buf.Write(resp[len(i.Data)+8:])
	return i, nil
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
	if Mode == Debug {
		if resp.Data[1] == 0x32 {
			log.Print("IC: 532")
		} else {
			log.Print("IC: unknown")
		}
		log.Printf("Ver: %d", resp.Data[2])
		log.Printf("Rev: %d", resp.Data[3])
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
		log.Printf("Support: %s", strings.Join(support, ","))
	}
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
	if Mode == Debug {
		log.Printf("SAMConfiguration: % #x", resp.Data)
	}
	return resp.Data[0] == 0x15, nil
}

// ReadPassiveTarget 读卡 并返回读到的uid
func (p *Pn532) ReadPassiveTarget(cardBaud byte) ([]byte, error) {
	// 532最多一次可以识读2张卡 但如果是Jewel卡 一次只能读一张 因为读两张意义不大 所以这里直接写死一张
	if success, err := p.SendCommand([]byte{command.InListPassiveTarget, 0x01, cardBaud}); err != nil {
		return nil, err
	} else if !success {
		return nil, errors.New("send command failed")
	}

	//EE2725E5
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
