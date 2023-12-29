package service

import (
	"github.com/grutz/gatt"
	"github.com/grutz/gatt/constants"
)

func NewBatteryService() *gatt.Service {
	lv := byte(100)
	s := gatt.NewService(constants.UUID16(0x180F))
	c := s.AddCharacteristic(constants.UUID16(0x2A19))
	c.HandleReadFunc(
		func(rsp gatt.ResponseWriter, req *gatt.ReadRequest) {
			rsp.Write([]byte{lv})
			lv--
		})

	// Characteristic User Description
	c.AddDescriptor(constants.UUID16(0x2901)).SetStringValue("Battery level between 0 and 100 percent")

	// Characteristic Presentation Format
	c.AddDescriptor(constants.UUID16(0x2904)).SetValue([]byte{4, 1, 39, 173, 1, 0, 0})

	return s
}
