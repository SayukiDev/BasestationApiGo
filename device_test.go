package BasestationAPI

import (
	"errors"
	"testing"
	"time"
)

func setConnectedFlag(t *testing.T, v bool) {
	t.Helper()
	deviceLock.Lock()
	defer deviceLock.Unlock()
	connectedFlag = v
}

// 制御対象が未設定なら実機に触れずに ErrNoDevice で失敗する。
func TestTryConnectWithoutTargets(t *testing.T) {
	resetState(t)
	if err := tryConnect(); !errors.Is(err, ErrNoDevice) {
		t.Fatalf("tryConnect() = %v, want ErrNoDevice", err)
	}
	deviceLock.Lock()
	defer deviceLock.Unlock()
	if connectedFlag {
		t.Error("接続失敗時に connectedFlag = true になっている")
	}
}

func TestTryConnectUpdatesLastActionTime(t *testing.T) {
	resetState(t)
	setConnectedFlag(t, true)

	before := time.Now()
	if err := tryConnect(); err != nil {
		t.Fatalf("tryConnect() = %v, want nil（接続済み）", err)
	}
	deviceLock.Lock()
	defer deviceLock.Unlock()
	if lastActionTime.Before(before) {
		t.Errorf("lastActionTime = %v が更新されていない", lastActionTime)
	}
}

func TestIdentifyWithoutTargets(t *testing.T) {
	resetState(t)
	if err := Identify("AA:BB:CC:DD:EE:FF"); !errors.Is(err, ErrNoDevice) {
		t.Fatalf("Identify() = %v, want ErrNoDevice", err)
	}
}

func TestIdentifyNotConnectedDevice(t *testing.T) {
	resetState(t)
	setConnected(t, "AA:BB:CC:DD:EE:01")
	setConnectedFlag(t, true) // tryConnect で実機に接続しにいかせない

	if err := Identify("AA:BB:CC:DD:EE:02"); !errors.Is(err, ErrDeviceNotFound) {
		t.Fatalf("Identify() = %v, want ErrDeviceNotFound", err)
	}
}

func TestSetPowerWithoutDevices(t *testing.T) {
	resetState(t)
	setConnectedFlag(t, true)

	for _, v := range []WriteablePwrStatus{PwrSleep, PwrStandby, PwrBooting} {
		if err := SetPower(v); err != nil {
			t.Errorf("SetPower(%v) = %v, want nil", v, err)
		}
	}
}

func TestGetPowerStateWithoutDevices(t *testing.T) {
	resetState(t)
	setConnectedFlag(t, true)

	got, err := GetPowerState()
	if err != nil {
		t.Fatalf("GetPowerState() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("GetPowerState() = nil, want 空 map")
	}
	if len(got) != 0 {
		t.Errorf("GetPowerState() = %v, want 空 map（接続デバイスなし）", got)
	}
}

func TestMatchPwrStatus(t *testing.T) {
	tests := []struct {
		in   byte
		want string
	}{
		{0x00, "SLEEP"},
		{0x01, "Booting"},
		{0x02, "STANDBY"},
		{0x0b, "ON"},
		{0x09, "Booting"},
	}
	for _, tt := range tests {
		if got := matchPwrStatus(tt.in); got != tt.want {
			t.Errorf("matchPwrStatus(0x%02x) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDisconnectClearsConnectedFlag(t *testing.T) {
	resetState(t)
	setConnectedFlag(t, true)

	if err := Disconnect(); err != nil {
		t.Fatalf("Disconnect() = %v, want nil", err)
	}
	deviceLock.Lock()
	defer deviceLock.Unlock()
	if connectedFlag {
		t.Error("Disconnect() 後も connectedFlag = true; 再接続されなくなる")
	}
}

func TestDisconnectWhenNotConnected(t *testing.T) {
	resetState(t)
	if err := Disconnect(); err != nil {
		t.Errorf("Disconnect() = %v, want nil", err)
	}
	deviceLock.Lock()
	defer deviceLock.Unlock()
	if connectedFlag {
		t.Error("connectedFlag = true, want false")
	}
}

// disconnectTick を同期的に走らせ、終了を待つ。
func runTick(t *testing.T, gen uint64) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		disconnectTick(gen)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnectTick が終了しない")
	}
}

func TestDisconnectTickIdleTimeout(t *testing.T) {
	resetState(t)
	disconnectCheckInterval = 5 * time.Millisecond
	idleTimeout = 20 * time.Millisecond
	deviceLock.Lock()
	connectedFlag = true
	lastActionTime = time.Now()
	tickGen = 1
	deviceLock.Unlock()

	runTick(t, 1)

	deviceLock.Lock()
	defer deviceLock.Unlock()
	if connectedFlag {
		t.Error("アイドルタイムアウト後も connectedFlag = true")
	}
}

func TestDisconnectTickKeepsAliveWhileActive(t *testing.T) {
	resetState(t)
	disconnectCheckInterval = 5 * time.Millisecond
	idleTimeout = time.Hour
	deviceLock.Lock()
	connectedFlag = true
	lastActionTime = time.Now()
	tickGen = 1
	deviceLock.Unlock()

	done := make(chan struct{})
	go func() {
		disconnectTick(1)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("操作直後なのに disconnectTick が終了した")
	default:
	}

	// Disconnect() で終了すること
	if err := Disconnect(); err != nil {
		t.Fatalf("Disconnect() = %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Disconnect() 後も disconnectTick が終了しない")
	}
}

// 古い世代の disconnectTick は、タイムアウト条件が成立していても切断せずに終了する。
func TestDisconnectTickStaleGenerationExits(t *testing.T) {
	resetState(t)
	disconnectCheckInterval = 5 * time.Millisecond
	idleTimeout = 0
	deviceLock.Lock()
	connectedFlag = true
	tickGen = 2
	deviceLock.Unlock()

	runTick(t, 1)

	deviceLock.Lock()
	defer deviceLock.Unlock()
	if !connectedFlag {
		t.Error("古い世代の disconnectTick が切断してしまった")
	}
}

func TestErrorSentinelsAreDistinct(t *testing.T) {
	errs := []error{ErrDeviceNotFound, ErrNoDevice, ErrAlreadyConnected, ErrServiceNotFound, ErrCharacteristicNotFound}
	for i := range errs {
		for j := range errs {
			if i != j && errors.Is(errs[i], errs[j]) {
				t.Errorf("%v と %v が同一のエラーになっている", errs[i], errs[j])
			}
		}
	}
}
