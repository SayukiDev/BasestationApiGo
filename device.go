package BasesationAPI

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

var (
	deviceLock     sync.Mutex
	deviceControl  []string
	lastActionTime time.Time
	connectedFlag  bool
	tickGen        uint64
)

var (
	idleTimeout             = 5 * time.Minute
	disconnectCheckInterval = 10 * time.Second
)

var (
	ErrServiceNotFound        = errors.New("service not found")
	ErrCharacteristicNotFound = errors.New("characteristic not found")
)

func SetDeviceControl(addrs []string) {
	deviceLock.Lock()
	defer deviceLock.Unlock()
	deviceControl = slices.Clone(addrs)
}

func getCharacteristic(device bluetooth.Device, service bluetooth.UUID, char bluetooth.UUID) (bluetooth.DeviceCharacteristic, error) {
	ss, err := device.DiscoverServices([]bluetooth.UUID{service})
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, err
	}
	if len(ss) == 0 {
		return bluetooth.DeviceCharacteristic{}, fmt.Errorf("%s: %w", service.String(), ErrServiceNotFound)
	}
	cs, err := ss[0].DiscoverCharacteristics([]bluetooth.UUID{char})
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, err
	}
	if len(cs) == 0 {
		return bluetooth.DeviceCharacteristic{}, fmt.Errorf("%s: %w", char.String(), ErrCharacteristicNotFound)
	}
	return cs[0], nil
}

func readValue(c bluetooth.DeviceCharacteristic, size int) ([]byte, error) {
	buf := make([]byte, size)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:min(n, len(buf))], nil
}

func tryConnect() error {
	deviceLock.Lock()
	defer deviceLock.Unlock()
	lastActionTime = time.Now()
	if connectedFlag {
		return nil
	}
	if err := connect(deviceControl); err != nil {
		return err
	}
	connectedFlag = true
	tickGen++
	go disconnectTick(tickGen)
	return nil
}

func matchPwrStatus(pwr byte) string {
	switch {
	case PwrOn.Equal(pwr):
		return "ON"
	case PwrStandby.Equal(pwr):
		return "STANDBY"
	case PwrSleep.Equal(pwr):
		return "SLEEP"
	case PwrBooting.Equal(pwr):
		return "Booting"
	default:
		return "Booting"
	}
}

func GetPowerState() (map[string]string, error) {
	if err := tryConnect(); err != nil {
		return nil, err
	}
	ds := getConnectedDevices()
	states := make(map[string]string, len(ds))
	var errs []error
	for _, d := range ds {
		addr := d.Address.String()
		c, err := getCharacteristic(d, ServiceUUID, PwrCharacteristicUUID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
			continue
		}
		val, err := readValue(c, 1)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
			continue
		}
		if len(val) == 0 {
			errs = append(errs, fmt.Errorf("%s: empty power value", addr))
			continue
		}
		states[addr] = matchPwrStatus(val[0])
	}
	return states, errors.Join(errs...)
}

func GetPowerStateSome(addrs ...string) (map[string]string, error) {
	if err := tryConnect(); err != nil {
		return nil, err
	}
	states := make(map[string]string, len(addrs))
	var errs []error
	for _, addr := range addrs {
		d, ok := getConnectedDeviceOne(addr)
		if !ok {
			errs = append(errs, fmt.Errorf("%s: %w", addr, ErrDeviceNotFound))
			continue
		}
		c, err := getCharacteristic(d, ServiceUUID, PwrCharacteristicUUID)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
			continue
		}
		val, err := readValue(c, 1)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
			continue
		}
		if len(val) == 0 {
			errs = append(errs, fmt.Errorf("%s: empty power value", addr))
			continue
		}
		states[addr] = matchPwrStatus(val[0])
	}
	return states, errors.Join(errs...)
}

func writePower(d bluetooth.Device, value WriteablePwrStatus) error {
	c, err := getCharacteristic(d, ServiceUUID, PwrCharacteristicUUID)
	if err != nil {
		return err
	}
	_, err = c.Write(value.Bytes())
	return err
}

func SetPower(value WriteablePwrStatus) error {
	if err := tryConnect(); err != nil {
		return err
	}
	var errs []error
	for _, d := range getConnectedDevices() {
		if err := writePower(d, value); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Address.String(), err))
		}
	}
	return errors.Join(errs...)
}

func SetPowerSome(value WriteablePwrStatus, addrs ...string) error {
	if err := tryConnect(); err != nil {
		return err
	}
	var errs []error
	for _, addr := range addrs {
		d, ok := getConnectedDeviceOne(addr)
		if !ok {
			errs = append(errs, fmt.Errorf("%s: %w", addr, ErrDeviceNotFound))
			continue
		}
		if err := writePower(d, value); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", addr, err))
		}
	}
	return errors.Join(errs...)
}

func Identify(addr string) error {
	if err := tryConnect(); err != nil {
		return err
	}
	d, ok := getConnectedDeviceOne(addr)
	if !ok {
		return fmt.Errorf("%s: %w", addr, ErrDeviceNotFound)
	}
	c, err := getCharacteristic(d, ServiceUUID, IdentifyCharacteristicUUID)
	if err != nil {
		return err
	}
	_, err = c.Write(identifyBytes)
	return err
}

func Disconnect() error {
	deviceLock.Lock()
	defer deviceLock.Unlock()
	if !connectedFlag {
		return nil
	}
	connectedFlag = false
	return disconnect()
}

func disconnectTick(gen uint64) {
	ticker := time.NewTicker(disconnectCheckInterval)
	defer ticker.Stop()
	for range ticker.C {
		deviceLock.Lock()
		if !connectedFlag || gen != tickGen {
			deviceLock.Unlock()
			return
		}
		if time.Since(lastActionTime) > idleTimeout {
			connectedFlag = false
			_ = disconnect()
			deviceLock.Unlock()
			return
		}
		deviceLock.Unlock()
	}
}
