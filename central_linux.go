package gatt

import (
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/grutz/gatt/constants"
)

type security int

const (
	securityLow = iota
	securityMed
	securityHigh
)

type central struct {
	attrs       *attrRange
	mtu         uint16
	addr        net.HardwareAddr
	security    security
	l2conn      io.ReadWriteCloser
	notifiers   map[uint16]*notifier
	notifiersmu *sync.Mutex
}

func newCentral(a *attrRange, addr net.HardwareAddr, l2conn io.ReadWriteCloser) *central {
	return &central{
		attrs:       a,
		mtu:         23,
		addr:        addr,
		security:    securityLow,
		l2conn:      l2conn,
		notifiers:   make(map[uint16]*notifier),
		notifiersmu: &sync.Mutex{},
	}
}

func (c *central) ID() string {
	return c.addr.String()
}

func (c *central) Close() error {
	c.notifiersmu.Lock()
	defer c.notifiersmu.Unlock()
	for _, n := range c.notifiers {
		n.stop()
	}
	return c.l2conn.Close()
}

func (c *central) MTU() int {
	return int(c.mtu)
}

func (c *central) loop() {
	for {
		// L2CAP implementations shall support a minimum MTU size of 48 bytes.
		// The default value is 672 bytes
		b := make([]byte, 672)
		n, err := c.l2conn.Read(b)
		if n == 0 || err != nil {
			c.Close()
			break
		}
		if rsp := c.handleReq(b[:n]); rsp != nil {
			c.l2conn.Write(rsp)
		}
	}
}

// handleReq dispatches a raw request from the central shim
// to an appropriate handler, based on its type.
// It panics if len(b) == 0.
func (c *central) handleReq(b []byte) []byte {
	var resp []byte
	switch reqType, req := b[0], b[1:]; reqType {
	case constants.AttOpMtuReq:
		resp = c.handleMTU(req)
	case constants.AttOpFindInfoReq:
		resp = c.handleFindInfo(req)
	case constants.AttOpFindByTypeValueReq:
		resp = c.handleFindByTypeValue(req)
	case constants.AttOpReadByTypeReq:
		resp = c.handleReadByType(req)
	case constants.AttOpReadReq:
		resp = c.handleRead(req)
	case constants.AttOpReadBlobReq:
		resp = c.handleReadBlob(req)
	case constants.AttOpReadByGroupReq:
		resp = c.handleReadByGroup(req)
	case constants.AttOpWriteReq, constants.AttOpWriteCmd:
		resp = c.handleWrite(reqType, req)
	case constants.AttOpReadMultiReq, constants.AttOpPrepWriteReq, constants.AttOpExecWriteReq, constants.AttOpSignedWriteCmd:
		fallthrough
	default:
		resp = constants.AttErrorRsp(reqType, 0x0000, constants.AttEcodeReqNotSupp)
	}
	return resp
}

func (c *central) handleMTU(b []byte) []byte {
	c.mtu = binary.LittleEndian.Uint16(b[:2])
	if c.mtu < 23 {
		c.mtu = 23
	}
	if c.mtu >= 256 {
		c.mtu = 256
	}
	return []byte{constants.AttOpMtuRsp, uint8(c.mtu), uint8(c.mtu >> 8)}
}

// REQ: FindInfoReq(0x04), StartHandle, EndHandle
// RSP: FindInfoRsp(0x05), UUIDFormat, Handle, UUID, Handle, UUID, ...
func (c *central) handleFindInfo(b []byte) []byte {
	start, end := readHandleRange(b[:4])

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(constants.AttOpFindInfoRsp)

	uuidLen := -1
	for _, a := range c.attrs.Subrange(start, end) {
		if uuidLen == -1 {
			uuidLen = a.typ.Len()
			if uuidLen == 2 {
				w.WriteByteFit(0x01) // TODO: constants for 16bit vs 128bit uuid magic numbers here
			} else {
				w.WriteByteFit(0x02)
			}
		}
		if a.typ.Len() != uuidLen {
			break
		}
		w.Chunk()
		w.WriteUint16Fit(a.h)
		w.WriteUUIDFit(a.typ)
		if ok := w.Commit(); !ok {
			break
		}
	}

	if uuidLen == -1 {
		return constants.AttErrorRsp(constants.AttOpFindInfoReq, start, constants.AttEcodeAttrNotFound)
	}
	return w.Bytes()
}

