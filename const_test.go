package BasestationAPI

import (
	"bytes"
	"strings"
	"testing"

	"tinygo.org/x/bluetooth"
)

// UUID の 16 バイト配列が意図した UUID 文字列になっているか（バイト順の検証）。
func TestUUIDString(t *testing.T) {
	tests := []struct {
		name string
		uuid bluetooth.UUID
		want string
	}{
		{"ServiceUUID", ServiceUUID, "00001523-1212-efde-1523-785feabcd124"},
		{"PwrCharacteristicUUID", PwrCharacteristicUUID, "00001525-1212-efde-1523-785feabcd124"},
		{"IdentifyCharacteristicUUID", IdentifyCharacteristicUUID, "00008421-1212-efde-1523-785feabcd124"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.uuid.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 3 つの UUID がすべて異なること（コピペ間違いの検出）。
func TestUUIDAreDistinct(t *testing.T) {
	uuids := map[string]bluetooth.UUID{
		"ServiceUUID":                ServiceUUID,
		"PwrCharacteristicUUID":      PwrCharacteristicUUID,
		"IdentifyCharacteristicUUID": IdentifyCharacteristicUUID,
	}
	seen := make(map[string]string, len(uuids))
	for name, uuid := range uuids {
		s := uuid.String()
		if prev, ok := seen[s]; ok {
			t.Errorf("%s と %s が同じ UUID %s になっている", prev, name, s)
		}
		seen[s] = name
	}
}

// キャラクタリスティックはサービスと同じベース UUID（下位 12 バイト）を共有する。
func TestUUIDShareSameBase(t *testing.T) {
	base := func(u bluetooth.UUID) string {
		s := u.String()
		return s[strings.Index(s, "-")+1:]
	}
	want := base(ServiceUUID)
	for name, uuid := range map[string]bluetooth.UUID{
		"PwrCharacteristicUUID":      PwrCharacteristicUUID,
		"IdentifyCharacteristicUUID": IdentifyCharacteristicUUID,
	} {
		if got := base(uuid); got != want {
			t.Errorf("%s のベース UUID = %s, want %s", name, got, want)
		}
	}
}

// 電源制御の書き込み値は 1 バイト固定。
func TestPwrValues(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		want []byte
	}{
		{"PwrSleep", PwrSleep.Bytes(), []byte{0x00}},
		{"PwrBooting", PwrBooting.Bytes(), []byte{0x01}},
		{"PwrStandby", PwrStandby.Bytes(), []byte{0x02}},
		{"PwrOn", PwrOn.Bytes(), []byte{0x0b}},
		{"identifyBytes", identifyBytes, []byte{0x01}},
	}
	for _, tt := range tests {
		if !bytes.Equal(tt.got, tt.want) {
			t.Errorf("%s = %#v, want %#v", tt.name, tt.got, tt.want)
		}
	}
}

// NamePrefix は strings.ToLower() した名前と比較する前提なので小文字でなければならない。
func TestNamePrefixIsLowerCase(t *testing.T) {
	if NamePrefix != strings.ToLower(NamePrefix) {
		t.Fatalf("NamePrefix = %q; ToLower した名前と比較しているため小文字である必要がある", NamePrefix)
	}
}

// スキャン時のフィルタ条件（scan.go の Scan コールバックと同じ式）の挙動。
func TestNamePrefixMatching(t *testing.T) {
	tests := []struct {
		localName string
		want      bool
	}{
		{"LHB-1A2B3C", true},
		{"lhb-1a2b3c", true},
		{"LhB-000000", true},
		{"LHB", false},
		{"HTC BS 1A2B", false},
		{"", false},
		{"XLHB-1A2B3C", false},
	}
	for _, tt := range tests {
		got := strings.HasPrefix(strings.ToLower(tt.localName), NamePrefix)
		if got != tt.want {
			t.Errorf("localName %q: マッチ = %v, want %v", tt.localName, got, tt.want)
		}
	}
}
