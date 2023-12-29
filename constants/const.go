package constants

// This file includes constants from the BLE spec.

var (
	AttrGAPUUID  = UUID16(0x1800)
	AttrGATTUUID = UUID16(0x1801)

	AttrPrimaryServiceUUID   = UUID16(0x2800)
	AttrSecondaryServiceUUID = UUID16(0x2801)
	AttrIncludeUUID          = UUID16(0x2802)
	AttrCharacteristicUUID   = UUID16(0x2803)

	AttrClientCharacteristicConfigUUID = UUID16(0x2902)
	AttrServerCharacteristicConfigUUID = UUID16(0x2903)

	AttrDeviceNameUUID        = UUID16(0x2A00)
	AttrAppearanceUUID        = UUID16(0x2A01)
	AttrPeripheralPrivacyUUID = UUID16(0x2A02)
	AttrReconnectionAddrUUID  = UUID16(0x2A03)
	AttrPeferredParamsUUID    = UUID16(0x2A04)
	AttrServiceChangedUUID    = UUID16(0x2A05)
)

const (
	GATTCCCNotifyFlag   = 0x0001
	GATTCCCIndicateFlag = 0x0002
)

const (
	AttOpError              = 0x01
	AttOpMtuReq             = 0x02
	AttOpMtuRsp             = 0x03
	AttOpFindInfoReq        = 0x04
	AttOpFindInfoRsp        = 0x05
	AttOpFindByTypeValueReq = 0x06
	AttOpFindByTypeValueRsp = 0x07
	AttOpReadByTypeReq      = 0x08
	AttOpReadByTypeRsp      = 0x09
	AttOpReadReq            = 0x0a
	AttOpReadRsp            = 0x0b
	AttOpReadBlobReq        = 0x0c
	AttOpReadBlobRsp        = 0x0d
	AttOpReadMultiReq       = 0x0e
	AttOpReadMultiRsp       = 0x0f
	AttOpReadByGroupReq     = 0x10
	AttOpReadByGroupRsp     = 0x11
	AttOpWriteReq           = 0x12
	AttOpWriteRsp           = 0x13
	AttOpWriteCmd           = 0x52
	AttOpPrepWriteReq       = 0x16
	AttOpPrepWriteRsp       = 0x17
	AttOpExecWriteReq       = 0x18
	AttOpExecWriteRsp       = 0x19
	AttOpHandleNotify       = 0x1b
	AttOpHandleInd          = 0x1d
	AttOpHandleCnf          = 0x1e
	AttOpSignedWriteCmd     = 0xd2
)

type AttEcode byte

const (
	AttEcodeSuccess           AttEcode = 0x00 // Success
	AttEcodeInvalidHandle     AttEcode = 0x01 // The attribute handle given was not valid on this server.
	AttEcodeReadNotPerm       AttEcode = 0x02 // The attribute cannot be read.
	AttEcodeWriteNotPerm      AttEcode = 0x03 // The attribute cannot be written.
	AttEcodeInvalidPDU        AttEcode = 0x04 // The attribute PDU was invalid.
	AttEcodeAuthentication    AttEcode = 0x05 // The attribute requires authentication before it can be read or written.
	AttEcodeReqNotSupp        AttEcode = 0x06 // Attribute server does not support the request received from the client.
	AttEcodeInvalidOffset     AttEcode = 0x07 // Offset specified was past the end of the attribute.
	AttEcodeAuthorization     AttEcode = 0x08 // The attribute requires authorization before it can be read or written.
	AttEcodePrepQueueFull     AttEcode = 0x09 // Too many prepare writes have been queued.
	AttEcodeAttrNotFound      AttEcode = 0x0a // No attribute found within the given attribute handle range.
	AttEcodeAttrNotLong       AttEcode = 0x0b // The attribute cannot be read or written using the Read Blob Request.
	AttEcodeInsuffEncrKeySize AttEcode = 0x0c // The Encryption Key Size used for encrypting this link is insufficient.
	AttEcodeInvalAttrValueLen AttEcode = 0x0d // The attribute value length is invalid for the operation.
	AttEcodeUnlikely          AttEcode = 0x0e // The attribute request that was requested has encountered an error that was unlikely, and therefore could not be completed as requested.
	AttEcodeInsuffEnc         AttEcode = 0x0f // The attribute requires encryption before it can be read or written.
	AttEcodeUnsuppGrpType     AttEcode = 0x10 // The attribute type is not a supported grouping attribute as defined by a higher layer specification.
	AttEcodeInsuffResources   AttEcode = 0x11 // Insufficient Resources to complete the request.
)