// REQ: FindByTypeValueReq(0x06), StartHandle, EndHandle, Type(UUID), Value
// RSP: FindByTypeValueRsp(0x07), AttrHandle, GroupEndHandle, AttrHandle, GroupEndHandle, ...
func (c *central) handleFindByTypeValue(b []byte) []byte {
	start, end := readHandleRange(b[:4])
	t := constants.UUID{b[4:6]}
	u := constants.UUID{b[6:]}

	// Only support the ATT ReadByGroupReq for GATT Primary Service Discovery.
	// More sepcifically, the "Discover Primary Services By Service UUID" sub-procedure
	if !t.Equal(constants.AttrPrimaryServiceUUID) {
		return constants.AttErrorRsp(constants.AttOpFindByTypeValueReq, start, constants.AttEcodeAttrNotFound)
	}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(constants.AttOpFindByTypeValueRsp)

	var wrote bool
	for _, a := range c.attrs.Subrange(start, end) {
		if !a.typ.Equal(constants.AttrPrimaryServiceUUID) {
			continue
		}
		if !(constants.UUID{a.value}.Equal(u)) {
			continue
		}
		s := a.pvt.(*Service)
		w.Chunk()
		w.WriteUint16Fit(s.h)
		w.WriteUint16Fit(s.endh)
		if ok := w.Commit(); !ok {
			break
		}
		wrote = true
	}
	if !wrote {
		return constants.AttErrorRsp(constants.AttOpFindByTypeValueReq, start, constants.AttEcodeAttrNotFound)
	}

	return w.Bytes()
}

// REQ: ReadByType(0x08), StartHandle, EndHandle, Type(UUID)
// RSP: ReadByType(0x09), LenOfEachDataField, DataField, DataField, ...
func (c *central) handleReadByType(b []byte) []byte {
	start, end := readHandleRange(b[:4])
	t := constants.UUID{b[4:]}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(constants.AttOpReadByTypeRsp)
	uuidLen := -1
	for _, a := range c.attrs.Subrange(start, end) {
		if !a.typ.Equal(t) {
			continue
		}
		if (a.secure&CharRead) != 0 && c.security > securityLow {
			return constants.AttErrorRsp(constants.AttOpReadByTypeReq, start, constants.AttEcodeAuthentication)
		}
		v := a.value
		if v == nil {
			rsp := newResponseWriter(int(c.mtu - 1))
			req := &ReadRequest{
				Request: Request{Central: c},
				Cap:     int(c.mtu - 1),
				Offset:  0,
			}
			if c, ok := a.pvt.(*Characteristic); ok {
				c.rhandler.ServeRead(rsp, req)
			} else if d, ok := a.pvt.(*Descriptor); ok {
				d.rhandler.ServeRead(rsp, req)
			}
			v = rsp.bytes()
		}
		if uuidLen == -1 {
			uuidLen = len(v)
			w.WriteByteFit(byte(uuidLen) + 2)
		}
		if len(v) != uuidLen {
			break
		}
		w.Chunk()
		w.WriteUint16Fit(a.h)
		w.WriteFit(v)
		if ok := w.Commit(); !ok {
			break
		}
	}
	if uuidLen == -1 {
		return constants.AttErrorRsp(constants.AttOpReadByTypeReq, start, constants.AttEcodeAttrNotFound)
	}
	return w.Bytes()
}

