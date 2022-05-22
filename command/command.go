package command

// 532 command
const (
	Diagnose           byte = 0x00
	GetFirmwareVersion byte = 0x02
	GetGeneralStatus   byte = 0x04
	ReadRegister       byte = 0x06
	WriteRegister      byte = 0x08
	ReadGPIO           byte = 0x0C
	WriteGPIO          byte = 0x0E
	SetSerialBaudRate  byte = 0x10
	SetParameters      byte = 0x12
	SAMConfiguration   byte = 0x14
	PowerDown          byte = 0x16

	RFConfiguration  byte = 0x32
	RFRegulationTest byte = 0x58

	InJumpForDEP        byte = 0x56
	InJumpForPSL        byte = 0x46
	InListPassiveTarget byte = 0x4A
	InATR               byte = 0x50
	InPSL               byte = 0x4E
	InDataExchange      byte = 0x40
	InCommunicateThru   byte = 0x42
	InDeselect          byte = 0x44
	InRelease           byte = 0x52
	InSelect            byte = 0x54
	InAutoPoll          byte = 0x60

	TgInitAsTarget        byte = 0x8C
	TgSetGeneralBytes     byte = 0x92
	TgGetData             byte = 0x86
	TgSetData             byte = 0x8E
	TgSetMetaData         byte = 0x94
	TgGetInitiatorCommand byte = 0x88
	TgResponseToInitiator byte = 0x90
	TgGetTargetStatus     byte = 0x8A
)

// defines the way of using the SAM (Security Access Module)
const (
	NormalMode      byte = 0x01 // the SAM is not used; this is the default mode
	VirtualCardMode byte = 0x02 // the couple PN532+SAM is seen as only one contactless SAM card from the external world
	WiredCardMode   byte = 0x03 // the host controller can access to the SAM with standard PCD commands
	DualCardMode    byte = 0x04 // both the PN532 and the SAM are visible from the external world as two separated targets
)

// Mifare command
const (
	MifareCmdAuthA     byte = 0x60
	MifareCmdAuthB     byte = 0x61
	MifareCmdRead      byte = 0x30
	MifareCmdWrite     byte = 0xA0
	MifareCmdTransfer  byte = 0xB0
	MifareCmdDecrement byte = 0xC0
	MifareCmdIncrement byte = 0xC1
	MifareCmdStore     byte = 0xC2
)

// ACKFrame The specific ACK frame is used for the synchronization of the packets
// and also for the abort mechanism. This frame may be used either from the host
// controller to the PN532 or from the PN532 to the host controller to indicate that
// the previous frame has been successfully received.
var ACKFrame = []byte{0x00, 0x00, 0xFF, 0x00, 0xFF, 0x00}

// NACKFrame The specific NACK frame is used for the synchronization of the packets.
// This frame is used only from the host controller to the PN532 to indicate that
// the previous response frame has not been successfully received, then asking for
// the retransmission of the last response frame from the PN532 to the host controller.
var NACKFrame = []byte{0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00}

// WakeUp When the host controller sends a command to the PN532 on the HSU link in order to
// exit from Power Down mode, the PN532 needs some delay to be fully operational.
// Either send a command with large preamble containing dummy data
// Or send first a 0x55 dummy byte and wait for the waking up delay before sending the command frame.
var WakeUp = []byte{0x55, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
