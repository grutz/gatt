package linux

type packetType uint8

// HCI Packet types
const (
	typCommandPkt packetType = 0x01
	typACLDataPkt            = 0x02
	typSCODataPkt            = 0x03
	typEventPkt              = 0x04
	typVendorPkt             = 0xFF
)
