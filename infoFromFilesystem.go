package hid

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Hot-Fix for when *SOMETHING* broke with an update of Linux kernel on SkaarOS.
// The Something was information about Serial, Product and Manufacturer
// of USB devices. This information is now read from the filesystem instead of
// the HID API. This is a temporary fix until the underlying issue is resolved.
// The issue is that the HID API does not return the serial number, product and
// manufacturer for USB devices on SkaarOS with the latest kernel (6)
// - kasper

type usbInfo struct {
	Manufacturer string
	Product      string
	Serial       string
}

var (
	usbCache     = make(map[string]usbInfo) // key: Path
	usbCacheLock sync.RWMutex               // protects usbCache
)

func getUSBdeviceInfoFromFilesystem(info *DeviceInfo) {

	// Check cache under read lock
	usbCacheLock.RLock()
	cached, found := usbCache[info.Path]
	usbCacheLock.RUnlock()

	if found {
		info.Manufacturer = cached.Manufacturer
		info.Product = cached.Product
		info.Serial = cached.Serial
		return
	}

	// Set cache entry to empty under write lock
	fmt.Println("getUSBdeviceInfoFromFilesystem: Reading bus/dev from path:", info.Path)
	usbCacheLock.Lock()
	usbCache[info.Path] = usbInfo{}
	usbCacheLock.Unlock()

	// Not found in cache; proceed with filesystem scan
	busNum, devNum, ok := parseBusDevFromPath(info.Path)
	if !ok {
		fmt.Println("Could not parse bus/dev from path:", info.Path)
		return
	}

	entries, err := os.ReadDir("/sys/bus/usb/devices/")
	if err != nil {
		fmt.Println("Failed to read /sys/bus/usb/devices/:", err)
		return
	}

	for _, entry := range entries {
		devPath := filepath.Join("/sys/bus/usb/devices", entry.Name())

		bPath := filepath.Join(devPath, "busnum")
		dPath := filepath.Join(devPath, "devnum")

		bData, bErr := os.ReadFile(bPath)
		dData, dErr := os.ReadFile(dPath)

		if bErr != nil || dErr != nil {
			continue
		}

		bVal, _ := strconv.Atoi(strings.TrimSpace(string(bData)))
		dVal, _ := strconv.Atoi(strings.TrimSpace(string(dData)))

		if bVal == busNum && dVal == devNum {
			info.Manufacturer = readSysAttr(devPath, "manufacturer")
			info.Product = readSysAttr(devPath, "product")
			info.Serial = readSysAttr(devPath, "serial")

			// Save to cache under write lock
			usbCacheLock.Lock()
			usbCache[info.Path] = usbInfo{
				Manufacturer: info.Manufacturer,
				Product:      info.Product,
				Serial:       info.Serial,
			}
			usbCacheLock.Unlock()

			return
		}
	}
}

func parseBusDevFromPath(path string) (int, int, bool) {
	parts := strings.Split(path, ":")
	if len(parts) < 2 {
		return 0, 0, false
	}
	bus, err1 := strconv.ParseInt(parts[0], 16, 0)
	dev, err2 := strconv.ParseInt(parts[1], 16, 0)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return int(bus), int(dev), true
}

func readSysAttr(devPath, attr string) string {
	data, err := os.ReadFile(filepath.Join(devPath, attr))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
