# pn532
通过串口与PN532通信的包 包含一些基础函数

因为基于 nfclib 的库并不方便使用，而市面上能够买到的 PN532 成品多是以串口的方式进行通信，所以我自己整了一个基于串口的库用来和 532 通信。

该库封装了一些基础的函数与命令，比如读写命令，状态命令，等等。具体使用方式可以参考 com_test.go，测试用例已基本覆盖常用功能。

## 安装

```go 
go get github.com/asjdf/pn532
```

## 快速开始
```go
package main

import (
	"github.com/asjdf/pn532"
	"log"
)

func main() {
	pn532.Mode = pn532.Release

	log.Print("初始化设备")
	device, err := pn532.QuickInit("COM4")
	if err != nil {
		log.Fatalf("初始化设备失败: %v", err)
	}
	log.Print("初始化成功")

	_, err = device.FirmwareVersion()
	if err != nil {
		log.Fatal(err)
	}
	
	log.Print("准备读取单张卡")
	uid, err := device.ReadPassiveTarget(pn532.ISO14443A)
	if err != nil {
		log.Fatalf("读取单张卡失败: %v", err)
	}
	log.Printf("读取单张卡成功 卡号: % X", uid)
}
```