package protocol

import (
	"bytes"
	"fmt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/nbt"
	"image/color"
	"io"
	"reflect"
	"unsafe"
)

// Writer implements writing methods for data types from Minecraft packets. Each Packet implementation has one
// passed to it when writing.
// Writer implements methods where values are passed using a pointer, so that Reader and Writer have a
// synonymous interface and both implement the IO interface.
type Writer struct {
	w interface {
		io.Writer
		io.ByteWriter
	}
	shieldID int32
}

// NewWriter creates a new initialised Writer with an underlying io.ByteWriter to write to.
func NewWriter(w interface {
	io.Writer
	io.ByteWriter
}, shieldID int32) *Writer {
	return &Writer{w: w, shieldID: shieldID}
}

// Uint8 writes a uint8 to the underlying buffer.
func (w *Writer) Uint8(x *uint8) {
	_ = w.w.WriteByte(*x)
}

// Bool writes a bool as either 0 or 1 to the underlying buffer.
func (w *Writer) Bool(x *bool) {
	_ = w.w.WriteByte(*(*byte)(unsafe.Pointer(x)))
}

// StringUTF ...
func (w *Writer) StringUTF(x *string) {
	l := int16(len(*x))
	w.Int16(&l)
	_, _ = w.w.Write([]byte(*x))
}

// String writes a string, prefixed with a varuint32, to the underlying buffer.
func (w *Writer) String(x *string) {
	l := uint32(len(*x))
	w.Varuint32(&l)
	_, _ = w.w.Write([]byte(*x))
}

// ByteSlice writes a []byte, prefixed with a varuint32, to the underlying buffer.
func (w *Writer) ByteSlice(x *[]byte) {
	l := uint32(len(*x))
	w.Varuint32(&l)
	_, _ = w.w.Write(*x)
}

// Bytes appends a []byte to the underlying buffer.
func (w *Writer) Bytes(x *[]byte) {
	_, _ = w.w.Write(*x)
}

// ByteFloat writes a rotational float32 as a single byte to the underlying buffer.
func (w *Writer) ByteFloat(x *float32) {
	_ = w.w.WriteByte(byte(*x / (360.0 / 256.0)))
}

// Vec3 writes an mgl32.Vec3 as 3 float32s to the underlying buffer.
func (w *Writer) Vec3(x *mgl32.Vec3) {
	w.Float32(&x[0])
	w.Float32(&x[1])
	w.Float32(&x[2])
}

// Vec2 writes an mgl32.Vec2 as 2 float32s to the underlying buffer.
func (w *Writer) Vec2(x *mgl32.Vec2) {
	w.Float32(&x[0])
	w.Float32(&x[1])
}

// BlockPos writes a BlockPos as 3 varint32s to the underlying buffer.
func (w *Writer) BlockPos(x *BlockPos) {
	w.Varint32(&x[0])
	w.Varint32(&x[1])
	w.Varint32(&x[2])
}

// UBlockPos writes a BlockPos as 2 varint32s and a varuint32 to the underlying buffer.
func (w *Writer) UBlockPos(x *BlockPos) {
	w.Varint32(&x[0])
	y := uint32(x[1])
	w.Varuint32(&y)
	w.Varint32(&x[2])
}

// VarRGBA writes a color.RGBA x as a varuint32 to the underlying buffer.
func (w *Writer) VarRGBA(x *color.RGBA) {
	val := uint32(x.R) | uint32(x.G)<<8 | uint32(x.B)<<16 | uint32(x.A)<<24
	w.Varuint32(&val)
}

// UUID writes a UUID to the underlying buffer.
func (w *Writer) UUID(x *uuid.UUID) {
	b := append((*x)[8:], (*x)[:8]...)
	for i, j := 0, 15; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	_, _ = w.w.Write(b)
}

