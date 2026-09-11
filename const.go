package BasestationAPI

import (
	"tinygo.org/x/bluetooth"
)

var (
	// 00001523-1212-efde-1523-785feabcd124
	ServiceUUID = bluetooth.NewUUID([16]byte{0x00, 0x00, 0x15, 0x23, 0x12, 0x12, 0xef, 0xde, 0x15, 0x23, 0x78, 0x5f, 0xea, 0xbc, 0xd1, 0x24})

	// 00001525-1212-efde-1523-785feabcd124
	PwrCharacteristicUUID = bluetooth.NewUUID([16]byte{0x00, 0x00, 0x15, 0x25, 0x12, 0x12, 0xef, 0xde, 0x15, 0x23, 0x78, 0x5f, 0xea, 0xbc, 0xd1, 0x24})

	// 00008421-1212-efde-1523-785feabcd124
	IdentifyCharacteristicUUID = bluetooth.NewUUID([16]byte{0x00, 0x00, 0x84, 0x21, 0x12, 0x12, 0xef, 0xde, 0x15, 0x23, 0x78, 0x5f, 0xea, 0xbc, 0xd1, 0x24})
)

const (
	NamePrefix = "lhb-"
)

var (
	PwrSleep      = newWriteAblePwrStatus(0x00)
	PwrStandby    = newWriteAblePwrStatus(0x02)
	PwrBooting    = newWriteAblePwrStatus(0x01)
	PwrOn         = newPwrStatus(0x0b)
	identifyBytes = []byte{0x01}
)

type PwrStatus struct {
	body byte
}

type WriteablePwrStatus struct {
	PwrStatus
}

func newPwrStatus(b byte) PwrStatus {
	return PwrStatus{b}
}

func newWriteAblePwrStatus(b byte) WriteablePwrStatus {
	return WriteablePwrStatus{
		PwrStatus: newPwrStatus(b),
	}
}

func (p PwrStatus) Equal(b byte) bool {
	return p.body == b
}

func (p PwrStatus) Bytes() []byte {
	return []byte{p.body}
}