// REQ: ReadReq(0x0A), Handle
// RSP: ReadRsp(0x0B), Value
func (c *central) handleRead(b []byte) []byte {
	h := binary.LittleEndian.Uint16(b)
	a, ok := c.attrs.At(h)
	if !ok {
		return constants.AttErrorRsp(constants.AttOpReadReq, h, constants.AttEcodeInvalidHandle)
	}
	if a.props&CharRead == 0 {
		return constants.AttErrorRsp(constants.AttOpReadReq, h, constants.AttEcodeReadNotPerm)
	}
	if a.secure&CharRead != 0 && c.security > securityLow {
		return constants.AttErrorRsp(constants.AttOpReadReq, h, constants.AttEcodeAuthentication)
	}
	v := a.value
	if v == nil {
		req := &ReadRequest{
			Request: Request{Central: c},
			Cap:     int(c.mtu - 1),
			Offset:  0,
		}
		rsp := newResponseWriter(int(c.mtu - 1))
		if c, ok := a.pvt.(*Characteristic); ok {
			c.rhandler.ServeRead(rsp, req)
		} else if d, ok := a.pvt.(*Descriptor); ok {
			d.rhandler.ServeRead(rsp, req)
		}
		v = rsp.bytes()
	}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(constants.AttOpReadRsp)
	w.Chunk()
	w.WriteFit(v)
	w.CommitFit()
	return w.Bytes()
}

// FIXME: check this, untested, might be broken
func (c *central) handleReadBlob(b []byte) []byte {
	h := binary.LittleEndian.Uint16(b)
	offset := binary.LittleEndian.Uint16(b[2:])
	a, ok := c.attrs.At(h)
	if !ok {
		return constants.AttErrorRsp(constants.AttOpReadBlobReq, h, constants.AttEcodeInvalidHandle)
	}
	if a.props&CharRead == 0 {
		return constants.AttErrorRsp(constants.AttOpReadBlobReq, h, constants.AttEcodeReadNotPerm)
	}
	if a.secure&CharRead != 0 && c.security > securityLow {
		return constants.AttErrorRsp(constants.AttOpReadBlobReq, h, constants.AttEcodeAuthentication)
	}
	v := a.value
	if v == nil {
		req := &ReadRequest{
			Request: Request{Central: c},
			Cap:     int(c.mtu - 1),
			Offset:  int(offset),
		}
		rsp := newResponseWriter(int(c.mtu - 1))
		if c, ok := a.pvt.(*Characteristic); ok {
			c.rhandler.ServeRead(rsp, req)
		} else if d, ok := a.pvt.(*Descriptor); ok {
			d.rhandler.ServeRead(rsp, req)
		}
		v = rsp.bytes()
		offset = 0 // the server has already adjusted for the offset
	}
	w := newL2capWriter(c.mtu)
	w.WriteByteFit(constants.AttOpReadBlobRsp)
	w.Chunk()
	w.WriteFit(v)
	if ok := w.ChunkSeek(offset); !ok {
		return constants.AttErrorRsp(constants.AttOpReadBlobReq, h, constants.AttEcodeInvalidOffset)
	}
	w.CommitFit()
	return w.Bytes()
}

func (c *central) handleReadByGroup(b []byte) []byte {
	start, end := readHandleRange(b)
	t := constants.UUID{b[4:]}

	// Only support the ATT ReadByGroupReq for GATT Primary Service Discovery.
	// More specifically, the "Discover All Primary Services" sub-procedure.
	if !t.Equal(constants.AttrPrimaryServiceUUID) {
		return constants.AttErrorRsp(constants.AttOpReadByGroupReq, start, constants.AttEcodeUnsuppGrpType)
	}

	w := newL2capWriter(c.mtu)
	w.WriteByteFit(constants.AttOpReadByGroupRsp)
	uuidLen := -1
	for _, a := range c.attrs.Subrange(start, end) {
		if !a.typ.Equal(constants.AttrPrimaryServiceUUID) {
			continue
		}
		if uuidLen == -1 {
			uuidLen = len(a.value)
			w.WriteByteFit(byte(uuidLen + 4))
		}
		if uuidLen != len(a.value) {
			break
		}
		s := a.pvt.(*Service)
		w.Chunk()
		w.WriteUint16Fit(s.h)
		w.WriteUint16Fit(s.endh)
		w.WriteFit(a.value)
		if ok := w.Commit(); !ok {
			break
		}
	}
	if uuidLen == -1 {
		return constants.AttErrorRsp(constants.AttOpReadByGroupReq, start, constants.AttEcodeAttrNotFound)
	}
	return w.Bytes()
}