// PlayerInventoryAction writes a PlayerInventoryAction.
func (w *Writer) PlayerInventoryAction(x *UseItemTransactionData) {
	w.Varint32(&x.LegacyRequestID)
	if x.LegacyRequestID < -1 && (x.LegacyRequestID&1) == 0 {
		l := uint32(len(x.LegacySetItemSlots))
		w.Varuint32(&l)

		for _, slot := range x.LegacySetItemSlots {
			SetItemSlot(w, &slot)
		}
	}
	l := uint32(len(x.Actions))
	w.Varuint32(&l)

	for _, a := range x.Actions {
		InvAction(w, &a)
	}
	w.Varuint32(&x.ActionType)
	w.BlockPos(&x.BlockPosition)
	w.Varint32(&x.BlockFace)
	w.Varint32(&x.HotBarSlot)
	w.ItemInstance(&x.HeldItem)
	w.Vec3(&x.Position)
	w.Vec3(&x.ClickedPosition)
	w.Varuint32(&x.BlockRuntimeID)
}

// EntityMetadata writes an entity metadata map x to the underlying buffer.
func (w *Writer) EntityMetadata(x *map[uint32]interface{}) {
	l := uint32(len(*x))
	w.Varuint32(&l)
	for key, value := range *x {
		w.Varuint32(&key)
		switch v := value.(type) {
		case byte:
			w.Varuint32(&entityDataByte)
			w.Uint8(&v)
		case int16:
			w.Varuint32(&entityDataInt16)
			w.Int16(&v)
		case int32:
			w.Varuint32(&entityDataInt32)
			w.Varint32(&v)
		case float32:
			w.Varuint32(&entityDataFloat32)
			w.Float32(&v)
		case string:
			w.Varuint32(&entityDataString)
			w.String(&v)
		case map[string]interface{}:
			w.Varuint32(&entityDataCompoundTag)
			w.NBT(&v, nbt.NetworkLittleEndian)
		case BlockPos:
			w.Varuint32(&entityDataBlockPos)
			w.BlockPos(&v)
		case int64:
			w.Varuint32(&entityDataInt64)
			w.Varint64(&v)
		case mgl32.Vec3:
			w.Varuint32(&entityDataVec3)
			w.Vec3(&v)
		default:
			w.UnknownEnumOption(reflect.TypeOf(value), "entity metadata")
		}
	}
}

// ItemInstance writes an ItemInstance x to the underlying buffer.
func (w *Writer) ItemInstance(i *ItemInstance) {
	x := &i.Stack
	w.Varint32(&x.NetworkID)
	if x.NetworkID == 0 {
		// The item was air, so there's no more data to follow. Return immediately.
		return
	}

	w.Uint16(&x.Count)
	w.Varuint32(&x.MetadataValue)

	hasNetID := i.StackNetworkID != 0
	w.Bool(&hasNetID)

	if hasNetID {
		w.Varint32(&i.StackNetworkID)
	}

	w.Varint32(&x.BlockRuntimeID)

	buf := new(bytes.Buffer)
	bufWriter := NewWriter(buf, w.shieldID)

	var length int16
	if len(x.NBTData) != 0 {
		length = int16(-1)
		version := uint8(1)

		bufWriter.Int16(&length)
		bufWriter.Uint8(&version)
		bufWriter.NBT(&x.NBTData, nbt.LittleEndian)
	} else {
		bufWriter.Int16(&length)
	}

	placeOnLen := int32(len(x.CanBePlacedOn))
	canBreak := int32(len(x.CanBreak))

	bufWriter.Int32(&placeOnLen)
	for _, block := range x.CanBePlacedOn {
		bufWriter.StringUTF(&block)
	}
	bufWriter.Int32(&canBreak)
	for _, block := range x.CanBreak {
		bufWriter.StringUTF(&block)
	}
	if x.NetworkID == bufWriter.shieldID {
		var blockingTick int64
		bufWriter.Int64(&blockingTick)
	}

	b := buf.Bytes()
	w.ByteSlice(&b)
}