func (a AttEcode) Error() string {
	switch i := int(a); {
	case i < 0x11:
		return AttEcodeName[a]
	case i >= 0x12 && i <= 0x7F: // Reserved for future use
		return "reserved error code"
	case i >= 0x80 && i <= 0x9F: // Application Error, defined by higher level
		return "reserved error code"
	case i >= 0xA0 && i <= 0xDF: // Reserved for future use
		return "reserved error code"
	case i >= 0xE0 && i <= 0xFF: // Common profile and service error codes
		return "profile or service error"
	default: // can't happen, just make compiler happy
		return "unknown error"
	}
}

var AttEcodeName = map[AttEcode]string{
	AttEcodeSuccess:           "success",
	AttEcodeInvalidHandle:     "invalid handle",
	AttEcodeReadNotPerm:       "read not permitted",
	AttEcodeWriteNotPerm:      "write not permitted",
	AttEcodeInvalidPDU:        "invalid PDU",
	AttEcodeAuthentication:    "insufficient authentication",
	AttEcodeReqNotSupp:        "request not supported",
	AttEcodeInvalidOffset:     "invalid offset",
	AttEcodeAuthorization:     "insufficient authorization",
	AttEcodePrepQueueFull:     "prepare queue full",
	AttEcodeAttrNotFound:      "attribute not found",
	AttEcodeAttrNotLong:       "attribute not long",
	AttEcodeInsuffEncrKeySize: "insufficient encryption key size",
	AttEcodeInvalAttrValueLen: "invalid attribute value length",
	AttEcodeUnlikely:          "unlikely error",
	AttEcodeInsuffEnc:         "insufficient encryption",
	AttEcodeUnsuppGrpType:     "unsupported group type",
	AttEcodeInsuffResources:   "insufficient resources",
}

func AttErrorRsp(op byte, h uint16, s AttEcode) []byte {
	return AttErr{Opcode: op, Attr: h, Status: s}.Marshal()
}

// attRspFor maps from att request
// codes to att response codes.
var AttRspFor = map[byte]byte{
	AttOpMtuReq:             AttOpMtuRsp,
	AttOpFindInfoReq:        AttOpFindInfoRsp,
	AttOpFindByTypeValueReq: AttOpFindByTypeValueRsp,
	AttOpReadByTypeReq:      AttOpReadByTypeRsp,
	AttOpReadReq:            AttOpReadRsp,
	AttOpReadBlobReq:        AttOpReadBlobRsp,
	AttOpReadMultiReq:       AttOpReadMultiRsp,
	AttOpReadByGroupReq:     AttOpReadByGroupRsp,
	AttOpWriteReq:           AttOpWriteRsp,
	AttOpPrepWriteReq:       AttOpPrepWriteRsp,
	AttOpExecWriteReq:       AttOpExecWriteRsp,
}

type AttErr struct {
	Opcode uint8
	Attr   uint16
	Status AttEcode
}

// TODO: Reformulate in a way that lets the caller avoid allocs.
// Accept a []byte? Write directly to an io.Writer?
func (e AttErr) Marshal() []byte {
	// little-endian encoding for Attr
	return []byte{AttOpError, e.Opcode, byte(e.Attr), byte(e.Attr >> 8), byte(e.Status)}
}

// EventType are Advertisement event types
type EventType uint8

func (e EventType) String() string {
	switch e {
	case AdvInd:
		return "ADV_IND"
	case AdvDirectInd:
		return "ADV_DIRECT_IND"
	case AdvScanInd:
		return "ADV_SCAN_IND"
	case AdvNonconnInd:
		return "ADV_NONCONN_IND"
	case ScanRsp:
		return "SCAN_RSP"
	default:
		return "Unknown"
	}
}

const (
	AdvInd        = 0x00 // Connectable undirected advertising (ADV_IND).
	AdvDirectInd  = 0x01 // Connectable directed advertising (ADV_DIRECT_IND)
	AdvScanInd    = 0x02 // Scannable undirected advertising (ADV_SCAN_IND)
	AdvNonconnInd = 0x03 // Non connectable undirected advertising (ADV_NONCONN_IND)
	ScanRsp       = 0x04 // Scan Response (SCAN_RSP)
)
