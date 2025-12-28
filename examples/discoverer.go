//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/grutz/gatt"
	"github.com/grutz/gatt/examples/option"
)

func onStateChanged(d gatt.Device, s gatt.State) {
	fmt.Println("State:", s)
	switch s {
	case gatt.StatePoweredOn:
		fmt.Println("scanning...")
		d.Scan(nil, false)
		return
	default:
		d.StopScanning()
	}
}

func onPeriphDiscovered(p gatt.Peripheral, a *gatt.Advertisement, rssi int) {
	fmt.Printf("\nPeripheral ID:%s, NAME:(%s)\n", p.ID(), p.Name())
	fmt.Println("  Local Name        =", a.LocalName)
	fmt.Println("  RSSI				 =", rssi)
	fmt.Println("  Manufacturer Data =", a.ManufacturerData)
	fmt.Println("  Service Data      =", a.ServiceData)
}

func main() {
	// Get device id, 0 or 1, from the command line arguments
	// and open device with that id
	devIdStr := os.Args[1]
	devId, err := strconv.Atoi(devIdStr)
	if err != nil {
		log.Fatalf("Invalid device id: %s\n", devIdStr)
		return
	}
	fmt.Println("Using device id:", devId)

	opt := option.DefaultClientOptions
	opt = append(opt, gatt.LnxDeviceID(devId, false))
	opt = append(opt, gatt.LnxSetScanMode(true)) // Passive scanning

	d, err := gatt.NewDevice(opt...)
	if err != nil {
		log.Fatalf("Failed to open device, err: %s\n", err)
		return
	}

	// Register handlers.
	d.Handle(gatt.PeripheralDiscovered(onPeriphDiscovered))
	d.Init(onStateChanged)
	select {}
}