// Item writes an ItemStack x to the underlying buffer.
func (w *Writer) Item(x *ItemStack) {
	w.Varint32(&x.NetworkID)
	if x.NetworkID == 0 {
		// The item was air, so there's no more data to follow. Return immediately.
		return
	}

	w.Uint16(&x.Count)
	w.Varuint32(&x.MetadataValue)
	w.Varint32(&x.BlockRuntimeID)

	var extraData []byte
	buf := bytes.NewBuffer(extraData)
	bufWriter := NewWriter(buf, w.shieldID)

	var length int16
	if len(x.NBTData) != 0 {
		length = int16(-1)
		version := uint8(1)

		bufWriter.Int16(&length)
		bufWriter.Uint8(&version)
		bufWriter.NBT(&x.NBTData, nbt.LittleEndian)
	} else {
		bufWriter.Int16(&length)
	}

	placeOnLen := int32(len(x.CanBePlacedOn))
	canBreak := int32(len(x.CanBreak))

	bufWriter.Int32(&placeOnLen)
	for _, block := range x.CanBePlacedOn {
		bufWriter.StringUTF(&block)
	}
	bufWriter.Int32(&canBreak)
	for _, block := range x.CanBreak {
		bufWriter.StringUTF(&block)
	}
	if x.NetworkID == bufWriter.shieldID {
		var blockingTick int64
		bufWriter.Int64(&blockingTick)
	}

	w.ByteSlice(&extraData)
}

// Varint64 writes an int64 as 1-10 bytes to the underlying buffer.
func (w *Writer) Varint64(x *int64) {
	u := *x
	ux := uint64(u) << 1
	if u < 0 {
		ux = ^ux
	}
	for ux >= 0x80 {
		_ = w.w.WriteByte(byte(ux) | 0x80)
		ux >>= 7
	}
	_ = w.w.WriteByte(byte(ux))
}

// Varuint64 writes a uint64 as 1-10 bytes to the underlying buffer.
func (w *Writer) Varuint64(x *uint64) {
	u := *x
	for u >= 0x80 {
		_ = w.w.WriteByte(byte(u) | 0x80)
		u >>= 7
	}
	_ = w.w.WriteByte(byte(u))
}

// Varint32 writes an int32 as 1-5 bytes to the underlying buffer.
func (w *Writer) Varint32(x *int32) {
	u := *x
	ux := uint32(u) << 1
	if u < 0 {
		ux = ^ux
	}
	for ux >= 0x80 {
		_ = w.w.WriteByte(byte(ux) | 0x80)
		ux >>= 7
	}
	_ = w.w.WriteByte(byte(ux))
}

// Varuint32 writes a uint32 as 1-5 bytes to the underlying buffer.
func (w *Writer) Varuint32(x *uint32) {
	u := *x
	for u >= 0x80 {
		_ = w.w.WriteByte(byte(u) | 0x80)
		u >>= 7
	}
	_ = w.w.WriteByte(byte(u))
}

// NBT writes a map as NBT to the underlying buffer using the encoding passed.
func (w *Writer) NBT(x *map[string]interface{}, encoding nbt.Encoding) {
	if err := nbt.NewEncoderWithEncoding(w.w, encoding).Encode(*x); err != nil {
		panic(err)
	}
}

// NBTList writes a slice as NBT to the underlying buffer using the encoding passed.
func (w *Writer) NBTList(x *[]interface{}, encoding nbt.Encoding) {
	if err := nbt.NewEncoderWithEncoding(w.w, encoding).Encode(*x); err != nil {
		panic(err)
	}
}

// UnknownEnumOption panics with an unknown enum option error.
func (w *Writer) UnknownEnumOption(value interface{}, enum string) {
	w.panicf("unknown value '%v' for enum type '%v'", value, enum)
}

// InvalidValue panics with an invalid value error.
func (w *Writer) InvalidValue(value interface{}, forField, reason string) {
	w.panicf("invalid value '%v' for %v: %v", value, forField, reason)
}

// panicf panics with the format and values passed.
func (w *Writer) panicf(format string, a ...interface{}) {
	panic(fmt.Errorf(format, a...))
}
