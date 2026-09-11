package BasestationAPI

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	adapter     = bluetooth.DefaultAdapter
	adapterOnce sync.Once
	adapterErr  error
)

func ensureAdapter() error {
	adapterOnce.Do(func() {
		adapterErr = adapter.Enable()
	})
	return adapterErr
}

type DeviceInfo struct {
	Name string
	Addr bluetooth.Address
	RSSI int16
}

var (
	devices            map[string]DeviceInfo
	lock               sync.RWMutex
	scanning           bool
	scanned            bool
	connected          []bluetooth.Device
	onScanDeviceUpdate = func(device DeviceInfo) {}
	onScanDeviceFailed = func(err error) {}
)

var (
	ErrDeviceNotFound   = errors.New("device not found")
	ErrNoDevice         = errors.New("no device to connect")
	ErrAlreadyConnected = errors.New("already connected")
)

func isBaseStationName(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), NamePrefix)
}

// Scanning is no synchronized.if you want to stop scanning, pls call StopScanning()
func Scanning() error {
	lock.Lock()
	if scanning {
		lock.Unlock()
		return nil
	}
	scanning = true
	devices = make(map[string]DeviceInfo, 4)
	lock.Unlock()

	if err := ensureAdapter(); err != nil {
		lock.Lock()
		scanning = false
		lock.Unlock()
		return err
	}

	go func() {
		defer func() {
			lock.Lock()
			scanning = false
			lock.Unlock()
		}()
		err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			if !isBaseStationName(result.LocalName()) {
				return
			}
			lock.Lock()
			d := DeviceInfo{
				Addr: result.Address,
				Name: result.LocalName(),
				RSSI: result.RSSI,
			}
			devices[result.Address.String()] = d
			lock.Unlock()
			onScanDeviceUpdate(d)
		})
		lock.Lock()
		scanned = true
		lock.Unlock()
		if err != nil {
			onScanDeviceFailed(err)
		}
	}()
	return nil
}

func ScanningWithTimeout(timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() { errCh <- Scanning() }()

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
	}
	if err := StopScanning(); err != nil {
		return err
	}
	return <-errCh
}

func StopScanning() error {
	lock.RLock()
	running := scanning
	lock.RUnlock()
	if !running {
		return nil
	}
	return adapter.StopScan()
}

func SetOnScanDeviceUpdate(f func(device DeviceInfo)) {
	lock.Lock()
	onScanDeviceUpdate = f
	lock.Unlock()
}

func SetOnScanDeviceFailed(f func(err error)) {
	lock.Lock()
	onScanDeviceFailed = f
	lock.Unlock()
}

type PublicDeviceInfo struct {
	Name string
	Addr string
	RSSi int16
}

func GetBaseStation() []PublicDeviceInfo {
	lock.RLock()
	defer lock.RUnlock()
	pds := make([]PublicDeviceInfo, 0, len(devices))
	for _, device := range devices {
		if !isBaseStationName(device.Name) {
			continue
		}
		pds = append(pds, PublicDeviceInfo{
			Addr: device.Addr.String(),
			Name: device.Name,
			RSSi: device.RSSI,
		})
	}
	return pds
}

func resolveAddresses(targets []string) ([]bluetooth.Address, []error) {
	lock.RLock()
	defer lock.RUnlock()
	addrs := make([]bluetooth.Address, 0, len(targets))
	var errs []error
	for _, target := range targets {
		if scanned {
			d, ok := devices[target]
			if !ok {
				errs = append(errs, fmt.Errorf("%s: %w", target, ErrDeviceNotFound))
				continue
			}
			addrs = append(addrs, d.Addr)
			continue
		}
		if _, err := bluetooth.ParseMAC(target); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", target, err))
			continue
		}
		var addr bluetooth.Address
		addr.Set(target)
		addrs = append(addrs, addr)
	}
	return addrs, errs
}

func connect(targets []string) error {
	if len(targets) == 0 {
		return ErrNoDevice
	}
	lock.RLock()
	already := len(connected) != 0
	lock.RUnlock()
	if already {
		return ErrAlreadyConnected
	}

	addrs, errs := resolveAddresses(targets)
	if len(addrs) == 0 {
		return errors.Join(errs...)
	}
	if err := ensureAdapter(); err != nil {
		return err
	}
	conns := make([]bluetooth.Device, 0, len(addrs))
	for _, addr := range addrs {
		cd, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr.String(), err))
			continue
		}
		conns = append(conns, cd)
	}
	if len(conns) == 0 {
		return errors.Join(errs...)
	}

	lock.Lock()
	connected = conns
	lock.Unlock()
	return nil
}

func getConnectedDevices() []bluetooth.Device {
	lock.RLock()
	defer lock.RUnlock()
	return slices.Clone(connected)
}

func getConnectedDeviceOne(addr string) (bluetooth.Device, bool) {
	lock.RLock()
	defer lock.RUnlock()
	for _, d := range connected {
		if d.Address.String() == addr {
			return d, true
		}
	}
	return bluetooth.Device{}, false
}

func disconnect() error {
	lock.Lock()
	conns := connected
	connected = nil
	lock.Unlock()

	var errs []error
	for _, device := range conns {
		if err := device.Disconnect(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", device.Address.String(), err))
		}
	}
	return errors.Join(errs...)
}
