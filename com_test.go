package pn532

import (
	"bytes"
	"github.com/asjdf/pn532/command"
	"go.bug.st/serial"
	"log"
	"testing"
)

const (
	testCom = "COM4"
)

func TestPn532_ReadFrame(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(ports) == 0 {
		log.Println("no device, skip test")
		t.SkipNow()
		return
	}
	device, err := Init(ports[0])
	if err != nil {
		log.Fatal(err)
	}
	err = device.WriteFrame([]byte{0x55, 0x55, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x03, 0xfd, 0xd4, 0x14, 0x01, 0x17, 0x00})
	if err != nil {
		log.Fatal(err)
	}
	_, err = device.WaitInfoFrame()
	if err != nil {
		log.Fatal(err)
	}
	log.Print("read frame ok")
}

func TestPn532_FirmwareVersion(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(ports) == 0 {
		log.Println("no device, skip test")
		t.SkipNow()
		return
	}
	device, err := Init(ports[0])
	if err != nil {
		log.Fatal(err)
	}
	v, err := device.FirmwareVersion()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("firmware version data: % #X", v)
}

func TestPn532_ReadPassiveTarget(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(ports) == 0 {
		log.Println("no device, skip test")
		t.SkipNow()
		return
	}
	device, err := QuickInit(ports[0])
	if err != nil {
		log.Fatal(err)
	}

	target, err := device.ReadPassiveTarget(ISO14443A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("target: %#v", target)
}

func TestPn532_MifareClassicAuthenticateBlock(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(ports) == 0 {
		log.Println("no device, skip test")
		t.SkipNow()
		return
	}
	device, err := QuickInit(ports[0])
	if err != nil {
		log.Fatal(err)
	}

	UID, err := device.ReadPassiveTarget(ISO14443A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("target: %#v", UID)

	success, err := device.MifareClassicAuthenticateBlock(UID, 0x3A, command.MifareCmdAuthB, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	if err != nil {
		return
	}
	if success {
		log.Print("authenticate success")
	} else {
		log.Print("authenticate failed")
	}
}

func TestPn532_MifareClassicReadBlock(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(ports) == 0 {
		log.Println("no device, skip test")
		t.SkipNow()
		return
	}
	device, err := QuickInit(ports[0])
	if err != nil {
		log.Fatal(err)
	}

	UID, err := device.ReadPassiveTarget(ISO14443A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("target: %#v", UID)

	success, err := device.MifareClassicAuthenticateBlock(UID, 0x3A, command.MifareCmdAuthB, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	if err != nil {
		return
	}
	if success {
		log.Print("authenticate success")
	} else {
		log.Print("authenticate failed")
	}
	block, err := device.MifareClassicReadBlock(0x3A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("block: % X", block)
}

func TestPn532_MifareClassicWriteBlock(t *testing.T) {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
		return
	}
	if len(ports) == 0 {
		log.Println("no device, skip test")
		t.SkipNow()
		return
	}
	device, err := QuickInit(ports[0])
	if err != nil {
		log.Fatal(err)
	}

	UID, err := device.ReadPassiveTarget(ISO14443A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("target: %#v", UID)

	success, err := device.MifareClassicAuthenticateBlock(UID, 0x3A, command.MifareCmdAuthB, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	if err != nil {
		return
	}
	if success {
		log.Print("authenticate success")
	} else {
		log.Print("authenticate failed")
	}
	block, err := device.MifareClassicReadBlock(0x3A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("block: % X", block)

	testBlock := []byte{0x11, 0x45, 0x14, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	success, err = device.MifareClassicWriteBlock(0x3A, testBlock)
	if err != nil {
		log.Fatal(err)
	} else if !success {
		log.Fatal("write block failed")
	}

	blockNew, err := device.MifareClassicReadBlock(0x3A)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("blockNew: % X", blockNew)

	if !bytes.Equal(testBlock, blockNew) {
		log.Fatal("verify block failed")
	}
	log.Print("verify block success")

	log.Print("rolling back")
	success, err = device.MifareClassicWriteBlock(0x3A, block)
	if err != nil {
		log.Fatal(err)
	} else if !success {
		log.Fatal("write block failed")
	}
}