func (c *central) handleWrite(reqType byte, b []byte) []byte {
	h := binary.LittleEndian.Uint16(b[:2])
	value := b[2:]

	a, ok := c.attrs.At(h)
	if !ok {
		return constants.AttErrorRsp(reqType, h, constants.AttEcodeInvalidHandle)
	}

	noRsp := reqType == constants.AttOpWriteCmd
	charFlag := CharWrite
	if noRsp {
		charFlag = CharWriteNR
	}
	if a.props&charFlag == 0 {
		return constants.AttErrorRsp(reqType, h, constants.AttEcodeWriteNotPerm)
	}
	if a.secure&charFlag == 0 && c.security > securityLow {
		return constants.AttErrorRsp(reqType, h, constants.AttEcodeAuthentication)
	}

	// Props of Service and Characteristic declration are read only.
	// So we only need deal with writable descriptors here.
	// (Characteristic's value is implemented with descriptor)
	if !a.typ.Equal(constants.AttrClientCharacteristicConfigUUID) {
		// Regular write, not CCC
		r := Request{Central: c}
		result := byte(0)
		if c, ok := a.pvt.(*Characteristic); ok {
			result = c.whandler.ServeWrite(r, value)
		} else if d, ok := a.pvt.(*Characteristic); ok {
			result = d.whandler.ServeWrite(r, value)
		}
		if noRsp {
			return nil
		} else {
			resultEcode := constants.AttEcode(result)
			if resultEcode == constants.AttEcodeSuccess {
				return []byte{constants.AttOpWriteRsp}
			} else {
				return constants.AttErrorRsp(reqType, h, resultEcode)
			}
		}
	}

	// CCC/descriptor write
	if len(value) != 2 {
		return constants.AttErrorRsp(reqType, h, constants.AttEcodeInvalAttrValueLen)
	}
	ccc := binary.LittleEndian.Uint16(value)
	// char := a.pvt.(*Descriptor).char
	if ccc&(constants.GATTCCCNotifyFlag|constants.GATTCCCIndicateFlag) != 0 {
		c.startNotify(&a, int(c.mtu-3))
	} else {
		c.stopNotify(&a)
	}
	if noRsp {
		return nil
	}
	return []byte{constants.AttOpWriteRsp}
}

func (c *central) sendNotification(a *attr, data []byte) (int, error) {
	w := newL2capWriter(c.mtu)
	added := 0
	if w.WriteByteFit(constants.AttOpHandleNotify) {
		added += 1
	}
	if w.WriteUint16Fit(a.pvt.(*Descriptor).char.vh) {
		added += 2
	}
	w.WriteFit(data)
	n, err := c.l2conn.Write(w.Bytes())
	if err != nil {
		return n, err
	}
	return n - added, err
}

func readHandleRange(b []byte) (start, end uint16) {
	return binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[2:])
}

func (c *central) startNotify(a *attr, maxlen int) {
	c.notifiersmu.Lock()
	defer c.notifiersmu.Unlock()
	if _, found := c.notifiers[a.h]; found {
		return
	}
	char := a.pvt.(*Descriptor).char
	n := newNotifier(c, a, maxlen)
	c.notifiers[a.h] = n
	go char.nhandler.ServeNotify(Request{Central: c}, n)
}

func (c *central) stopNotify(a *attr) {
	c.notifiersmu.Lock()
	defer c.notifiersmu.Unlock()
	// char := a.pvt.(*Characteristic)
	if n, found := c.notifiers[a.h]; found {
		n.stop()
		delete(c.notifiers, a.h)
	}
}
