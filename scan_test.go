package BasesationAPI

import (
	"errors"
	"sort"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

// グローバル状態を初期化する。パッケージ全体で共有しているため
// テストは並列実行しないこと。
func resetState(t *testing.T) {
	t.Helper()
	reset := func() {
		lock.Lock()
		devices = nil
		scanning = false
		scanned = false
		connected = nil
		lock.Unlock()

		deviceLock.Lock()
		deviceControl = nil
		connectedFlag = false
		lastActionTime = time.Time{}
		tickGen = 0
		deviceLock.Unlock()

		idleTimeout = 5 * time.Minute
		disconnectCheckInterval = 10 * time.Second
	}
	reset()
	t.Cleanup(reset)
}

func mustAddr(t *testing.T, s string) bluetooth.Address {
	t.Helper()
	if _, err := bluetooth.ParseMAC(s); err != nil {
		t.Fatalf("不正な MAC %q: %v", s, err)
	}
	var a bluetooth.Address
	a.Set(s)
	return a
}

func seedDevices(t *testing.T, infos ...DeviceInfo) {
	t.Helper()
	lock.Lock()
	defer lock.Unlock()
	devices = make(map[string]DeviceInfo, len(infos))
	for _, info := range infos {
		devices[info.Addr.String()] = info
	}
}

func setConnected(t *testing.T, addrs ...string) {
	t.Helper()
	lock.Lock()
	defer lock.Unlock()
	connected = make([]bluetooth.Device, 0, len(addrs))
	for _, a := range addrs {
		connected = append(connected, bluetooth.Device{Address: mustAddr(t, a)})
	}
}

func TestIsBaseStationName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"LHB-1A2B3C", true},
		{"lhb-1a2b3c", true},
		{"LhB-000000", true},
		{"LHB", false},
		{"LHBX", false},
		{"HTC BS 1A2B", false},
		{"", false},
		{"XLHB-1A2B3C", false},
	}
	for _, tt := range tests {
		if got := isBaseStationName(tt.name); got != tt.want {
			t.Errorf("isBaseStationName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestGetBaseStationEmpty(t *testing.T) {
	resetState(t)
	got := GetBaseStation()
	if got == nil {
		t.Fatal("GetBaseStation() = nil, want 空スライス")
	}
	if len(got) != 0 {
		t.Fatalf("GetBaseStation() = %v, want 空スライス", got)
	}
}

func TestGetBaseStationMapsFields(t *testing.T) {
	resetState(t)
	addr := mustAddr(t, "AA:BB:CC:DD:EE:01")
	seedDevices(t, DeviceInfo{Name: "LHB-1A2B3C", Addr: addr, RSSI: -42})

	got := GetBaseStation()
	if len(got) != 1 {
		t.Fatalf("len(GetBaseStation()) = %d, want 1", len(got))
	}
	want := PublicDeviceInfo{Name: "LHB-1A2B3C", Addr: addr.String(), RSSi: -42}
	if got[0] != want {
		t.Errorf("GetBaseStation()[0] = %+v, want %+v", got[0], want)
	}
}

func TestGetBaseStationFiltersNonBaseStation(t *testing.T) {
	resetState(t)
	seedDevices(t,
		DeviceInfo{Name: "LHB-000001", Addr: mustAddr(t, "AA:BB:CC:DD:EE:01"), RSSI: -40},
		DeviceInfo{Name: "lhb-000002", Addr: mustAddr(t, "AA:BB:CC:DD:EE:02"), RSSI: -50},
		DeviceInfo{Name: "Some Other Device", Addr: mustAddr(t, "AA:BB:CC:DD:EE:03"), RSSI: -60},
		DeviceInfo{Name: "LHBX", Addr: mustAddr(t, "AA:BB:CC:DD:EE:04"), RSSI: -40},
	)

	got := GetBaseStation()
	names := make([]string, 0, len(got))
	for _, d := range got {
		names = append(names, d.Name)
	}
	sort.Strings(names)
	want := []string{"LHB-000001", "lhb-000002"}
	if len(names) != len(want) {
		t.Fatalf("GetBaseStation() = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("GetBaseStation() = %v, want %v", names, want)
		}
	}
}

// 既にスキャン中なら Scanning() はアダプタに触れず即座に nil を返す。
func TestScanningWhileAlreadyScanning(t *testing.T) {
	resetState(t)
	lock.Lock()
	scanning = true
	lock.Unlock()

	done := make(chan error, 1)
	go func() { done <- Scanning() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Scanning() = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Scanning() がブロックした（スキャン中の二重呼び出しは即 return すべき）")
	}

	lock.RLock()
	defer lock.RUnlock()
	if !scanning {
		t.Error("二重呼び出しで scanning フラグが落ちた")
	}
	if devices != nil {
		t.Error("二重呼び出しでスキャン結果がクリアされた")
	}
}

func TestStopScanningWhenNotScanning(t *testing.T) {
	resetState(t)
	if err := StopScanning(); err != nil {
		t.Errorf("StopScanning() = %v, want nil（スキャンしていない場合は何もしない）", err)
	}
}

func TestSetDeviceControl(t *testing.T) {
	resetState(t)
	addrs := []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02"}
	SetDeviceControl(addrs)
	addrs[0] = "mutated" // 呼び出し側のスライスを書き換えても影響しないこと

	deviceLock.Lock()
	defer deviceLock.Unlock()
	want := []string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02"}
	if len(deviceControl) != len(want) {
		t.Fatalf("deviceControl = %v, want %v", deviceControl, want)
	}
	for i := range want {
		if deviceControl[i] != want[i] {
			t.Fatalf("deviceControl = %v, want %v", deviceControl, want)
		}
	}
}

func TestResolveAddressesUnscanned(t *testing.T) {
	resetState(t)
	addrs, errs := resolveAddresses([]string{"AA:BB:CC:DD:EE:01", "not-a-mac"})
	if len(addrs) != 1 || addrs[0].String() != mustAddr(t, "AA:BB:CC:DD:EE:01").String() {
		t.Errorf("addrs = %v, want [AA:BB:CC:DD:EE:01]", addrs)
	}
	if len(errs) != 1 {
		t.Errorf("errs = %v, want 不正な MAC のエラー 1 件", errs)
	}
}

func TestResolveAddressesScanned(t *testing.T) {
	resetState(t)
	found := mustAddr(t, "AA:BB:CC:DD:EE:01")
	seedDevices(t, DeviceInfo{Name: "LHB-000001", Addr: found})
	lock.Lock()
	scanned = true
	lock.Unlock()

	addrs, errs := resolveAddresses([]string{found.String(), "AA:BB:CC:DD:EE:FF"})
	if len(addrs) != 1 || addrs[0].String() != found.String() {
		t.Errorf("addrs = %v, want [%s]", addrs, found.String())
	}
	if len(errs) != 1 || !errors.Is(errs[0], ErrDeviceNotFound) {
		t.Errorf("errs = %v, want ErrDeviceNotFound 1 件", errs)
	}
}

func TestGetConnectedDevices(t *testing.T) {
	resetState(t)
	if got := getConnectedDevices(); len(got) != 0 {
		t.Fatalf("getConnectedDevices() = %v, want 空", got)
	}

	setConnected(t, "AA:BB:CC:DD:EE:01")
	want := mustAddr(t, "AA:BB:CC:DD:EE:01").String()
	got := getConnectedDevices()
	if len(got) != 1 || got[0].Address.String() != want {
		t.Fatalf("getConnectedDevices() = %v, want 1 件 (%s)", got, want)
	}

	// 返されたスライスを書き換えても内部状態に影響しないこと
	got[0] = bluetooth.Device{}
	if again := getConnectedDevices(); again[0].Address.String() != want {
		t.Error("getConnectedDevices() が内部スライスをそのまま返している")
	}
}

func TestGetConnectedDeviceOne(t *testing.T) {
	resetState(t)
	addr := mustAddr(t, "AA:BB:CC:DD:EE:01")
	setConnected(t, "AA:BB:CC:DD:EE:01")

	t.Run("found", func(t *testing.T) {
		d, ok := getConnectedDeviceOne(addr.String())
		if !ok {
			t.Fatalf("getConnectedDeviceOne(%q) = _, false, want true", addr.String())
		}
		if d.Address.String() != addr.String() {
			t.Errorf("Address = %s, want %s", d.Address.String(), addr.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, ok := getConnectedDeviceOne("AA:BB:CC:DD:EE:FF"); ok {
			t.Error("未接続のアドレスで ok = true になっている")
		}
	})
}

func TestDisconnectWithoutConnection(t *testing.T) {
	resetState(t)
	if err := disconnect(); err != nil {
		t.Errorf("disconnect() = %v, want nil", err)
	}
}

func TestConnectWithoutTargets(t *testing.T) {
	resetState(t)
	if err := connect(nil); !errors.Is(err, ErrNoDevice) {
		t.Errorf("connect(nil) = %v, want ErrNoDevice", err)
	}
	if got := getConnectedDevices(); len(got) != 0 {
		t.Errorf("getConnectedDevices() = %v, want 空", got)
	}
}

func TestConnectWhenAlreadyConnected(t *testing.T) {
	resetState(t)
	setConnected(t, "AA:BB:CC:DD:EE:01")
	if err := connect([]string{"AA:BB:CC:DD:EE:02"}); !errors.Is(err, ErrAlreadyConnected) {
		t.Errorf("connect() = %v, want ErrAlreadyConnected", err)
	}
}

// スキャン済みで制御対象がすべてスキャン結果に無い場合、
// アダプタに接続しにいかず ErrDeviceNotFound を返す。
func TestConnectAllTargetsUnknown(t *testing.T) {
	resetState(t)
	lock.Lock()
	scanned = true
	devices = map[string]DeviceInfo{}
	lock.Unlock()

	err := connect([]string{"AA:BB:CC:DD:EE:01", "AA:BB:CC:DD:EE:02"})
	if !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("connect() = %v, want ErrDeviceNotFound", err)
	}
	if got := getConnectedDevices(); len(got) != 0 {
		t.Errorf("getConnectedDevices() = %v, want 空", got)
	}
}
