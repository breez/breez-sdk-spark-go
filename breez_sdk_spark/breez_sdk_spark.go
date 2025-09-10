package breez_sdk_spark

// #include <breez_sdk_spark.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/breez/breez-sdk-spark-go/breez_sdk_common"
	"io"
	"math"
	"runtime"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_breez_sdk_spark_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_breez_sdk_spark_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	FfiConverterBitcoinChainServiceINSTANCE.register()
	FfiConverterStorageINSTANCE.register()
	FfiConverterCallbackInterfaceEventListenerINSTANCE.register()
	FfiConverterCallbackInterfaceLoggerINSTANCE.register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_breez_sdk_spark_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("breez_sdk_spark: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_func_connect()
		})
		if checksum != 40345 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_func_connect: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_func_default_config()
		})
		if checksum != 62194 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_func_default_config: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_func_default_storage()
		})
		if checksum != 46285 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_func_default_storage: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_func_init_logging()
		})
		if checksum != 8518 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_func_init_logging: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_func_parse()
		})
		if checksum != 58372 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_func_parse: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_bitcoinchainservice_get_address_utxos()
		})
		if checksum != 20959 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_bitcoinchainservice_get_address_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_bitcoinchainservice_get_transaction_hex()
		})
		if checksum != 19571 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_bitcoinchainservice_get_transaction_hex: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_bitcoinchainservice_broadcast_transaction()
		})
		if checksum != 61083 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_bitcoinchainservice_broadcast_transaction: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_add_event_listener()
		})
		if checksum != 61844 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_add_event_listener: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_claim_deposit()
		})
		if checksum != 43529 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_claim_deposit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_disconnect()
		})
		if checksum != 30986 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_disconnect: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_get_info()
		})
		if checksum != 6771 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_get_info: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_get_payment()
		})
		if checksum != 11540 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_get_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_list_payments()
		})
		if checksum != 16156 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_list_payments: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_list_unclaimed_deposits()
		})
		if checksum != 22486 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_list_unclaimed_deposits: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_lnurl_pay()
		})
		if checksum != 10147 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_lnurl_pay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_poll_lightning_send_payment()
		})
		if checksum != 5478 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_poll_lightning_send_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_prepare_lnurl_pay()
		})
		if checksum != 37691 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_prepare_lnurl_pay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_prepare_send_payment()
		})
		if checksum != 34185 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_prepare_send_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_receive_payment()
		})
		if checksum != 36984 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_receive_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_refund_deposit()
		})
		if checksum != 33646 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_refund_deposit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_remove_event_listener()
		})
		if checksum != 60980 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_remove_event_listener: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_send_payment()
		})
		if checksum != 54349 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_send_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_send_payment_internal()
		})
		if checksum != 37855 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_send_payment_internal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_breezsdk_sync_wallet()
		})
		if checksum != 36066 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_breezsdk_sync_wallet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_sdkbuilder_build()
		})
		if checksum != 8126 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_sdkbuilder_build: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_sdkbuilder_with_chain_service()
		})
		if checksum != 2848 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_sdkbuilder_with_chain_service: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_sdkbuilder_with_lnurl_client()
		})
		if checksum != 61720 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_sdkbuilder_with_lnurl_client: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_sdkbuilder_with_rest_chain_service()
		})
		if checksum != 56288 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_sdkbuilder_with_rest_chain_service: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_get_cached_item()
		})
		if checksum != 11423 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_get_cached_item: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_set_cached_item()
		})
		if checksum != 17965 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_set_cached_item: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_list_payments()
		})
		if checksum != 55103 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_list_payments: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_insert_payment()
		})
		if checksum != 35649 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_insert_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_set_payment_metadata()
		})
		if checksum != 780 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_set_payment_metadata: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_get_payment_by_id()
		})
		if checksum != 32084 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_get_payment_by_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_add_deposit()
		})
		if checksum != 31647 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_add_deposit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_delete_deposit()
		})
		if checksum != 19211 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_delete_deposit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_list_deposits()
		})
		if checksum != 11262 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_list_deposits: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_storage_update_deposit()
		})
		if checksum != 58400 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_storage_update_deposit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_constructor_sdkbuilder_new()
		})
		if checksum != 52744 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_constructor_sdkbuilder_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_eventlistener_on_event()
		})
		if checksum != 10824 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_eventlistener_on_event: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_spark_checksum_method_logger_log()
		})
		if checksum != 11839 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_spark: uniffi_breez_sdk_spark_checksum_method_logger_log: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type BitcoinChainService interface {
	GetAddressUtxos(address string) ([]Utxo, error)
	GetTransactionHex(txid string) (string, error)
	BroadcastTransaction(tx string) error
}
type BitcoinChainServiceImpl struct {
	ffiObject FfiObject
}

func (_self *BitcoinChainServiceImpl) GetAddressUtxos(address string) ([]Utxo, error) {
	_pointer := _self.ffiObject.incrementPointer("BitcoinChainService")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[ChainServiceError](
		FfiConverterChainServiceErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) []Utxo {
			return FfiConverterSequenceUtxoINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_bitcoinchainservice_get_address_utxos(
			_pointer, FfiConverterStringINSTANCE.Lower(address)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BitcoinChainServiceImpl) GetTransactionHex(txid string) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("BitcoinChainService")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[ChainServiceError](
		FfiConverterChainServiceErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) string {
			return FfiConverterStringINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_bitcoinchainservice_get_transaction_hex(
			_pointer, FfiConverterStringINSTANCE.Lower(txid)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BitcoinChainServiceImpl) BroadcastTransaction(tx string) error {
	_pointer := _self.ffiObject.incrementPointer("BitcoinChainService")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[ChainServiceError](
		FfiConverterChainServiceErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_bitcoinchainservice_broadcast_transaction(
			_pointer, FfiConverterStringINSTANCE.Lower(tx)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}
func (object *BitcoinChainServiceImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBitcoinChainService struct {
	handleMap *concurrentHandleMap[BitcoinChainService]
}

var FfiConverterBitcoinChainServiceINSTANCE = FfiConverterBitcoinChainService{
	handleMap: newConcurrentHandleMap[BitcoinChainService](),
}

func (c FfiConverterBitcoinChainService) Lift(pointer unsafe.Pointer) BitcoinChainService {
	result := &BitcoinChainServiceImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_breez_sdk_spark_fn_clone_bitcoinchainservice(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_breez_sdk_spark_fn_free_bitcoinchainservice(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*BitcoinChainServiceImpl).Destroy)
	return result
}

func (c FfiConverterBitcoinChainService) Read(reader io.Reader) BitcoinChainService {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBitcoinChainService) Lower(value BitcoinChainService) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterBitcoinChainService) Write(writer io.Writer, value BitcoinChainService) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBitcoinChainService struct{}

func (_ FfiDestroyerBitcoinChainService) Destroy(value BitcoinChainService) {
	if val, ok := value.(*BitcoinChainServiceImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *BitcoinChainServiceImpl")
	}
}

type uniffiCallbackResult C.int8_t

const (
	uniffiIdxCallbackFree               uniffiCallbackResult = 0
	uniffiCallbackResultSuccess         uniffiCallbackResult = 0
	uniffiCallbackResultError           uniffiCallbackResult = 1
	uniffiCallbackUnexpectedResultError uniffiCallbackResult = 2
	uniffiCallbackCancelled             uniffiCallbackResult = 3
)

type concurrentHandleMap[T any] struct {
	handles       map[uint64]T
	currentHandle uint64
	lock          sync.RWMutex
}

func newConcurrentHandleMap[T any]() *concurrentHandleMap[T] {
	return &concurrentHandleMap[T]{
		handles: map[uint64]T{},
	}
}

func (cm *concurrentHandleMap[T]) insert(obj T) uint64 {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.currentHandle = cm.currentHandle + 1
	cm.handles[cm.currentHandle] = obj
	return cm.currentHandle
}

func (cm *concurrentHandleMap[T]) remove(handle uint64) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	delete(cm.handles, handle)
}

func (cm *concurrentHandleMap[T]) tryGet(handle uint64) (T, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.handles[handle]
	return val, ok
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod0
func breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod0(uniffiHandle C.uint64_t, address C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterBitcoinChainServiceINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteRustBuffer(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructRustBuffer{}
		uniffiOutReturn := &asyncResult.returnValue
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		res, err :=
			uniffiObj.GetAddressUtxos(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: address,
				}),
			)

		if err != nil {
			var actualError *ChainServiceError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterChainServiceErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

		*uniffiOutReturn = FfiConverterSequenceUtxoINSTANCE.Lower(res)
	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod1
func breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod1(uniffiHandle C.uint64_t, txid C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterBitcoinChainServiceINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteRustBuffer(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructRustBuffer{}
		uniffiOutReturn := &asyncResult.returnValue
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		res, err :=
			uniffiObj.GetTransactionHex(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: txid,
				}),
			)

		if err != nil {
			var actualError *ChainServiceError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterChainServiceErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

		*uniffiOutReturn = FfiConverterStringINSTANCE.Lower(res)
	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod2
func breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod2(uniffiHandle C.uint64_t, tx C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterBitcoinChainServiceINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.BroadcastTransaction(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: tx,
				}),
			)

		if err != nil {
			var actualError *ChainServiceError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterChainServiceErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

var UniffiVTableCallbackInterfaceBitcoinChainServiceINSTANCE = C.UniffiVTableCallbackInterfaceBitcoinChainService{
	getAddressUtxos:      (C.UniffiCallbackInterfaceBitcoinChainServiceMethod0)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod0),
	getTransactionHex:    (C.UniffiCallbackInterfaceBitcoinChainServiceMethod1)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod1),
	broadcastTransaction: (C.UniffiCallbackInterfaceBitcoinChainServiceMethod2)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceMethod2),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceFree),
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceFree
func breez_sdk_spark_cgo_dispatchCallbackInterfaceBitcoinChainServiceFree(handle C.uint64_t) {
	FfiConverterBitcoinChainServiceINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterBitcoinChainService) register() {
	C.uniffi_breez_sdk_spark_fn_init_callback_vtable_bitcoinchainservice(&UniffiVTableCallbackInterfaceBitcoinChainServiceINSTANCE)
}

// `BreezSDK` is a wrapper around `SparkSDK` that provides a more structured API
// with request/response objects and comprehensive error handling.
type BreezSdkInterface interface {
	// Registers a listener to receive SDK events
	//
	// # Arguments
	//
	// * `listener` - An implementation of the `EventListener` trait
	//
	// # Returns
	//
	// A unique identifier for the listener, which can be used to remove it later
	AddEventListener(listener EventListener) string
	ClaimDeposit(request ClaimDepositRequest) (ClaimDepositResponse, error)
	// Stops the SDK's background tasks
	//
	// This method stops the background tasks started by the `start()` method.
	// It should be called before your application terminates to ensure proper cleanup.
	//
	// # Returns
	//
	// Result containing either success or an `SdkError` if the background task couldn't be stopped
	Disconnect() error
	// Returns the balance of the wallet in satoshis
	GetInfo(request GetInfoRequest) (GetInfoResponse, error)
	GetPayment(request GetPaymentRequest) (GetPaymentResponse, error)
	// Lists payments from the storage with pagination
	//
	// This method provides direct access to the payment history stored in the database.
	// It returns payments in reverse chronological order (newest first).
	//
	// # Arguments
	//
	// * `request` - Contains pagination parameters (offset and limit)
	//
	// # Returns
	//
	// * `Ok(ListPaymentsResponse)` - Contains the list of payments if successful
	// * `Err(SdkError)` - If there was an error accessing the storage

	ListPayments(request ListPaymentsRequest) (ListPaymentsResponse, error)
	ListUnclaimedDeposits(request ListUnclaimedDepositsRequest) (ListUnclaimedDepositsResponse, error)
	LnurlPay(request LnurlPayRequest) (LnurlPayResponse, error)
	PollLightningSendPayment(paymentId string)
	PrepareLnurlPay(request PrepareLnurlPayRequest) (PrepareLnurlPayResponse, error)
	PrepareSendPayment(request PrepareSendPaymentRequest) (PrepareSendPaymentResponse, error)
	ReceivePayment(request ReceivePaymentRequest) (ReceivePaymentResponse, error)
	RefundDeposit(request RefundDepositRequest) (RefundDepositResponse, error)
	// Removes a previously registered event listener
	//
	// # Arguments
	//
	// * `id` - The listener ID returned from `add_event_listener`
	//
	// # Returns
	//
	// `true` if the listener was found and removed, `false` otherwise
	RemoveEventListener(id string) bool
	SendPayment(request SendPaymentRequest) (SendPaymentResponse, error)
	SendPaymentInternal(request SendPaymentRequest, suppressPaymentEvent bool) (SendPaymentResponse, error)
	// Synchronizes the wallet with the Spark network
	SyncWallet(request SyncWalletRequest) (SyncWalletResponse, error)
}

// `BreezSDK` is a wrapper around `SparkSDK` that provides a more structured API
// with request/response objects and comprehensive error handling.
type BreezSdk struct {
	ffiObject FfiObject
}

// Registers a listener to receive SDK events
//
// # Arguments
//
// * `listener` - An implementation of the `EventListener` trait
//
// # Returns
//
// A unique identifier for the listener, which can be used to remove it later
func (_self *BreezSdk) AddEventListener(listener EventListener) string {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_breez_sdk_spark_fn_method_breezsdk_add_event_listener(
				_pointer, FfiConverterCallbackInterfaceEventListenerINSTANCE.Lower(listener), _uniffiStatus),
		}
	}))
}

func (_self *BreezSdk) ClaimDeposit(request ClaimDepositRequest) (ClaimDepositResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) ClaimDepositResponse {
			return FfiConverterClaimDepositResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_claim_deposit(
			_pointer, FfiConverterClaimDepositRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Stops the SDK's background tasks
//
// This method stops the background tasks started by the `start()` method.
// It should be called before your application terminates to ensure proper cleanup.
//
// # Returns
//
// Result containing either success or an `SdkError` if the background task couldn't be stopped
func (_self *BreezSdk) Disconnect() error {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[SdkError](FfiConverterSdkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_disconnect(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

// Returns the balance of the wallet in satoshis
func (_self *BreezSdk) GetInfo(request GetInfoRequest) (GetInfoResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) GetInfoResponse {
			return FfiConverterGetInfoResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_get_info(
			_pointer, FfiConverterGetInfoRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) GetPayment(request GetPaymentRequest) (GetPaymentResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) GetPaymentResponse {
			return FfiConverterGetPaymentResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_get_payment(
			_pointer, FfiConverterGetPaymentRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Lists payments from the storage with pagination
//
// This method provides direct access to the payment history stored in the database.
// It returns payments in reverse chronological order (newest first).
//
// # Arguments
//
// * `request` - Contains pagination parameters (offset and limit)
//
// # Returns
//
// * `Ok(ListPaymentsResponse)` - Contains the list of payments if successful
// * `Err(SdkError)` - If there was an error accessing the storage

func (_self *BreezSdk) ListPayments(request ListPaymentsRequest) (ListPaymentsResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) ListPaymentsResponse {
			return FfiConverterListPaymentsResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_list_payments(
			_pointer, FfiConverterListPaymentsRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) ListUnclaimedDeposits(request ListUnclaimedDepositsRequest) (ListUnclaimedDepositsResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) ListUnclaimedDepositsResponse {
			return FfiConverterListUnclaimedDepositsResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_list_unclaimed_deposits(
			_pointer, FfiConverterListUnclaimedDepositsRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) LnurlPay(request LnurlPayRequest) (LnurlPayResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) LnurlPayResponse {
			return FfiConverterLnurlPayResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_lnurl_pay(
			_pointer, FfiConverterLnurlPayRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) PollLightningSendPayment(paymentId string) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_poll_lightning_send_payment(
			_pointer, FfiConverterStringINSTANCE.Lower(paymentId), _uniffiStatus)
		return false
	})
}

func (_self *BreezSdk) PrepareLnurlPay(request PrepareLnurlPayRequest) (PrepareLnurlPayResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) PrepareLnurlPayResponse {
			return FfiConverterPrepareLnurlPayResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_prepare_lnurl_pay(
			_pointer, FfiConverterPrepareLnurlPayRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) PrepareSendPayment(request PrepareSendPaymentRequest) (PrepareSendPaymentResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) PrepareSendPaymentResponse {
			return FfiConverterPrepareSendPaymentResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_prepare_send_payment(
			_pointer, FfiConverterPrepareSendPaymentRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) ReceivePayment(request ReceivePaymentRequest) (ReceivePaymentResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) ReceivePaymentResponse {
			return FfiConverterReceivePaymentResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_receive_payment(
			_pointer, FfiConverterReceivePaymentRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) RefundDeposit(request RefundDepositRequest) (RefundDepositResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) RefundDepositResponse {
			return FfiConverterRefundDepositResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_refund_deposit(
			_pointer, FfiConverterRefundDepositRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Removes a previously registered event listener
//
// # Arguments
//
// * `id` - The listener ID returned from `add_event_listener`
//
// # Returns
//
// `true` if the listener was found and removed, `false` otherwise
func (_self *BreezSdk) RemoveEventListener(id string) bool {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_breez_sdk_spark_fn_method_breezsdk_remove_event_listener(
			_pointer, FfiConverterStringINSTANCE.Lower(id), _uniffiStatus)
	}))
}

func (_self *BreezSdk) SendPayment(request SendPaymentRequest) (SendPaymentResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) SendPaymentResponse {
			return FfiConverterSendPaymentResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_send_payment(
			_pointer, FfiConverterSendPaymentRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *BreezSdk) SendPaymentInternal(request SendPaymentRequest, suppressPaymentEvent bool) (SendPaymentResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) SendPaymentResponse {
			return FfiConverterSendPaymentResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_breezsdk_send_payment_internal(
			_pointer, FfiConverterSendPaymentRequestINSTANCE.Lower(request), FfiConverterBoolINSTANCE.Lower(suppressPaymentEvent)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Synchronizes the wallet with the Spark network
func (_self *BreezSdk) SyncWallet(request SyncWalletRequest) (SyncWalletResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*BreezSdk")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SdkError](FfiConverterSdkError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_breez_sdk_spark_fn_method_breezsdk_sync_wallet(
				_pointer, FfiConverterSyncWalletRequestINSTANCE.Lower(request), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue SyncWalletResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSyncWalletResponseINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *BreezSdk) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBreezSdk struct{}

var FfiConverterBreezSdkINSTANCE = FfiConverterBreezSdk{}

func (c FfiConverterBreezSdk) Lift(pointer unsafe.Pointer) *BreezSdk {
	result := &BreezSdk{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_breez_sdk_spark_fn_clone_breezsdk(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_breez_sdk_spark_fn_free_breezsdk(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*BreezSdk).Destroy)
	return result
}

func (c FfiConverterBreezSdk) Read(reader io.Reader) *BreezSdk {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBreezSdk) Lower(value *BreezSdk) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*BreezSdk")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterBreezSdk) Write(writer io.Writer, value *BreezSdk) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBreezSdk struct{}

func (_ FfiDestroyerBreezSdk) Destroy(value *BreezSdk) {
	value.Destroy()
}

// Builder for creating `BreezSdk` instances with customizable components.
type SdkBuilderInterface interface {
	// Builds the `BreezSdk` instance with the configured components.
	Build() (*BreezSdk, error)
	// Sets the chain service to be used by the SDK.
	// Arguments:
	// - `chain_service`: The chain service to be used.
	WithChainService(chainService BitcoinChainService)
	WithLnurlClient(lnurlClient breez_sdk_common.RestClient)
	// Sets the REST chain service to be used by the SDK.
	// Arguments:
	// - `url`: The base URL of the REST API.
	// - `credentials`: Optional credentials for basic authentication.
	WithRestChainService(url string, credentials *Credentials)
}

// Builder for creating `BreezSdk` instances with customizable components.
type SdkBuilder struct {
	ffiObject FfiObject
}

// Creates a new `SdkBuilder` with the provided configuration.
// Arguments:
// - `config`: The configuration to be used.
// - `mnemonic`: The mnemonic phrase for the wallet.
// - `storage`: The storage backend to be used.
func NewSdkBuilder(config Config, mnemonic string, storage Storage) *SdkBuilder {
	return FfiConverterSdkBuilderINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_breez_sdk_spark_fn_constructor_sdkbuilder_new(FfiConverterConfigINSTANCE.Lower(config), FfiConverterStringINSTANCE.Lower(mnemonic), FfiConverterStorageINSTANCE.Lower(storage), _uniffiStatus)
	}))
}

// Builds the `BreezSdk` instance with the configured components.
func (_self *SdkBuilder) Build() (*BreezSdk, error) {
	_pointer := _self.ffiObject.incrementPointer("*SdkBuilder")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_breez_sdk_spark_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *BreezSdk {
			return FfiConverterBreezSdkINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_sdkbuilder_build(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_pointer(handle)
		},
	)

	return res, err
}

// Sets the chain service to be used by the SDK.
// Arguments:
// - `chain_service`: The chain service to be used.
func (_self *SdkBuilder) WithChainService(chainService BitcoinChainService) {
	_pointer := _self.ffiObject.incrementPointer("*SdkBuilder")
	defer _self.ffiObject.decrementPointer()
	uniffiRustCallAsync[error](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_sdkbuilder_with_chain_service(
			_pointer, FfiConverterBitcoinChainServiceINSTANCE.Lower(chainService)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

}

func (_self *SdkBuilder) WithLnurlClient(lnurlClient breez_sdk_common.RestClient) {
	_pointer := _self.ffiObject.incrementPointer("*SdkBuilder")
	defer _self.ffiObject.decrementPointer()
	uniffiRustCallAsync[error](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_sdkbuilder_with_lnurl_client(
			_pointer, breez_sdk_common.FfiConverterRestClientINSTANCE.Lower(lnurlClient)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

}

// Sets the REST chain service to be used by the SDK.
// Arguments:
// - `url`: The base URL of the REST API.
// - `credentials`: Optional credentials for basic authentication.
func (_self *SdkBuilder) WithRestChainService(url string, credentials *Credentials) {
	_pointer := _self.ffiObject.incrementPointer("*SdkBuilder")
	defer _self.ffiObject.decrementPointer()
	uniffiRustCallAsync[error](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_sdkbuilder_with_rest_chain_service(
			_pointer, FfiConverterStringINSTANCE.Lower(url), FfiConverterOptionalCredentialsINSTANCE.Lower(credentials)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

}
func (object *SdkBuilder) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSdkBuilder struct{}

var FfiConverterSdkBuilderINSTANCE = FfiConverterSdkBuilder{}

func (c FfiConverterSdkBuilder) Lift(pointer unsafe.Pointer) *SdkBuilder {
	result := &SdkBuilder{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_breez_sdk_spark_fn_clone_sdkbuilder(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_breez_sdk_spark_fn_free_sdkbuilder(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*SdkBuilder).Destroy)
	return result
}

func (c FfiConverterSdkBuilder) Read(reader io.Reader) *SdkBuilder {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSdkBuilder) Lower(value *SdkBuilder) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SdkBuilder")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSdkBuilder) Write(writer io.Writer, value *SdkBuilder) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSdkBuilder struct{}

func (_ FfiDestroyerSdkBuilder) Destroy(value *SdkBuilder) {
	value.Destroy()
}

// Trait for persistent storage
type Storage interface {
	GetCachedItem(key string) (*string, error)
	SetCachedItem(key string, value string) error
	// Lists payments with pagination
	//
	// # Arguments
	//
	// * `offset` - Number of records to skip
	// * `limit` - Maximum number of records to return
	//
	// # Returns
	//
	// A vector of payments or a `StorageError`
	ListPayments(offset *uint32, limit *uint32) ([]Payment, error)
	// Inserts a payment into storage
	//
	// # Arguments
	//
	// * `payment` - The payment to insert
	//
	// # Returns
	//
	// Success or a `StorageError`
	InsertPayment(payment Payment) error
	// Inserts payment metadata into storage
	//
	// # Arguments
	//
	// * `payment_id` - The ID of the payment
	// * `metadata` - The metadata to insert
	//
	// # Returns
	//
	// Success or a `StorageError`
	SetPaymentMetadata(paymentId string, metadata PaymentMetadata) error
	// Gets a payment by its ID
	// # Arguments
	//
	// * `id` - The ID of the payment to retrieve
	//
	// # Returns
	//
	// The payment if found or None if not found
	GetPaymentById(id string) (Payment, error)
	// Add a deposit to storage
	// # Arguments
	//
	// * `txid` - The transaction ID of the deposit
	// * `vout` - The output index of the deposit
	// * `amount_sats` - The amount of the deposit in sats
	//
	// # Returns
	//
	// Success or a `StorageError`
	AddDeposit(txid string, vout uint32, amountSats uint64) error
	// Removes an unclaimed deposit from storage
	// # Arguments
	//
	// * `txid` - The transaction ID of the deposit
	// * `vout` - The output index of the deposit
	//
	// # Returns
	//
	// Success or a `StorageError`
	DeleteDeposit(txid string, vout uint32) error
	// Lists all unclaimed deposits from storage
	// # Returns
	//
	// A vector of `DepositInfo` or a `StorageError`
	ListDeposits() ([]DepositInfo, error)
	// Updates or inserts unclaimed deposit details
	// # Arguments
	//
	// * `txid` - The transaction ID of the deposit
	// * `vout` - The output index of the deposit
	// * `payload` - The payload for the update
	//
	// # Returns
	//
	// Success or a `StorageError`
	UpdateDeposit(txid string, vout uint32, payload UpdateDepositPayload) error
}

// Trait for persistent storage
type StorageImpl struct {
	ffiObject FfiObject
}

func (_self *StorageImpl) GetCachedItem(key string) (*string, error) {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) *string {
			return FfiConverterOptionalStringINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_storage_get_cached_item(
			_pointer, FfiConverterStringINSTANCE.Lower(key)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

func (_self *StorageImpl) SetCachedItem(key string, value string) error {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_storage_set_cached_item(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterStringINSTANCE.Lower(value)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}

// Lists payments with pagination
//
// # Arguments
//
// * `offset` - Number of records to skip
// * `limit` - Maximum number of records to return
//
// # Returns
//
// A vector of payments or a `StorageError`
func (_self *StorageImpl) ListPayments(offset *uint32, limit *uint32) ([]Payment, error) {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) []Payment {
			return FfiConverterSequencePaymentINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_storage_list_payments(
			_pointer, FfiConverterOptionalUint32INSTANCE.Lower(offset), FfiConverterOptionalUint32INSTANCE.Lower(limit)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Inserts a payment into storage
//
// # Arguments
//
// * `payment` - The payment to insert
//
// # Returns
//
// Success or a `StorageError`
func (_self *StorageImpl) InsertPayment(payment Payment) error {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_storage_insert_payment(
			_pointer, FfiConverterPaymentINSTANCE.Lower(payment)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}

// Inserts payment metadata into storage
//
// # Arguments
//
// * `payment_id` - The ID of the payment
// * `metadata` - The metadata to insert
//
// # Returns
//
// Success or a `StorageError`
func (_self *StorageImpl) SetPaymentMetadata(paymentId string, metadata PaymentMetadata) error {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_storage_set_payment_metadata(
			_pointer, FfiConverterStringINSTANCE.Lower(paymentId), FfiConverterPaymentMetadataINSTANCE.Lower(metadata)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}

// Gets a payment by its ID
// # Arguments
//
// * `id` - The ID of the payment to retrieve
//
// # Returns
//
// The payment if found or None if not found
func (_self *StorageImpl) GetPaymentById(id string) (Payment, error) {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) Payment {
			return FfiConverterPaymentINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_storage_get_payment_by_id(
			_pointer, FfiConverterStringINSTANCE.Lower(id)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Add a deposit to storage
// # Arguments
//
// * `txid` - The transaction ID of the deposit
// * `vout` - The output index of the deposit
// * `amount_sats` - The amount of the deposit in sats
//
// # Returns
//
// Success or a `StorageError`
func (_self *StorageImpl) AddDeposit(txid string, vout uint32, amountSats uint64) error {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_storage_add_deposit(
			_pointer, FfiConverterStringINSTANCE.Lower(txid), FfiConverterUint32INSTANCE.Lower(vout), FfiConverterUint64INSTANCE.Lower(amountSats)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}

// Removes an unclaimed deposit from storage
// # Arguments
//
// * `txid` - The transaction ID of the deposit
// * `vout` - The output index of the deposit
//
// # Returns
//
// Success or a `StorageError`
func (_self *StorageImpl) DeleteDeposit(txid string, vout uint32) error {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_storage_delete_deposit(
			_pointer, FfiConverterStringINSTANCE.Lower(txid), FfiConverterUint32INSTANCE.Lower(vout)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}

// Lists all unclaimed deposits from storage
// # Returns
//
// A vector of `DepositInfo` or a `StorageError`
func (_self *StorageImpl) ListDeposits() ([]DepositInfo, error) {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) []DepositInfo {
			return FfiConverterSequenceDepositInfoINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_method_storage_list_deposits(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Updates or inserts unclaimed deposit details
// # Arguments
//
// * `txid` - The transaction ID of the deposit
// * `vout` - The output index of the deposit
// * `payload` - The payload for the update
//
// # Returns
//
// Success or a `StorageError`
func (_self *StorageImpl) UpdateDeposit(txid string, vout uint32, payload UpdateDepositPayload) error {
	_pointer := _self.ffiObject.incrementPointer("Storage")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[StorageError](
		FfiConverterStorageErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_breez_sdk_spark_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_breez_sdk_spark_fn_method_storage_update_deposit(
			_pointer, FfiConverterStringINSTANCE.Lower(txid), FfiConverterUint32INSTANCE.Lower(vout), FfiConverterUpdateDepositPayloadINSTANCE.Lower(payload)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_void(handle)
		},
	)

	return err
}
func (object *StorageImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterStorage struct {
	handleMap *concurrentHandleMap[Storage]
}

var FfiConverterStorageINSTANCE = FfiConverterStorage{
	handleMap: newConcurrentHandleMap[Storage](),
}

func (c FfiConverterStorage) Lift(pointer unsafe.Pointer) Storage {
	result := &StorageImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_breez_sdk_spark_fn_clone_storage(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_breez_sdk_spark_fn_free_storage(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*StorageImpl).Destroy)
	return result
}

func (c FfiConverterStorage) Read(reader io.Reader) Storage {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterStorage) Lower(value Storage) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterStorage) Write(writer io.Writer, value Storage) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerStorage struct{}

func (_ FfiDestroyerStorage) Destroy(value Storage) {
	if val, ok := value.(*StorageImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *StorageImpl")
	}
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod0
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod0(uniffiHandle C.uint64_t, key C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteRustBuffer(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructRustBuffer{}
		uniffiOutReturn := &asyncResult.returnValue
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		res, err :=
			uniffiObj.GetCachedItem(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: key,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

		*uniffiOutReturn = FfiConverterOptionalStringINSTANCE.Lower(res)
	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod1
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod1(uniffiHandle C.uint64_t, key C.RustBuffer, value C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.SetCachedItem(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: key,
				}),
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: value,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod2
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod2(uniffiHandle C.uint64_t, offset C.RustBuffer, limit C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteRustBuffer(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructRustBuffer{}
		uniffiOutReturn := &asyncResult.returnValue
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		res, err :=
			uniffiObj.ListPayments(
				FfiConverterOptionalUint32INSTANCE.Lift(GoRustBuffer{
					inner: offset,
				}),
				FfiConverterOptionalUint32INSTANCE.Lift(GoRustBuffer{
					inner: limit,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

		*uniffiOutReturn = FfiConverterSequencePaymentINSTANCE.Lower(res)
	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod3
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod3(uniffiHandle C.uint64_t, payment C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.InsertPayment(
				FfiConverterPaymentINSTANCE.Lift(GoRustBuffer{
					inner: payment,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod4
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod4(uniffiHandle C.uint64_t, paymentId C.RustBuffer, metadata C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.SetPaymentMetadata(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: paymentId,
				}),
				FfiConverterPaymentMetadataINSTANCE.Lift(GoRustBuffer{
					inner: metadata,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod5
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod5(uniffiHandle C.uint64_t, id C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteRustBuffer(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructRustBuffer{}
		uniffiOutReturn := &asyncResult.returnValue
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		res, err :=
			uniffiObj.GetPaymentById(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: id,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

		*uniffiOutReturn = FfiConverterPaymentINSTANCE.Lower(res)
	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod6
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod6(uniffiHandle C.uint64_t, txid C.RustBuffer, vout C.uint32_t, amountSats C.uint64_t, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.AddDeposit(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: txid,
				}),
				FfiConverterUint32INSTANCE.Lift(vout),
				FfiConverterUint64INSTANCE.Lift(amountSats),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod7
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod7(uniffiHandle C.uint64_t, txid C.RustBuffer, vout C.uint32_t, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.DeleteDeposit(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: txid,
				}),
				FfiConverterUint32INSTANCE.Lift(vout),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod8
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod8(uniffiHandle C.uint64_t, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteRustBuffer(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructRustBuffer{}
		uniffiOutReturn := &asyncResult.returnValue
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		res, err :=
			uniffiObj.ListDeposits()

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

		*uniffiOutReturn = FfiConverterSequenceDepositInfoINSTANCE.Lower(res)
	}()
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod9
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod9(uniffiHandle C.uint64_t, txid C.RustBuffer, vout C.uint32_t, payload C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterStorageINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_spark_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		callStatus := &asyncResult.callStatus
		defer func() {
			result <- *asyncResult
		}()

		err :=
			uniffiObj.UpdateDeposit(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: txid,
				}),
				FfiConverterUint32INSTANCE.Lift(vout),
				FfiConverterUpdateDepositPayloadINSTANCE.Lift(GoRustBuffer{
					inner: payload,
				}),
			)

		if err != nil {
			var actualError *StorageError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterStorageErrorINSTANCE.Lower(actualError),
					}
					return
				}
			} else {
				*callStatus = C.RustCallStatus{
					code: C.int8_t(uniffiCallbackUnexpectedResultError),
				}
				return
			}
		}

	}()
}

var UniffiVTableCallbackInterfaceStorageINSTANCE = C.UniffiVTableCallbackInterfaceStorage{
	getCachedItem:      (C.UniffiCallbackInterfaceStorageMethod0)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod0),
	setCachedItem:      (C.UniffiCallbackInterfaceStorageMethod1)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod1),
	listPayments:       (C.UniffiCallbackInterfaceStorageMethod2)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod2),
	insertPayment:      (C.UniffiCallbackInterfaceStorageMethod3)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod3),
	setPaymentMetadata: (C.UniffiCallbackInterfaceStorageMethod4)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod4),
	getPaymentById:     (C.UniffiCallbackInterfaceStorageMethod5)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod5),
	addDeposit:         (C.UniffiCallbackInterfaceStorageMethod6)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod6),
	deleteDeposit:      (C.UniffiCallbackInterfaceStorageMethod7)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod7),
	listDeposits:       (C.UniffiCallbackInterfaceStorageMethod8)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod8),
	updateDeposit:      (C.UniffiCallbackInterfaceStorageMethod9)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageMethod9),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageFree),
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageFree
func breez_sdk_spark_cgo_dispatchCallbackInterfaceStorageFree(handle C.uint64_t) {
	FfiConverterStorageINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterStorage) register() {
	C.uniffi_breez_sdk_spark_fn_init_callback_vtable_storage(&UniffiVTableCallbackInterfaceStorageINSTANCE)
}

type ClaimDepositRequest struct {
	Txid   string
	Vout   uint32
	MaxFee *Fee
}

func (r *ClaimDepositRequest) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Vout)
	FfiDestroyerOptionalFee{}.Destroy(r.MaxFee)
}

type FfiConverterClaimDepositRequest struct{}

var FfiConverterClaimDepositRequestINSTANCE = FfiConverterClaimDepositRequest{}

func (c FfiConverterClaimDepositRequest) Lift(rb RustBufferI) ClaimDepositRequest {
	return LiftFromRustBuffer[ClaimDepositRequest](c, rb)
}

func (c FfiConverterClaimDepositRequest) Read(reader io.Reader) ClaimDepositRequest {
	return ClaimDepositRequest{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterOptionalFeeINSTANCE.Read(reader),
	}
}

func (c FfiConverterClaimDepositRequest) Lower(value ClaimDepositRequest) C.RustBuffer {
	return LowerIntoRustBuffer[ClaimDepositRequest](c, value)
}

func (c FfiConverterClaimDepositRequest) Write(writer io.Writer, value ClaimDepositRequest) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
	FfiConverterOptionalFeeINSTANCE.Write(writer, value.MaxFee)
}

type FfiDestroyerClaimDepositRequest struct{}

func (_ FfiDestroyerClaimDepositRequest) Destroy(value ClaimDepositRequest) {
	value.Destroy()
}

type ClaimDepositResponse struct {
	Payment Payment
}

func (r *ClaimDepositResponse) Destroy() {
	FfiDestroyerPayment{}.Destroy(r.Payment)
}

type FfiConverterClaimDepositResponse struct{}

var FfiConverterClaimDepositResponseINSTANCE = FfiConverterClaimDepositResponse{}

func (c FfiConverterClaimDepositResponse) Lift(rb RustBufferI) ClaimDepositResponse {
	return LiftFromRustBuffer[ClaimDepositResponse](c, rb)
}

func (c FfiConverterClaimDepositResponse) Read(reader io.Reader) ClaimDepositResponse {
	return ClaimDepositResponse{
		FfiConverterPaymentINSTANCE.Read(reader),
	}
}

func (c FfiConverterClaimDepositResponse) Lower(value ClaimDepositResponse) C.RustBuffer {
	return LowerIntoRustBuffer[ClaimDepositResponse](c, value)
}

func (c FfiConverterClaimDepositResponse) Write(writer io.Writer, value ClaimDepositResponse) {
	FfiConverterPaymentINSTANCE.Write(writer, value.Payment)
}

type FfiDestroyerClaimDepositResponse struct{}

func (_ FfiDestroyerClaimDepositResponse) Destroy(value ClaimDepositResponse) {
	value.Destroy()
}

type Config struct {
	ApiKey             *string
	Network            Network
	SyncIntervalSecs   uint32
	MaxDepositClaimFee *Fee
}

func (r *Config) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.ApiKey)
	FfiDestroyerNetwork{}.Destroy(r.Network)
	FfiDestroyerUint32{}.Destroy(r.SyncIntervalSecs)
	FfiDestroyerOptionalFee{}.Destroy(r.MaxDepositClaimFee)
}

type FfiConverterConfig struct{}

var FfiConverterConfigINSTANCE = FfiConverterConfig{}

func (c FfiConverterConfig) Lift(rb RustBufferI) Config {
	return LiftFromRustBuffer[Config](c, rb)
}

func (c FfiConverterConfig) Read(reader io.Reader) Config {
	return Config{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterNetworkINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterOptionalFeeINSTANCE.Read(reader),
	}
}

func (c FfiConverterConfig) Lower(value Config) C.RustBuffer {
	return LowerIntoRustBuffer[Config](c, value)
}

func (c FfiConverterConfig) Write(writer io.Writer, value Config) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.ApiKey)
	FfiConverterNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterUint32INSTANCE.Write(writer, value.SyncIntervalSecs)
	FfiConverterOptionalFeeINSTANCE.Write(writer, value.MaxDepositClaimFee)
}

type FfiDestroyerConfig struct{}

func (_ FfiDestroyerConfig) Destroy(value Config) {
	value.Destroy()
}

type ConnectRequest struct {
	Config     Config
	Mnemonic   string
	StorageDir string
}

func (r *ConnectRequest) Destroy() {
	FfiDestroyerConfig{}.Destroy(r.Config)
	FfiDestroyerString{}.Destroy(r.Mnemonic)
	FfiDestroyerString{}.Destroy(r.StorageDir)
}

type FfiConverterConnectRequest struct{}

var FfiConverterConnectRequestINSTANCE = FfiConverterConnectRequest{}

func (c FfiConverterConnectRequest) Lift(rb RustBufferI) ConnectRequest {
	return LiftFromRustBuffer[ConnectRequest](c, rb)
}

func (c FfiConverterConnectRequest) Read(reader io.Reader) ConnectRequest {
	return ConnectRequest{
		FfiConverterConfigINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterConnectRequest) Lower(value ConnectRequest) C.RustBuffer {
	return LowerIntoRustBuffer[ConnectRequest](c, value)
}

func (c FfiConverterConnectRequest) Write(writer io.Writer, value ConnectRequest) {
	FfiConverterConfigINSTANCE.Write(writer, value.Config)
	FfiConverterStringINSTANCE.Write(writer, value.Mnemonic)
	FfiConverterStringINSTANCE.Write(writer, value.StorageDir)
}

type FfiDestroyerConnectRequest struct{}

func (_ FfiDestroyerConnectRequest) Destroy(value ConnectRequest) {
	value.Destroy()
}

type Credentials struct {
	Username string
	Password string
}

func (r *Credentials) Destroy() {
	FfiDestroyerString{}.Destroy(r.Username)
	FfiDestroyerString{}.Destroy(r.Password)
}

type FfiConverterCredentials struct{}

var FfiConverterCredentialsINSTANCE = FfiConverterCredentials{}

func (c FfiConverterCredentials) Lift(rb RustBufferI) Credentials {
	return LiftFromRustBuffer[Credentials](c, rb)
}

func (c FfiConverterCredentials) Read(reader io.Reader) Credentials {
	return Credentials{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterCredentials) Lower(value Credentials) C.RustBuffer {
	return LowerIntoRustBuffer[Credentials](c, value)
}

func (c FfiConverterCredentials) Write(writer io.Writer, value Credentials) {
	FfiConverterStringINSTANCE.Write(writer, value.Username)
	FfiConverterStringINSTANCE.Write(writer, value.Password)
}

type FfiDestroyerCredentials struct{}

func (_ FfiDestroyerCredentials) Destroy(value Credentials) {
	value.Destroy()
}

type DepositInfo struct {
	Txid       string
	Vout       uint32
	AmountSats uint64
	RefundTx   *string
	RefundTxId *string
	ClaimError *DepositClaimError
}

func (r *DepositInfo) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Vout)
	FfiDestroyerUint64{}.Destroy(r.AmountSats)
	FfiDestroyerOptionalString{}.Destroy(r.RefundTx)
	FfiDestroyerOptionalString{}.Destroy(r.RefundTxId)
	FfiDestroyerOptionalDepositClaimError{}.Destroy(r.ClaimError)
}

type FfiConverterDepositInfo struct{}

var FfiConverterDepositInfoINSTANCE = FfiConverterDepositInfo{}

func (c FfiConverterDepositInfo) Lift(rb RustBufferI) DepositInfo {
	return LiftFromRustBuffer[DepositInfo](c, rb)
}

func (c FfiConverterDepositInfo) Read(reader io.Reader) DepositInfo {
	return DepositInfo{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalDepositClaimErrorINSTANCE.Read(reader),
	}
}

func (c FfiConverterDepositInfo) Lower(value DepositInfo) C.RustBuffer {
	return LowerIntoRustBuffer[DepositInfo](c, value)
}

func (c FfiConverterDepositInfo) Write(writer io.Writer, value DepositInfo) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSats)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.RefundTx)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.RefundTxId)
	FfiConverterOptionalDepositClaimErrorINSTANCE.Write(writer, value.ClaimError)
}

type FfiDestroyerDepositInfo struct{}

func (_ FfiDestroyerDepositInfo) Destroy(value DepositInfo) {
	value.Destroy()
}

// Request to get the balance of the wallet
type GetInfoRequest struct {
}

func (r *GetInfoRequest) Destroy() {
}

type FfiConverterGetInfoRequest struct{}

var FfiConverterGetInfoRequestINSTANCE = FfiConverterGetInfoRequest{}

func (c FfiConverterGetInfoRequest) Lift(rb RustBufferI) GetInfoRequest {
	return LiftFromRustBuffer[GetInfoRequest](c, rb)
}

func (c FfiConverterGetInfoRequest) Read(reader io.Reader) GetInfoRequest {
	return GetInfoRequest{}
}

func (c FfiConverterGetInfoRequest) Lower(value GetInfoRequest) C.RustBuffer {
	return LowerIntoRustBuffer[GetInfoRequest](c, value)
}

func (c FfiConverterGetInfoRequest) Write(writer io.Writer, value GetInfoRequest) {
}

type FfiDestroyerGetInfoRequest struct{}

func (_ FfiDestroyerGetInfoRequest) Destroy(value GetInfoRequest) {
	value.Destroy()
}

// Response containing the balance of the wallet
type GetInfoResponse struct {
	// The balance in satoshis
	BalanceSats uint64
}

func (r *GetInfoResponse) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.BalanceSats)
}

type FfiConverterGetInfoResponse struct{}

var FfiConverterGetInfoResponseINSTANCE = FfiConverterGetInfoResponse{}

func (c FfiConverterGetInfoResponse) Lift(rb RustBufferI) GetInfoResponse {
	return LiftFromRustBuffer[GetInfoResponse](c, rb)
}

func (c FfiConverterGetInfoResponse) Read(reader io.Reader) GetInfoResponse {
	return GetInfoResponse{
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterGetInfoResponse) Lower(value GetInfoResponse) C.RustBuffer {
	return LowerIntoRustBuffer[GetInfoResponse](c, value)
}

func (c FfiConverterGetInfoResponse) Write(writer io.Writer, value GetInfoResponse) {
	FfiConverterUint64INSTANCE.Write(writer, value.BalanceSats)
}

type FfiDestroyerGetInfoResponse struct{}

func (_ FfiDestroyerGetInfoResponse) Destroy(value GetInfoResponse) {
	value.Destroy()
}

type GetPaymentRequest struct {
	PaymentId string
}

func (r *GetPaymentRequest) Destroy() {
	FfiDestroyerString{}.Destroy(r.PaymentId)
}

type FfiConverterGetPaymentRequest struct{}

var FfiConverterGetPaymentRequestINSTANCE = FfiConverterGetPaymentRequest{}

func (c FfiConverterGetPaymentRequest) Lift(rb RustBufferI) GetPaymentRequest {
	return LiftFromRustBuffer[GetPaymentRequest](c, rb)
}

func (c FfiConverterGetPaymentRequest) Read(reader io.Reader) GetPaymentRequest {
	return GetPaymentRequest{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterGetPaymentRequest) Lower(value GetPaymentRequest) C.RustBuffer {
	return LowerIntoRustBuffer[GetPaymentRequest](c, value)
}

func (c FfiConverterGetPaymentRequest) Write(writer io.Writer, value GetPaymentRequest) {
	FfiConverterStringINSTANCE.Write(writer, value.PaymentId)
}

type FfiDestroyerGetPaymentRequest struct{}

func (_ FfiDestroyerGetPaymentRequest) Destroy(value GetPaymentRequest) {
	value.Destroy()
}

type GetPaymentResponse struct {
	Payment Payment
}

func (r *GetPaymentResponse) Destroy() {
	FfiDestroyerPayment{}.Destroy(r.Payment)
}

type FfiConverterGetPaymentResponse struct{}

var FfiConverterGetPaymentResponseINSTANCE = FfiConverterGetPaymentResponse{}

func (c FfiConverterGetPaymentResponse) Lift(rb RustBufferI) GetPaymentResponse {
	return LiftFromRustBuffer[GetPaymentResponse](c, rb)
}

func (c FfiConverterGetPaymentResponse) Read(reader io.Reader) GetPaymentResponse {
	return GetPaymentResponse{
		FfiConverterPaymentINSTANCE.Read(reader),
	}
}

func (c FfiConverterGetPaymentResponse) Lower(value GetPaymentResponse) C.RustBuffer {
	return LowerIntoRustBuffer[GetPaymentResponse](c, value)
}

func (c FfiConverterGetPaymentResponse) Write(writer io.Writer, value GetPaymentResponse) {
	FfiConverterPaymentINSTANCE.Write(writer, value.Payment)
}

type FfiDestroyerGetPaymentResponse struct{}

func (_ FfiDestroyerGetPaymentResponse) Destroy(value GetPaymentResponse) {
	value.Destroy()
}

// Request to list payments with pagination
type ListPaymentsRequest struct {
	// Number of records to skip
	Offset *uint32
	// Maximum number of records to return
	Limit *uint32
}

func (r *ListPaymentsRequest) Destroy() {
	FfiDestroyerOptionalUint32{}.Destroy(r.Offset)
	FfiDestroyerOptionalUint32{}.Destroy(r.Limit)
}

type FfiConverterListPaymentsRequest struct{}

var FfiConverterListPaymentsRequestINSTANCE = FfiConverterListPaymentsRequest{}

func (c FfiConverterListPaymentsRequest) Lift(rb RustBufferI) ListPaymentsRequest {
	return LiftFromRustBuffer[ListPaymentsRequest](c, rb)
}

func (c FfiConverterListPaymentsRequest) Read(reader io.Reader) ListPaymentsRequest {
	return ListPaymentsRequest{
		FfiConverterOptionalUint32INSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterListPaymentsRequest) Lower(value ListPaymentsRequest) C.RustBuffer {
	return LowerIntoRustBuffer[ListPaymentsRequest](c, value)
}

func (c FfiConverterListPaymentsRequest) Write(writer io.Writer, value ListPaymentsRequest) {
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Offset)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Limit)
}

type FfiDestroyerListPaymentsRequest struct{}

func (_ FfiDestroyerListPaymentsRequest) Destroy(value ListPaymentsRequest) {
	value.Destroy()
}

// Response from listing payments
type ListPaymentsResponse struct {
	// The list of payments
	Payments []Payment
}

func (r *ListPaymentsResponse) Destroy() {
	FfiDestroyerSequencePayment{}.Destroy(r.Payments)
}

type FfiConverterListPaymentsResponse struct{}

var FfiConverterListPaymentsResponseINSTANCE = FfiConverterListPaymentsResponse{}

func (c FfiConverterListPaymentsResponse) Lift(rb RustBufferI) ListPaymentsResponse {
	return LiftFromRustBuffer[ListPaymentsResponse](c, rb)
}

func (c FfiConverterListPaymentsResponse) Read(reader io.Reader) ListPaymentsResponse {
	return ListPaymentsResponse{
		FfiConverterSequencePaymentINSTANCE.Read(reader),
	}
}

func (c FfiConverterListPaymentsResponse) Lower(value ListPaymentsResponse) C.RustBuffer {
	return LowerIntoRustBuffer[ListPaymentsResponse](c, value)
}

func (c FfiConverterListPaymentsResponse) Write(writer io.Writer, value ListPaymentsResponse) {
	FfiConverterSequencePaymentINSTANCE.Write(writer, value.Payments)
}

type FfiDestroyerListPaymentsResponse struct{}

func (_ FfiDestroyerListPaymentsResponse) Destroy(value ListPaymentsResponse) {
	value.Destroy()
}

type ListUnclaimedDepositsRequest struct {
}

func (r *ListUnclaimedDepositsRequest) Destroy() {
}

type FfiConverterListUnclaimedDepositsRequest struct{}

var FfiConverterListUnclaimedDepositsRequestINSTANCE = FfiConverterListUnclaimedDepositsRequest{}

func (c FfiConverterListUnclaimedDepositsRequest) Lift(rb RustBufferI) ListUnclaimedDepositsRequest {
	return LiftFromRustBuffer[ListUnclaimedDepositsRequest](c, rb)
}

func (c FfiConverterListUnclaimedDepositsRequest) Read(reader io.Reader) ListUnclaimedDepositsRequest {
	return ListUnclaimedDepositsRequest{}
}

func (c FfiConverterListUnclaimedDepositsRequest) Lower(value ListUnclaimedDepositsRequest) C.RustBuffer {
	return LowerIntoRustBuffer[ListUnclaimedDepositsRequest](c, value)
}

func (c FfiConverterListUnclaimedDepositsRequest) Write(writer io.Writer, value ListUnclaimedDepositsRequest) {
}

type FfiDestroyerListUnclaimedDepositsRequest struct{}

func (_ FfiDestroyerListUnclaimedDepositsRequest) Destroy(value ListUnclaimedDepositsRequest) {
	value.Destroy()
}

type ListUnclaimedDepositsResponse struct {
	Deposits []DepositInfo
}

func (r *ListUnclaimedDepositsResponse) Destroy() {
	FfiDestroyerSequenceDepositInfo{}.Destroy(r.Deposits)
}

type FfiConverterListUnclaimedDepositsResponse struct{}

var FfiConverterListUnclaimedDepositsResponseINSTANCE = FfiConverterListUnclaimedDepositsResponse{}

func (c FfiConverterListUnclaimedDepositsResponse) Lift(rb RustBufferI) ListUnclaimedDepositsResponse {
	return LiftFromRustBuffer[ListUnclaimedDepositsResponse](c, rb)
}

func (c FfiConverterListUnclaimedDepositsResponse) Read(reader io.Reader) ListUnclaimedDepositsResponse {
	return ListUnclaimedDepositsResponse{
		FfiConverterSequenceDepositInfoINSTANCE.Read(reader),
	}
}

func (c FfiConverterListUnclaimedDepositsResponse) Lower(value ListUnclaimedDepositsResponse) C.RustBuffer {
	return LowerIntoRustBuffer[ListUnclaimedDepositsResponse](c, value)
}

func (c FfiConverterListUnclaimedDepositsResponse) Write(writer io.Writer, value ListUnclaimedDepositsResponse) {
	FfiConverterSequenceDepositInfoINSTANCE.Write(writer, value.Deposits)
}

type FfiDestroyerListUnclaimedDepositsResponse struct{}

func (_ FfiDestroyerListUnclaimedDepositsResponse) Destroy(value ListUnclaimedDepositsResponse) {
	value.Destroy()
}

// Represents the payment LNURL info
type LnurlPayInfo struct {
	LnAddress              *string
	Comment                *string
	Domain                 *string
	Metadata               *string
	ProcessedSuccessAction *breez_sdk_common.SuccessActionProcessed
	RawSuccessAction       *breez_sdk_common.SuccessAction
}

func (r *LnurlPayInfo) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.LnAddress)
	FfiDestroyerOptionalString{}.Destroy(r.Comment)
	FfiDestroyerOptionalString{}.Destroy(r.Domain)
	FfiDestroyerOptionalString{}.Destroy(r.Metadata)
	FfiDestroyerOptionalSuccessActionProcessed{}.Destroy(r.ProcessedSuccessAction)
	FfiDestroyerOptionalSuccessAction{}.Destroy(r.RawSuccessAction)
}

type FfiConverterLnurlPayInfo struct{}

var FfiConverterLnurlPayInfoINSTANCE = FfiConverterLnurlPayInfo{}

func (c FfiConverterLnurlPayInfo) Lift(rb RustBufferI) LnurlPayInfo {
	return LiftFromRustBuffer[LnurlPayInfo](c, rb)
}

func (c FfiConverterLnurlPayInfo) Read(reader io.Reader) LnurlPayInfo {
	return LnurlPayInfo{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalSuccessActionProcessedINSTANCE.Read(reader),
		FfiConverterOptionalSuccessActionINSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlPayInfo) Lower(value LnurlPayInfo) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlPayInfo](c, value)
}

func (c FfiConverterLnurlPayInfo) Write(writer io.Writer, value LnurlPayInfo) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.LnAddress)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Comment)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Domain)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Metadata)
	FfiConverterOptionalSuccessActionProcessedINSTANCE.Write(writer, value.ProcessedSuccessAction)
	FfiConverterOptionalSuccessActionINSTANCE.Write(writer, value.RawSuccessAction)
}

type FfiDestroyerLnurlPayInfo struct{}

func (_ FfiDestroyerLnurlPayInfo) Destroy(value LnurlPayInfo) {
	value.Destroy()
}

type LnurlPayRequest struct {
	PrepareResponse PrepareLnurlPayResponse
}

func (r *LnurlPayRequest) Destroy() {
	FfiDestroyerPrepareLnurlPayResponse{}.Destroy(r.PrepareResponse)
}

type FfiConverterLnurlPayRequest struct{}

var FfiConverterLnurlPayRequestINSTANCE = FfiConverterLnurlPayRequest{}

func (c FfiConverterLnurlPayRequest) Lift(rb RustBufferI) LnurlPayRequest {
	return LiftFromRustBuffer[LnurlPayRequest](c, rb)
}

func (c FfiConverterLnurlPayRequest) Read(reader io.Reader) LnurlPayRequest {
	return LnurlPayRequest{
		FfiConverterPrepareLnurlPayResponseINSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlPayRequest) Lower(value LnurlPayRequest) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlPayRequest](c, value)
}

func (c FfiConverterLnurlPayRequest) Write(writer io.Writer, value LnurlPayRequest) {
	FfiConverterPrepareLnurlPayResponseINSTANCE.Write(writer, value.PrepareResponse)
}

type FfiDestroyerLnurlPayRequest struct{}

func (_ FfiDestroyerLnurlPayRequest) Destroy(value LnurlPayRequest) {
	value.Destroy()
}

type LnurlPayResponse struct {
	Payment       Payment
	SuccessAction *breez_sdk_common.SuccessActionProcessed
}

func (r *LnurlPayResponse) Destroy() {
	FfiDestroyerPayment{}.Destroy(r.Payment)
	FfiDestroyerOptionalSuccessActionProcessed{}.Destroy(r.SuccessAction)
}

type FfiConverterLnurlPayResponse struct{}

var FfiConverterLnurlPayResponseINSTANCE = FfiConverterLnurlPayResponse{}

func (c FfiConverterLnurlPayResponse) Lift(rb RustBufferI) LnurlPayResponse {
	return LiftFromRustBuffer[LnurlPayResponse](c, rb)
}

func (c FfiConverterLnurlPayResponse) Read(reader io.Reader) LnurlPayResponse {
	return LnurlPayResponse{
		FfiConverterPaymentINSTANCE.Read(reader),
		FfiConverterOptionalSuccessActionProcessedINSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlPayResponse) Lower(value LnurlPayResponse) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlPayResponse](c, value)
}

func (c FfiConverterLnurlPayResponse) Write(writer io.Writer, value LnurlPayResponse) {
	FfiConverterPaymentINSTANCE.Write(writer, value.Payment)
	FfiConverterOptionalSuccessActionProcessedINSTANCE.Write(writer, value.SuccessAction)
}

type FfiDestroyerLnurlPayResponse struct{}

func (_ FfiDestroyerLnurlPayResponse) Destroy(value LnurlPayResponse) {
	value.Destroy()
}

type LogEntry struct {
	Line  string
	Level string
}

func (r *LogEntry) Destroy() {
	FfiDestroyerString{}.Destroy(r.Line)
	FfiDestroyerString{}.Destroy(r.Level)
}

type FfiConverterLogEntry struct{}

var FfiConverterLogEntryINSTANCE = FfiConverterLogEntry{}

func (c FfiConverterLogEntry) Lift(rb RustBufferI) LogEntry {
	return LiftFromRustBuffer[LogEntry](c, rb)
}

func (c FfiConverterLogEntry) Read(reader io.Reader) LogEntry {
	return LogEntry{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterLogEntry) Lower(value LogEntry) C.RustBuffer {
	return LowerIntoRustBuffer[LogEntry](c, value)
}

func (c FfiConverterLogEntry) Write(writer io.Writer, value LogEntry) {
	FfiConverterStringINSTANCE.Write(writer, value.Line)
	FfiConverterStringINSTANCE.Write(writer, value.Level)
}

type FfiDestroyerLogEntry struct{}

func (_ FfiDestroyerLogEntry) Destroy(value LogEntry) {
	value.Destroy()
}

// Represents a payment (sent or received)
type Payment struct {
	// Unique identifier for the payment
	Id string
	// Type of payment (send or receive)
	PaymentType PaymentType
	// Status of the payment
	Status PaymentStatus
	// Amount in satoshis
	Amount uint64
	// Fee paid in satoshis
	Fees uint64
	// Timestamp of when the payment was created
	Timestamp uint64
	// Method of payment. Sometimes the payment details is empty so this field
	// is used to determine the payment method.
	Method PaymentMethod
	// Details of the payment
	Details *PaymentDetails
}

func (r *Payment) Destroy() {
	FfiDestroyerString{}.Destroy(r.Id)
	FfiDestroyerPaymentType{}.Destroy(r.PaymentType)
	FfiDestroyerPaymentStatus{}.Destroy(r.Status)
	FfiDestroyerUint64{}.Destroy(r.Amount)
	FfiDestroyerUint64{}.Destroy(r.Fees)
	FfiDestroyerUint64{}.Destroy(r.Timestamp)
	FfiDestroyerPaymentMethod{}.Destroy(r.Method)
	FfiDestroyerOptionalPaymentDetails{}.Destroy(r.Details)
}

type FfiConverterPayment struct{}

var FfiConverterPaymentINSTANCE = FfiConverterPayment{}

func (c FfiConverterPayment) Lift(rb RustBufferI) Payment {
	return LiftFromRustBuffer[Payment](c, rb)
}

func (c FfiConverterPayment) Read(reader io.Reader) Payment {
	return Payment{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterPaymentTypeINSTANCE.Read(reader),
		FfiConverterPaymentStatusINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterPaymentMethodINSTANCE.Read(reader),
		FfiConverterOptionalPaymentDetailsINSTANCE.Read(reader),
	}
}

func (c FfiConverterPayment) Lower(value Payment) C.RustBuffer {
	return LowerIntoRustBuffer[Payment](c, value)
}

func (c FfiConverterPayment) Write(writer io.Writer, value Payment) {
	FfiConverterStringINSTANCE.Write(writer, value.Id)
	FfiConverterPaymentTypeINSTANCE.Write(writer, value.PaymentType)
	FfiConverterPaymentStatusINSTANCE.Write(writer, value.Status)
	FfiConverterUint64INSTANCE.Write(writer, value.Amount)
	FfiConverterUint64INSTANCE.Write(writer, value.Fees)
	FfiConverterUint64INSTANCE.Write(writer, value.Timestamp)
	FfiConverterPaymentMethodINSTANCE.Write(writer, value.Method)
	FfiConverterOptionalPaymentDetailsINSTANCE.Write(writer, value.Details)
}

type FfiDestroyerPayment struct{}

func (_ FfiDestroyerPayment) Destroy(value Payment) {
	value.Destroy()
}

// Metadata associated with a payment that cannot be extracted from the Spark operator.
type PaymentMetadata struct {
	LnurlPayInfo *LnurlPayInfo
}

func (r *PaymentMetadata) Destroy() {
	FfiDestroyerOptionalLnurlPayInfo{}.Destroy(r.LnurlPayInfo)
}

type FfiConverterPaymentMetadata struct{}

var FfiConverterPaymentMetadataINSTANCE = FfiConverterPaymentMetadata{}

func (c FfiConverterPaymentMetadata) Lift(rb RustBufferI) PaymentMetadata {
	return LiftFromRustBuffer[PaymentMetadata](c, rb)
}

func (c FfiConverterPaymentMetadata) Read(reader io.Reader) PaymentMetadata {
	return PaymentMetadata{
		FfiConverterOptionalLnurlPayInfoINSTANCE.Read(reader),
	}
}

func (c FfiConverterPaymentMetadata) Lower(value PaymentMetadata) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentMetadata](c, value)
}

func (c FfiConverterPaymentMetadata) Write(writer io.Writer, value PaymentMetadata) {
	FfiConverterOptionalLnurlPayInfoINSTANCE.Write(writer, value.LnurlPayInfo)
}

type FfiDestroyerPaymentMetadata struct{}

func (_ FfiDestroyerPaymentMetadata) Destroy(value PaymentMetadata) {
	value.Destroy()
}

type PrepareLnurlPayRequest struct {
	AmountSats               uint64
	PayRequest               breez_sdk_common.LnurlPayRequestDetails
	Comment                  *string
	ValidateSuccessActionUrl *bool
}

func (r *PrepareLnurlPayRequest) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.AmountSats)
	breez_sdk_common.FfiDestroyerLnurlPayRequestDetails{}.Destroy(r.PayRequest)
	FfiDestroyerOptionalString{}.Destroy(r.Comment)
	FfiDestroyerOptionalBool{}.Destroy(r.ValidateSuccessActionUrl)
}

type FfiConverterPrepareLnurlPayRequest struct{}

var FfiConverterPrepareLnurlPayRequestINSTANCE = FfiConverterPrepareLnurlPayRequest{}

func (c FfiConverterPrepareLnurlPayRequest) Lift(rb RustBufferI) PrepareLnurlPayRequest {
	return LiftFromRustBuffer[PrepareLnurlPayRequest](c, rb)
}

func (c FfiConverterPrepareLnurlPayRequest) Read(reader io.Reader) PrepareLnurlPayRequest {
	return PrepareLnurlPayRequest{
		FfiConverterUint64INSTANCE.Read(reader),
		breez_sdk_common.FfiConverterLnurlPayRequestDetailsINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterPrepareLnurlPayRequest) Lower(value PrepareLnurlPayRequest) C.RustBuffer {
	return LowerIntoRustBuffer[PrepareLnurlPayRequest](c, value)
}

func (c FfiConverterPrepareLnurlPayRequest) Write(writer io.Writer, value PrepareLnurlPayRequest) {
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSats)
	breez_sdk_common.FfiConverterLnurlPayRequestDetailsINSTANCE.Write(writer, value.PayRequest)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Comment)
	FfiConverterOptionalBoolINSTANCE.Write(writer, value.ValidateSuccessActionUrl)
}

type FfiDestroyerPrepareLnurlPayRequest struct{}

func (_ FfiDestroyerPrepareLnurlPayRequest) Destroy(value PrepareLnurlPayRequest) {
	value.Destroy()
}

type PrepareLnurlPayResponse struct {
	AmountSats     uint64
	Comment        *string
	PayRequest     breez_sdk_common.LnurlPayRequestDetails
	FeeSats        uint64
	InvoiceDetails breez_sdk_common.Bolt11InvoiceDetails
	SuccessAction  *breez_sdk_common.SuccessAction
}

func (r *PrepareLnurlPayResponse) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.AmountSats)
	FfiDestroyerOptionalString{}.Destroy(r.Comment)
	breez_sdk_common.FfiDestroyerLnurlPayRequestDetails{}.Destroy(r.PayRequest)
	FfiDestroyerUint64{}.Destroy(r.FeeSats)
	breez_sdk_common.FfiDestroyerBolt11InvoiceDetails{}.Destroy(r.InvoiceDetails)
	FfiDestroyerOptionalSuccessAction{}.Destroy(r.SuccessAction)
}

type FfiConverterPrepareLnurlPayResponse struct{}

var FfiConverterPrepareLnurlPayResponseINSTANCE = FfiConverterPrepareLnurlPayResponse{}

func (c FfiConverterPrepareLnurlPayResponse) Lift(rb RustBufferI) PrepareLnurlPayResponse {
	return LiftFromRustBuffer[PrepareLnurlPayResponse](c, rb)
}

func (c FfiConverterPrepareLnurlPayResponse) Read(reader io.Reader) PrepareLnurlPayResponse {
	return PrepareLnurlPayResponse{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		breez_sdk_common.FfiConverterLnurlPayRequestDetailsINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		breez_sdk_common.FfiConverterBolt11InvoiceDetailsINSTANCE.Read(reader),
		FfiConverterOptionalSuccessActionINSTANCE.Read(reader),
	}
}

func (c FfiConverterPrepareLnurlPayResponse) Lower(value PrepareLnurlPayResponse) C.RustBuffer {
	return LowerIntoRustBuffer[PrepareLnurlPayResponse](c, value)
}

func (c FfiConverterPrepareLnurlPayResponse) Write(writer io.Writer, value PrepareLnurlPayResponse) {
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSats)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Comment)
	breez_sdk_common.FfiConverterLnurlPayRequestDetailsINSTANCE.Write(writer, value.PayRequest)
	FfiConverterUint64INSTANCE.Write(writer, value.FeeSats)
	breez_sdk_common.FfiConverterBolt11InvoiceDetailsINSTANCE.Write(writer, value.InvoiceDetails)
	FfiConverterOptionalSuccessActionINSTANCE.Write(writer, value.SuccessAction)
}

type FfiDestroyerPrepareLnurlPayResponse struct{}

func (_ FfiDestroyerPrepareLnurlPayResponse) Destroy(value PrepareLnurlPayResponse) {
	value.Destroy()
}

type PrepareSendPaymentRequest struct {
	PaymentRequest string
	AmountSats     *uint64
}

func (r *PrepareSendPaymentRequest) Destroy() {
	FfiDestroyerString{}.Destroy(r.PaymentRequest)
	FfiDestroyerOptionalUint64{}.Destroy(r.AmountSats)
}

type FfiConverterPrepareSendPaymentRequest struct{}

var FfiConverterPrepareSendPaymentRequestINSTANCE = FfiConverterPrepareSendPaymentRequest{}

func (c FfiConverterPrepareSendPaymentRequest) Lift(rb RustBufferI) PrepareSendPaymentRequest {
	return LiftFromRustBuffer[PrepareSendPaymentRequest](c, rb)
}

func (c FfiConverterPrepareSendPaymentRequest) Read(reader io.Reader) PrepareSendPaymentRequest {
	return PrepareSendPaymentRequest{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterPrepareSendPaymentRequest) Lower(value PrepareSendPaymentRequest) C.RustBuffer {
	return LowerIntoRustBuffer[PrepareSendPaymentRequest](c, value)
}

func (c FfiConverterPrepareSendPaymentRequest) Write(writer io.Writer, value PrepareSendPaymentRequest) {
	FfiConverterStringINSTANCE.Write(writer, value.PaymentRequest)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.AmountSats)
}

type FfiDestroyerPrepareSendPaymentRequest struct{}

func (_ FfiDestroyerPrepareSendPaymentRequest) Destroy(value PrepareSendPaymentRequest) {
	value.Destroy()
}

type PrepareSendPaymentResponse struct {
	PaymentMethod SendPaymentMethod
	AmountSats    uint64
}

func (r *PrepareSendPaymentResponse) Destroy() {
	FfiDestroyerSendPaymentMethod{}.Destroy(r.PaymentMethod)
	FfiDestroyerUint64{}.Destroy(r.AmountSats)
}

type FfiConverterPrepareSendPaymentResponse struct{}

var FfiConverterPrepareSendPaymentResponseINSTANCE = FfiConverterPrepareSendPaymentResponse{}

func (c FfiConverterPrepareSendPaymentResponse) Lift(rb RustBufferI) PrepareSendPaymentResponse {
	return LiftFromRustBuffer[PrepareSendPaymentResponse](c, rb)
}

func (c FfiConverterPrepareSendPaymentResponse) Read(reader io.Reader) PrepareSendPaymentResponse {
	return PrepareSendPaymentResponse{
		FfiConverterSendPaymentMethodINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterPrepareSendPaymentResponse) Lower(value PrepareSendPaymentResponse) C.RustBuffer {
	return LowerIntoRustBuffer[PrepareSendPaymentResponse](c, value)
}

func (c FfiConverterPrepareSendPaymentResponse) Write(writer io.Writer, value PrepareSendPaymentResponse) {
	FfiConverterSendPaymentMethodINSTANCE.Write(writer, value.PaymentMethod)
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSats)
}

type FfiDestroyerPrepareSendPaymentResponse struct{}

func (_ FfiDestroyerPrepareSendPaymentResponse) Destroy(value PrepareSendPaymentResponse) {
	value.Destroy()
}

type ReceivePaymentRequest struct {
	PaymentMethod ReceivePaymentMethod
}

func (r *ReceivePaymentRequest) Destroy() {
	FfiDestroyerReceivePaymentMethod{}.Destroy(r.PaymentMethod)
}

type FfiConverterReceivePaymentRequest struct{}

var FfiConverterReceivePaymentRequestINSTANCE = FfiConverterReceivePaymentRequest{}

func (c FfiConverterReceivePaymentRequest) Lift(rb RustBufferI) ReceivePaymentRequest {
	return LiftFromRustBuffer[ReceivePaymentRequest](c, rb)
}

func (c FfiConverterReceivePaymentRequest) Read(reader io.Reader) ReceivePaymentRequest {
	return ReceivePaymentRequest{
		FfiConverterReceivePaymentMethodINSTANCE.Read(reader),
	}
}

func (c FfiConverterReceivePaymentRequest) Lower(value ReceivePaymentRequest) C.RustBuffer {
	return LowerIntoRustBuffer[ReceivePaymentRequest](c, value)
}

func (c FfiConverterReceivePaymentRequest) Write(writer io.Writer, value ReceivePaymentRequest) {
	FfiConverterReceivePaymentMethodINSTANCE.Write(writer, value.PaymentMethod)
}

type FfiDestroyerReceivePaymentRequest struct{}

func (_ FfiDestroyerReceivePaymentRequest) Destroy(value ReceivePaymentRequest) {
	value.Destroy()
}

type ReceivePaymentResponse struct {
	PaymentRequest string
	FeeSats        uint64
}

func (r *ReceivePaymentResponse) Destroy() {
	FfiDestroyerString{}.Destroy(r.PaymentRequest)
	FfiDestroyerUint64{}.Destroy(r.FeeSats)
}

type FfiConverterReceivePaymentResponse struct{}

var FfiConverterReceivePaymentResponseINSTANCE = FfiConverterReceivePaymentResponse{}

func (c FfiConverterReceivePaymentResponse) Lift(rb RustBufferI) ReceivePaymentResponse {
	return LiftFromRustBuffer[ReceivePaymentResponse](c, rb)
}

func (c FfiConverterReceivePaymentResponse) Read(reader io.Reader) ReceivePaymentResponse {
	return ReceivePaymentResponse{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterReceivePaymentResponse) Lower(value ReceivePaymentResponse) C.RustBuffer {
	return LowerIntoRustBuffer[ReceivePaymentResponse](c, value)
}

func (c FfiConverterReceivePaymentResponse) Write(writer io.Writer, value ReceivePaymentResponse) {
	FfiConverterStringINSTANCE.Write(writer, value.PaymentRequest)
	FfiConverterUint64INSTANCE.Write(writer, value.FeeSats)
}

type FfiDestroyerReceivePaymentResponse struct{}

func (_ FfiDestroyerReceivePaymentResponse) Destroy(value ReceivePaymentResponse) {
	value.Destroy()
}

type RefundDepositRequest struct {
	Txid               string
	Vout               uint32
	DestinationAddress string
	Fee                Fee
}

func (r *RefundDepositRequest) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Vout)
	FfiDestroyerString{}.Destroy(r.DestinationAddress)
	FfiDestroyerFee{}.Destroy(r.Fee)
}

type FfiConverterRefundDepositRequest struct{}

var FfiConverterRefundDepositRequestINSTANCE = FfiConverterRefundDepositRequest{}

func (c FfiConverterRefundDepositRequest) Lift(rb RustBufferI) RefundDepositRequest {
	return LiftFromRustBuffer[RefundDepositRequest](c, rb)
}

func (c FfiConverterRefundDepositRequest) Read(reader io.Reader) RefundDepositRequest {
	return RefundDepositRequest{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterFeeINSTANCE.Read(reader),
	}
}

func (c FfiConverterRefundDepositRequest) Lower(value RefundDepositRequest) C.RustBuffer {
	return LowerIntoRustBuffer[RefundDepositRequest](c, value)
}

func (c FfiConverterRefundDepositRequest) Write(writer io.Writer, value RefundDepositRequest) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
	FfiConverterStringINSTANCE.Write(writer, value.DestinationAddress)
	FfiConverterFeeINSTANCE.Write(writer, value.Fee)
}

type FfiDestroyerRefundDepositRequest struct{}

func (_ FfiDestroyerRefundDepositRequest) Destroy(value RefundDepositRequest) {
	value.Destroy()
}

type RefundDepositResponse struct {
	TxId  string
	TxHex string
}

func (r *RefundDepositResponse) Destroy() {
	FfiDestroyerString{}.Destroy(r.TxId)
	FfiDestroyerString{}.Destroy(r.TxHex)
}

type FfiConverterRefundDepositResponse struct{}

var FfiConverterRefundDepositResponseINSTANCE = FfiConverterRefundDepositResponse{}

func (c FfiConverterRefundDepositResponse) Lift(rb RustBufferI) RefundDepositResponse {
	return LiftFromRustBuffer[RefundDepositResponse](c, rb)
}

func (c FfiConverterRefundDepositResponse) Read(reader io.Reader) RefundDepositResponse {
	return RefundDepositResponse{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterRefundDepositResponse) Lower(value RefundDepositResponse) C.RustBuffer {
	return LowerIntoRustBuffer[RefundDepositResponse](c, value)
}

func (c FfiConverterRefundDepositResponse) Write(writer io.Writer, value RefundDepositResponse) {
	FfiConverterStringINSTANCE.Write(writer, value.TxId)
	FfiConverterStringINSTANCE.Write(writer, value.TxHex)
}

type FfiDestroyerRefundDepositResponse struct{}

func (_ FfiDestroyerRefundDepositResponse) Destroy(value RefundDepositResponse) {
	value.Destroy()
}

type SendOnchainFeeQuote struct {
	Id          string
	ExpiresAt   uint64
	SpeedFast   SendOnchainSpeedFeeQuote
	SpeedMedium SendOnchainSpeedFeeQuote
	SpeedSlow   SendOnchainSpeedFeeQuote
}

func (r *SendOnchainFeeQuote) Destroy() {
	FfiDestroyerString{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.ExpiresAt)
	FfiDestroyerSendOnchainSpeedFeeQuote{}.Destroy(r.SpeedFast)
	FfiDestroyerSendOnchainSpeedFeeQuote{}.Destroy(r.SpeedMedium)
	FfiDestroyerSendOnchainSpeedFeeQuote{}.Destroy(r.SpeedSlow)
}

type FfiConverterSendOnchainFeeQuote struct{}

var FfiConverterSendOnchainFeeQuoteINSTANCE = FfiConverterSendOnchainFeeQuote{}

func (c FfiConverterSendOnchainFeeQuote) Lift(rb RustBufferI) SendOnchainFeeQuote {
	return LiftFromRustBuffer[SendOnchainFeeQuote](c, rb)
}

func (c FfiConverterSendOnchainFeeQuote) Read(reader io.Reader) SendOnchainFeeQuote {
	return SendOnchainFeeQuote{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterSendOnchainSpeedFeeQuoteINSTANCE.Read(reader),
		FfiConverterSendOnchainSpeedFeeQuoteINSTANCE.Read(reader),
		FfiConverterSendOnchainSpeedFeeQuoteINSTANCE.Read(reader),
	}
}

func (c FfiConverterSendOnchainFeeQuote) Lower(value SendOnchainFeeQuote) C.RustBuffer {
	return LowerIntoRustBuffer[SendOnchainFeeQuote](c, value)
}

func (c FfiConverterSendOnchainFeeQuote) Write(writer io.Writer, value SendOnchainFeeQuote) {
	FfiConverterStringINSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.ExpiresAt)
	FfiConverterSendOnchainSpeedFeeQuoteINSTANCE.Write(writer, value.SpeedFast)
	FfiConverterSendOnchainSpeedFeeQuoteINSTANCE.Write(writer, value.SpeedMedium)
	FfiConverterSendOnchainSpeedFeeQuoteINSTANCE.Write(writer, value.SpeedSlow)
}

type FfiDestroyerSendOnchainFeeQuote struct{}

func (_ FfiDestroyerSendOnchainFeeQuote) Destroy(value SendOnchainFeeQuote) {
	value.Destroy()
}

type SendOnchainSpeedFeeQuote struct {
	UserFeeSat        uint64
	L1BroadcastFeeSat uint64
}

func (r *SendOnchainSpeedFeeQuote) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.UserFeeSat)
	FfiDestroyerUint64{}.Destroy(r.L1BroadcastFeeSat)
}

type FfiConverterSendOnchainSpeedFeeQuote struct{}

var FfiConverterSendOnchainSpeedFeeQuoteINSTANCE = FfiConverterSendOnchainSpeedFeeQuote{}

func (c FfiConverterSendOnchainSpeedFeeQuote) Lift(rb RustBufferI) SendOnchainSpeedFeeQuote {
	return LiftFromRustBuffer[SendOnchainSpeedFeeQuote](c, rb)
}

func (c FfiConverterSendOnchainSpeedFeeQuote) Read(reader io.Reader) SendOnchainSpeedFeeQuote {
	return SendOnchainSpeedFeeQuote{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterSendOnchainSpeedFeeQuote) Lower(value SendOnchainSpeedFeeQuote) C.RustBuffer {
	return LowerIntoRustBuffer[SendOnchainSpeedFeeQuote](c, value)
}

func (c FfiConverterSendOnchainSpeedFeeQuote) Write(writer io.Writer, value SendOnchainSpeedFeeQuote) {
	FfiConverterUint64INSTANCE.Write(writer, value.UserFeeSat)
	FfiConverterUint64INSTANCE.Write(writer, value.L1BroadcastFeeSat)
}

type FfiDestroyerSendOnchainSpeedFeeQuote struct{}

func (_ FfiDestroyerSendOnchainSpeedFeeQuote) Destroy(value SendOnchainSpeedFeeQuote) {
	value.Destroy()
}

type SendPaymentRequest struct {
	PrepareResponse PrepareSendPaymentResponse
	Options         *SendPaymentOptions
}

func (r *SendPaymentRequest) Destroy() {
	FfiDestroyerPrepareSendPaymentResponse{}.Destroy(r.PrepareResponse)
	FfiDestroyerOptionalSendPaymentOptions{}.Destroy(r.Options)
}

type FfiConverterSendPaymentRequest struct{}

var FfiConverterSendPaymentRequestINSTANCE = FfiConverterSendPaymentRequest{}

func (c FfiConverterSendPaymentRequest) Lift(rb RustBufferI) SendPaymentRequest {
	return LiftFromRustBuffer[SendPaymentRequest](c, rb)
}

func (c FfiConverterSendPaymentRequest) Read(reader io.Reader) SendPaymentRequest {
	return SendPaymentRequest{
		FfiConverterPrepareSendPaymentResponseINSTANCE.Read(reader),
		FfiConverterOptionalSendPaymentOptionsINSTANCE.Read(reader),
	}
}

func (c FfiConverterSendPaymentRequest) Lower(value SendPaymentRequest) C.RustBuffer {
	return LowerIntoRustBuffer[SendPaymentRequest](c, value)
}

func (c FfiConverterSendPaymentRequest) Write(writer io.Writer, value SendPaymentRequest) {
	FfiConverterPrepareSendPaymentResponseINSTANCE.Write(writer, value.PrepareResponse)
	FfiConverterOptionalSendPaymentOptionsINSTANCE.Write(writer, value.Options)
}

type FfiDestroyerSendPaymentRequest struct{}

func (_ FfiDestroyerSendPaymentRequest) Destroy(value SendPaymentRequest) {
	value.Destroy()
}

type SendPaymentResponse struct {
	Payment Payment
}

func (r *SendPaymentResponse) Destroy() {
	FfiDestroyerPayment{}.Destroy(r.Payment)
}

type FfiConverterSendPaymentResponse struct{}

var FfiConverterSendPaymentResponseINSTANCE = FfiConverterSendPaymentResponse{}

func (c FfiConverterSendPaymentResponse) Lift(rb RustBufferI) SendPaymentResponse {
	return LiftFromRustBuffer[SendPaymentResponse](c, rb)
}

func (c FfiConverterSendPaymentResponse) Read(reader io.Reader) SendPaymentResponse {
	return SendPaymentResponse{
		FfiConverterPaymentINSTANCE.Read(reader),
	}
}

func (c FfiConverterSendPaymentResponse) Lower(value SendPaymentResponse) C.RustBuffer {
	return LowerIntoRustBuffer[SendPaymentResponse](c, value)
}

func (c FfiConverterSendPaymentResponse) Write(writer io.Writer, value SendPaymentResponse) {
	FfiConverterPaymentINSTANCE.Write(writer, value.Payment)
}

type FfiDestroyerSendPaymentResponse struct{}

func (_ FfiDestroyerSendPaymentResponse) Destroy(value SendPaymentResponse) {
	value.Destroy()
}

// Request to sync the wallet with the Spark network
type SyncWalletRequest struct {
}

func (r *SyncWalletRequest) Destroy() {
}

type FfiConverterSyncWalletRequest struct{}

var FfiConverterSyncWalletRequestINSTANCE = FfiConverterSyncWalletRequest{}

func (c FfiConverterSyncWalletRequest) Lift(rb RustBufferI) SyncWalletRequest {
	return LiftFromRustBuffer[SyncWalletRequest](c, rb)
}

func (c FfiConverterSyncWalletRequest) Read(reader io.Reader) SyncWalletRequest {
	return SyncWalletRequest{}
}

func (c FfiConverterSyncWalletRequest) Lower(value SyncWalletRequest) C.RustBuffer {
	return LowerIntoRustBuffer[SyncWalletRequest](c, value)
}

func (c FfiConverterSyncWalletRequest) Write(writer io.Writer, value SyncWalletRequest) {
}

type FfiDestroyerSyncWalletRequest struct{}

func (_ FfiDestroyerSyncWalletRequest) Destroy(value SyncWalletRequest) {
	value.Destroy()
}

// Response from synchronizing the wallet
type SyncWalletResponse struct {
}

func (r *SyncWalletResponse) Destroy() {
}

type FfiConverterSyncWalletResponse struct{}

var FfiConverterSyncWalletResponseINSTANCE = FfiConverterSyncWalletResponse{}

func (c FfiConverterSyncWalletResponse) Lift(rb RustBufferI) SyncWalletResponse {
	return LiftFromRustBuffer[SyncWalletResponse](c, rb)
}

func (c FfiConverterSyncWalletResponse) Read(reader io.Reader) SyncWalletResponse {
	return SyncWalletResponse{}
}

func (c FfiConverterSyncWalletResponse) Lower(value SyncWalletResponse) C.RustBuffer {
	return LowerIntoRustBuffer[SyncWalletResponse](c, value)
}

func (c FfiConverterSyncWalletResponse) Write(writer io.Writer, value SyncWalletResponse) {
}

type FfiDestroyerSyncWalletResponse struct{}

func (_ FfiDestroyerSyncWalletResponse) Destroy(value SyncWalletResponse) {
	value.Destroy()
}

type TxStatus struct {
	Confirmed   bool
	BlockHeight *uint32
	BlockTime   *uint64
}

func (r *TxStatus) Destroy() {
	FfiDestroyerBool{}.Destroy(r.Confirmed)
	FfiDestroyerOptionalUint32{}.Destroy(r.BlockHeight)
	FfiDestroyerOptionalUint64{}.Destroy(r.BlockTime)
}

type FfiConverterTxStatus struct{}

var FfiConverterTxStatusINSTANCE = FfiConverterTxStatus{}

func (c FfiConverterTxStatus) Lift(rb RustBufferI) TxStatus {
	return LiftFromRustBuffer[TxStatus](c, rb)
}

func (c FfiConverterTxStatus) Read(reader io.Reader) TxStatus {
	return TxStatus{
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTxStatus) Lower(value TxStatus) C.RustBuffer {
	return LowerIntoRustBuffer[TxStatus](c, value)
}

func (c FfiConverterTxStatus) Write(writer io.Writer, value TxStatus) {
	FfiConverterBoolINSTANCE.Write(writer, value.Confirmed)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.BlockHeight)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.BlockTime)
}

type FfiDestroyerTxStatus struct{}

func (_ FfiDestroyerTxStatus) Destroy(value TxStatus) {
	value.Destroy()
}

type Utxo struct {
	Txid   string
	Vout   uint32
	Value  uint64
	Status TxStatus
}

func (r *Utxo) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Vout)
	FfiDestroyerUint64{}.Destroy(r.Value)
	FfiDestroyerTxStatus{}.Destroy(r.Status)
}

type FfiConverterUtxo struct{}

var FfiConverterUtxoINSTANCE = FfiConverterUtxo{}

func (c FfiConverterUtxo) Lift(rb RustBufferI) Utxo {
	return LiftFromRustBuffer[Utxo](c, rb)
}

func (c FfiConverterUtxo) Read(reader io.Reader) Utxo {
	return Utxo{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterTxStatusINSTANCE.Read(reader),
	}
}

func (c FfiConverterUtxo) Lower(value Utxo) C.RustBuffer {
	return LowerIntoRustBuffer[Utxo](c, value)
}

func (c FfiConverterUtxo) Write(writer io.Writer, value Utxo) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
	FfiConverterUint64INSTANCE.Write(writer, value.Value)
	FfiConverterTxStatusINSTANCE.Write(writer, value.Status)
}

type FfiDestroyerUtxo struct{}

func (_ FfiDestroyerUtxo) Destroy(value Utxo) {
	value.Destroy()
}

type ChainServiceError struct {
	err error
}

// Convience method to turn *ChainServiceError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ChainServiceError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ChainServiceError) Error() string {
	return fmt.Sprintf("ChainServiceError: %s", err.err.Error())
}

func (err ChainServiceError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrChainServiceErrorInvalidAddress = fmt.Errorf("ChainServiceErrorInvalidAddress")
var ErrChainServiceErrorServiceConnectivity = fmt.Errorf("ChainServiceErrorServiceConnectivity")
var ErrChainServiceErrorGeneric = fmt.Errorf("ChainServiceErrorGeneric")

// Variant structs
type ChainServiceErrorInvalidAddress struct {
	Field0 string
}

func NewChainServiceErrorInvalidAddress(
	var0 string,
) *ChainServiceError {
	return &ChainServiceError{err: &ChainServiceErrorInvalidAddress{
		Field0: var0}}
}

func (e ChainServiceErrorInvalidAddress) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ChainServiceErrorInvalidAddress) Error() string {
	return fmt.Sprint("InvalidAddress",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ChainServiceErrorInvalidAddress) Is(target error) bool {
	return target == ErrChainServiceErrorInvalidAddress
}

type ChainServiceErrorServiceConnectivity struct {
	Field0 string
}

func NewChainServiceErrorServiceConnectivity(
	var0 string,
) *ChainServiceError {
	return &ChainServiceError{err: &ChainServiceErrorServiceConnectivity{
		Field0: var0}}
}

func (e ChainServiceErrorServiceConnectivity) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ChainServiceErrorServiceConnectivity) Error() string {
	return fmt.Sprint("ServiceConnectivity",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ChainServiceErrorServiceConnectivity) Is(target error) bool {
	return target == ErrChainServiceErrorServiceConnectivity
}

type ChainServiceErrorGeneric struct {
	Field0 string
}

func NewChainServiceErrorGeneric(
	var0 string,
) *ChainServiceError {
	return &ChainServiceError{err: &ChainServiceErrorGeneric{
		Field0: var0}}
}

func (e ChainServiceErrorGeneric) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ChainServiceErrorGeneric) Error() string {
	return fmt.Sprint("Generic",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ChainServiceErrorGeneric) Is(target error) bool {
	return target == ErrChainServiceErrorGeneric
}

type FfiConverterChainServiceError struct{}

var FfiConverterChainServiceErrorINSTANCE = FfiConverterChainServiceError{}

func (c FfiConverterChainServiceError) Lift(eb RustBufferI) *ChainServiceError {
	return LiftFromRustBuffer[*ChainServiceError](c, eb)
}

func (c FfiConverterChainServiceError) Lower(value *ChainServiceError) C.RustBuffer {
	return LowerIntoRustBuffer[*ChainServiceError](c, value)
}

func (c FfiConverterChainServiceError) Read(reader io.Reader) *ChainServiceError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &ChainServiceError{&ChainServiceErrorInvalidAddress{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &ChainServiceError{&ChainServiceErrorServiceConnectivity{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &ChainServiceError{&ChainServiceErrorGeneric{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterChainServiceError.Read()", errorID))
	}
}

func (c FfiConverterChainServiceError) Write(writer io.Writer, value *ChainServiceError) {
	switch variantValue := value.err.(type) {
	case *ChainServiceErrorInvalidAddress:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ChainServiceErrorServiceConnectivity:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ChainServiceErrorGeneric:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterChainServiceError.Write", value))
	}
}

type FfiDestroyerChainServiceError struct{}

func (_ FfiDestroyerChainServiceError) Destroy(value *ChainServiceError) {
	switch variantValue := value.err.(type) {
	case ChainServiceErrorInvalidAddress:
		variantValue.destroy()
	case ChainServiceErrorServiceConnectivity:
		variantValue.destroy()
	case ChainServiceErrorGeneric:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerChainServiceError.Destroy", value))
	}
}

type DepositClaimError interface {
	Destroy()
}
type DepositClaimErrorDepositClaimFeeExceeded struct {
	Tx        string
	Vout      uint32
	MaxFee    Fee
	ActualFee uint64
}

func (e DepositClaimErrorDepositClaimFeeExceeded) Destroy() {
	FfiDestroyerString{}.Destroy(e.Tx)
	FfiDestroyerUint32{}.Destroy(e.Vout)
	FfiDestroyerFee{}.Destroy(e.MaxFee)
	FfiDestroyerUint64{}.Destroy(e.ActualFee)
}

type DepositClaimErrorMissingUtxo struct {
	Tx   string
	Vout uint32
}

func (e DepositClaimErrorMissingUtxo) Destroy() {
	FfiDestroyerString{}.Destroy(e.Tx)
	FfiDestroyerUint32{}.Destroy(e.Vout)
}

type DepositClaimErrorGeneric struct {
	Message string
}

func (e DepositClaimErrorGeneric) Destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

type FfiConverterDepositClaimError struct{}

var FfiConverterDepositClaimErrorINSTANCE = FfiConverterDepositClaimError{}

func (c FfiConverterDepositClaimError) Lift(rb RustBufferI) DepositClaimError {
	return LiftFromRustBuffer[DepositClaimError](c, rb)
}

func (c FfiConverterDepositClaimError) Lower(value DepositClaimError) C.RustBuffer {
	return LowerIntoRustBuffer[DepositClaimError](c, value)
}
func (FfiConverterDepositClaimError) Read(reader io.Reader) DepositClaimError {
	id := readInt32(reader)
	switch id {
	case 1:
		return DepositClaimErrorDepositClaimFeeExceeded{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
			FfiConverterFeeINSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 2:
		return DepositClaimErrorMissingUtxo{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
		}
	case 3:
		return DepositClaimErrorGeneric{
			FfiConverterStringINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterDepositClaimError.Read()", id))
	}
}

func (FfiConverterDepositClaimError) Write(writer io.Writer, value DepositClaimError) {
	switch variant_value := value.(type) {
	case DepositClaimErrorDepositClaimFeeExceeded:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Tx)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Vout)
		FfiConverterFeeINSTANCE.Write(writer, variant_value.MaxFee)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.ActualFee)
	case DepositClaimErrorMissingUtxo:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Tx)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Vout)
	case DepositClaimErrorGeneric:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Message)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterDepositClaimError.Write", value))
	}
}

type FfiDestroyerDepositClaimError struct{}

func (_ FfiDestroyerDepositClaimError) Destroy(value DepositClaimError) {
	value.Destroy()
}

type Fee interface {
	Destroy()
}
type FeeFixed struct {
	Amount uint64
}

func (e FeeFixed) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.Amount)
}

type FeeRate struct {
	SatPerVbyte uint64
}

func (e FeeRate) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.SatPerVbyte)
}

type FfiConverterFee struct{}

var FfiConverterFeeINSTANCE = FfiConverterFee{}

func (c FfiConverterFee) Lift(rb RustBufferI) Fee {
	return LiftFromRustBuffer[Fee](c, rb)
}

func (c FfiConverterFee) Lower(value Fee) C.RustBuffer {
	return LowerIntoRustBuffer[Fee](c, value)
}
func (FfiConverterFee) Read(reader io.Reader) Fee {
	id := readInt32(reader)
	switch id {
	case 1:
		return FeeFixed{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 2:
		return FeeRate{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterFee.Read()", id))
	}
}

func (FfiConverterFee) Write(writer io.Writer, value Fee) {
	switch variant_value := value.(type) {
	case FeeFixed:
		writeInt32(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Amount)
	case FeeRate:
		writeInt32(writer, 2)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.SatPerVbyte)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterFee.Write", value))
	}
}

type FfiDestroyerFee struct{}

func (_ FfiDestroyerFee) Destroy(value Fee) {
	value.Destroy()
}

type Network uint

const (
	NetworkMainnet Network = 1
	NetworkRegtest Network = 2
)

type FfiConverterNetwork struct{}

var FfiConverterNetworkINSTANCE = FfiConverterNetwork{}

func (c FfiConverterNetwork) Lift(rb RustBufferI) Network {
	return LiftFromRustBuffer[Network](c, rb)
}

func (c FfiConverterNetwork) Lower(value Network) C.RustBuffer {
	return LowerIntoRustBuffer[Network](c, value)
}
func (FfiConverterNetwork) Read(reader io.Reader) Network {
	id := readInt32(reader)
	return Network(id)
}

func (FfiConverterNetwork) Write(writer io.Writer, value Network) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerNetwork struct{}

func (_ FfiDestroyerNetwork) Destroy(value Network) {
}

type OnchainConfirmationSpeed uint

const (
	OnchainConfirmationSpeedFast   OnchainConfirmationSpeed = 1
	OnchainConfirmationSpeedMedium OnchainConfirmationSpeed = 2
	OnchainConfirmationSpeedSlow   OnchainConfirmationSpeed = 3
)

type FfiConverterOnchainConfirmationSpeed struct{}

var FfiConverterOnchainConfirmationSpeedINSTANCE = FfiConverterOnchainConfirmationSpeed{}

func (c FfiConverterOnchainConfirmationSpeed) Lift(rb RustBufferI) OnchainConfirmationSpeed {
	return LiftFromRustBuffer[OnchainConfirmationSpeed](c, rb)
}

func (c FfiConverterOnchainConfirmationSpeed) Lower(value OnchainConfirmationSpeed) C.RustBuffer {
	return LowerIntoRustBuffer[OnchainConfirmationSpeed](c, value)
}
func (FfiConverterOnchainConfirmationSpeed) Read(reader io.Reader) OnchainConfirmationSpeed {
	id := readInt32(reader)
	return OnchainConfirmationSpeed(id)
}

func (FfiConverterOnchainConfirmationSpeed) Write(writer io.Writer, value OnchainConfirmationSpeed) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerOnchainConfirmationSpeed struct{}

func (_ FfiDestroyerOnchainConfirmationSpeed) Destroy(value OnchainConfirmationSpeed) {
}

type PaymentDetails interface {
	Destroy()
}
type PaymentDetailsSpark struct {
}

func (e PaymentDetailsSpark) Destroy() {
}

type PaymentDetailsLightning struct {
	Description       *string
	Preimage          *string
	Invoice           string
	PaymentHash       string
	DestinationPubkey string
	LnurlPayInfo      *LnurlPayInfo
}

func (e PaymentDetailsLightning) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(e.Description)
	FfiDestroyerOptionalString{}.Destroy(e.Preimage)
	FfiDestroyerString{}.Destroy(e.Invoice)
	FfiDestroyerString{}.Destroy(e.PaymentHash)
	FfiDestroyerString{}.Destroy(e.DestinationPubkey)
	FfiDestroyerOptionalLnurlPayInfo{}.Destroy(e.LnurlPayInfo)
}

type PaymentDetailsWithdraw struct {
	TxId string
}

func (e PaymentDetailsWithdraw) Destroy() {
	FfiDestroyerString{}.Destroy(e.TxId)
}

type PaymentDetailsDeposit struct {
	TxId string
}

func (e PaymentDetailsDeposit) Destroy() {
	FfiDestroyerString{}.Destroy(e.TxId)
}

type FfiConverterPaymentDetails struct{}

var FfiConverterPaymentDetailsINSTANCE = FfiConverterPaymentDetails{}

func (c FfiConverterPaymentDetails) Lift(rb RustBufferI) PaymentDetails {
	return LiftFromRustBuffer[PaymentDetails](c, rb)
}

func (c FfiConverterPaymentDetails) Lower(value PaymentDetails) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentDetails](c, value)
}
func (FfiConverterPaymentDetails) Read(reader io.Reader) PaymentDetails {
	id := readInt32(reader)
	switch id {
	case 1:
		return PaymentDetailsSpark{}
	case 2:
		return PaymentDetailsLightning{
			FfiConverterOptionalStringINSTANCE.Read(reader),
			FfiConverterOptionalStringINSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterOptionalLnurlPayInfoINSTANCE.Read(reader),
		}
	case 3:
		return PaymentDetailsWithdraw{
			FfiConverterStringINSTANCE.Read(reader),
		}
	case 4:
		return PaymentDetailsDeposit{
			FfiConverterStringINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterPaymentDetails.Read()", id))
	}
}

func (FfiConverterPaymentDetails) Write(writer io.Writer, value PaymentDetails) {
	switch variant_value := value.(type) {
	case PaymentDetailsSpark:
		writeInt32(writer, 1)
	case PaymentDetailsLightning:
		writeInt32(writer, 2)
		FfiConverterOptionalStringINSTANCE.Write(writer, variant_value.Description)
		FfiConverterOptionalStringINSTANCE.Write(writer, variant_value.Preimage)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Invoice)
		FfiConverterStringINSTANCE.Write(writer, variant_value.PaymentHash)
		FfiConverterStringINSTANCE.Write(writer, variant_value.DestinationPubkey)
		FfiConverterOptionalLnurlPayInfoINSTANCE.Write(writer, variant_value.LnurlPayInfo)
	case PaymentDetailsWithdraw:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variant_value.TxId)
	case PaymentDetailsDeposit:
		writeInt32(writer, 4)
		FfiConverterStringINSTANCE.Write(writer, variant_value.TxId)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterPaymentDetails.Write", value))
	}
}

type FfiDestroyerPaymentDetails struct{}

func (_ FfiDestroyerPaymentDetails) Destroy(value PaymentDetails) {
	value.Destroy()
}

type PaymentMethod uint

const (
	PaymentMethodLightning PaymentMethod = 1
	PaymentMethodSpark     PaymentMethod = 2
	PaymentMethodDeposit   PaymentMethod = 3
	PaymentMethodWithdraw  PaymentMethod = 4
	PaymentMethodUnknown   PaymentMethod = 5
)

type FfiConverterPaymentMethod struct{}

var FfiConverterPaymentMethodINSTANCE = FfiConverterPaymentMethod{}

func (c FfiConverterPaymentMethod) Lift(rb RustBufferI) PaymentMethod {
	return LiftFromRustBuffer[PaymentMethod](c, rb)
}

func (c FfiConverterPaymentMethod) Lower(value PaymentMethod) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentMethod](c, value)
}
func (FfiConverterPaymentMethod) Read(reader io.Reader) PaymentMethod {
	id := readInt32(reader)
	return PaymentMethod(id)
}

func (FfiConverterPaymentMethod) Write(writer io.Writer, value PaymentMethod) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerPaymentMethod struct{}

func (_ FfiDestroyerPaymentMethod) Destroy(value PaymentMethod) {
}

// The status of a payment
type PaymentStatus uint

const (
	// Payment is completed successfully
	PaymentStatusCompleted PaymentStatus = 1
	// Payment is in progress
	PaymentStatusPending PaymentStatus = 2
	// Payment has failed
	PaymentStatusFailed PaymentStatus = 3
)

type FfiConverterPaymentStatus struct{}

var FfiConverterPaymentStatusINSTANCE = FfiConverterPaymentStatus{}

func (c FfiConverterPaymentStatus) Lift(rb RustBufferI) PaymentStatus {
	return LiftFromRustBuffer[PaymentStatus](c, rb)
}

func (c FfiConverterPaymentStatus) Lower(value PaymentStatus) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentStatus](c, value)
}
func (FfiConverterPaymentStatus) Read(reader io.Reader) PaymentStatus {
	id := readInt32(reader)
	return PaymentStatus(id)
}

func (FfiConverterPaymentStatus) Write(writer io.Writer, value PaymentStatus) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerPaymentStatus struct{}

func (_ FfiDestroyerPaymentStatus) Destroy(value PaymentStatus) {
}

// The type of payment
type PaymentType uint

const (
	// Payment sent from this wallet
	PaymentTypeSend PaymentType = 1
	// Payment received to this wallet
	PaymentTypeReceive PaymentType = 2
)

type FfiConverterPaymentType struct{}

var FfiConverterPaymentTypeINSTANCE = FfiConverterPaymentType{}

func (c FfiConverterPaymentType) Lift(rb RustBufferI) PaymentType {
	return LiftFromRustBuffer[PaymentType](c, rb)
}

func (c FfiConverterPaymentType) Lower(value PaymentType) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentType](c, value)
}
func (FfiConverterPaymentType) Read(reader io.Reader) PaymentType {
	id := readInt32(reader)
	return PaymentType(id)
}

func (FfiConverterPaymentType) Write(writer io.Writer, value PaymentType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerPaymentType struct{}

func (_ FfiDestroyerPaymentType) Destroy(value PaymentType) {
}

type ReceivePaymentMethod interface {
	Destroy()
}
type ReceivePaymentMethodSparkAddress struct {
}

func (e ReceivePaymentMethodSparkAddress) Destroy() {
}

type ReceivePaymentMethodBitcoinAddress struct {
}

func (e ReceivePaymentMethodBitcoinAddress) Destroy() {
}

type ReceivePaymentMethodBolt11Invoice struct {
	Description string
	AmountSats  *uint64
}

func (e ReceivePaymentMethodBolt11Invoice) Destroy() {
	FfiDestroyerString{}.Destroy(e.Description)
	FfiDestroyerOptionalUint64{}.Destroy(e.AmountSats)
}

type FfiConverterReceivePaymentMethod struct{}

var FfiConverterReceivePaymentMethodINSTANCE = FfiConverterReceivePaymentMethod{}

func (c FfiConverterReceivePaymentMethod) Lift(rb RustBufferI) ReceivePaymentMethod {
	return LiftFromRustBuffer[ReceivePaymentMethod](c, rb)
}

func (c FfiConverterReceivePaymentMethod) Lower(value ReceivePaymentMethod) C.RustBuffer {
	return LowerIntoRustBuffer[ReceivePaymentMethod](c, value)
}
func (FfiConverterReceivePaymentMethod) Read(reader io.Reader) ReceivePaymentMethod {
	id := readInt32(reader)
	switch id {
	case 1:
		return ReceivePaymentMethodSparkAddress{}
	case 2:
		return ReceivePaymentMethodBitcoinAddress{}
	case 3:
		return ReceivePaymentMethodBolt11Invoice{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterOptionalUint64INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterReceivePaymentMethod.Read()", id))
	}
}

func (FfiConverterReceivePaymentMethod) Write(writer io.Writer, value ReceivePaymentMethod) {
	switch variant_value := value.(type) {
	case ReceivePaymentMethodSparkAddress:
		writeInt32(writer, 1)
	case ReceivePaymentMethodBitcoinAddress:
		writeInt32(writer, 2)
	case ReceivePaymentMethodBolt11Invoice:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Description)
		FfiConverterOptionalUint64INSTANCE.Write(writer, variant_value.AmountSats)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterReceivePaymentMethod.Write", value))
	}
}

type FfiDestroyerReceivePaymentMethod struct{}

func (_ FfiDestroyerReceivePaymentMethod) Destroy(value ReceivePaymentMethod) {
	value.Destroy()
}

// Error type for the `BreezSdk`
type SdkError struct {
	err error
}

// Convience method to turn *SdkError into error
// Avoiding treating nil pointer as non nil error interface
func (err *SdkError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err SdkError) Error() string {
	return fmt.Sprintf("SdkError: %s", err.err.Error())
}

func (err SdkError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrSdkErrorSparkError = fmt.Errorf("SdkErrorSparkError")
var ErrSdkErrorInvalidUuid = fmt.Errorf("SdkErrorInvalidUuid")
var ErrSdkErrorInvalidInput = fmt.Errorf("SdkErrorInvalidInput")
var ErrSdkErrorNetworkError = fmt.Errorf("SdkErrorNetworkError")
var ErrSdkErrorStorageError = fmt.Errorf("SdkErrorStorageError")
var ErrSdkErrorChainServiceError = fmt.Errorf("SdkErrorChainServiceError")
var ErrSdkErrorDepositClaimFeeExceeded = fmt.Errorf("SdkErrorDepositClaimFeeExceeded")
var ErrSdkErrorMissingUtxo = fmt.Errorf("SdkErrorMissingUtxo")
var ErrSdkErrorLnurlError = fmt.Errorf("SdkErrorLnurlError")
var ErrSdkErrorGeneric = fmt.Errorf("SdkErrorGeneric")

// Variant structs
type SdkErrorSparkError struct {
	Field0 string
}

func NewSdkErrorSparkError(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorSparkError{
		Field0: var0}}
}

func (e SdkErrorSparkError) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorSparkError) Error() string {
	return fmt.Sprint("SparkError",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorSparkError) Is(target error) bool {
	return target == ErrSdkErrorSparkError
}

type SdkErrorInvalidUuid struct {
	Field0 string
}

func NewSdkErrorInvalidUuid(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorInvalidUuid{
		Field0: var0}}
}

func (e SdkErrorInvalidUuid) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorInvalidUuid) Error() string {
	return fmt.Sprint("InvalidUuid",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorInvalidUuid) Is(target error) bool {
	return target == ErrSdkErrorInvalidUuid
}

// Invalid input error
type SdkErrorInvalidInput struct {
	Field0 string
}

// Invalid input error
func NewSdkErrorInvalidInput(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorInvalidInput{
		Field0: var0}}
}

func (e SdkErrorInvalidInput) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorInvalidInput) Error() string {
	return fmt.Sprint("InvalidInput",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorInvalidInput) Is(target error) bool {
	return target == ErrSdkErrorInvalidInput
}

// Network error
type SdkErrorNetworkError struct {
	Field0 string
}

// Network error
func NewSdkErrorNetworkError(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorNetworkError{
		Field0: var0}}
}

func (e SdkErrorNetworkError) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorNetworkError) Error() string {
	return fmt.Sprint("NetworkError",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorNetworkError) Is(target error) bool {
	return target == ErrSdkErrorNetworkError
}

// Storage error
type SdkErrorStorageError struct {
	Field0 string
}

// Storage error
func NewSdkErrorStorageError(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorStorageError{
		Field0: var0}}
}

func (e SdkErrorStorageError) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorStorageError) Error() string {
	return fmt.Sprint("StorageError",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorStorageError) Is(target error) bool {
	return target == ErrSdkErrorStorageError
}

type SdkErrorChainServiceError struct {
	Field0 string
}

func NewSdkErrorChainServiceError(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorChainServiceError{
		Field0: var0}}
}

func (e SdkErrorChainServiceError) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorChainServiceError) Error() string {
	return fmt.Sprint("ChainServiceError",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorChainServiceError) Is(target error) bool {
	return target == ErrSdkErrorChainServiceError
}

type SdkErrorDepositClaimFeeExceeded struct {
	Tx        string
	Vout      uint32
	MaxFee    Fee
	ActualFee uint64
}

func NewSdkErrorDepositClaimFeeExceeded(
	tx string,
	vout uint32,
	maxFee Fee,
	actualFee uint64,
) *SdkError {
	return &SdkError{err: &SdkErrorDepositClaimFeeExceeded{
		Tx:        tx,
		Vout:      vout,
		MaxFee:    maxFee,
		ActualFee: actualFee}}
}

func (e SdkErrorDepositClaimFeeExceeded) destroy() {
	FfiDestroyerString{}.Destroy(e.Tx)
	FfiDestroyerUint32{}.Destroy(e.Vout)
	FfiDestroyerFee{}.Destroy(e.MaxFee)
	FfiDestroyerUint64{}.Destroy(e.ActualFee)
}

func (err SdkErrorDepositClaimFeeExceeded) Error() string {
	return fmt.Sprint("DepositClaimFeeExceeded",
		": ",

		"Tx=",
		err.Tx,
		", ",
		"Vout=",
		err.Vout,
		", ",
		"MaxFee=",
		err.MaxFee,
		", ",
		"ActualFee=",
		err.ActualFee,
	)
}

func (self SdkErrorDepositClaimFeeExceeded) Is(target error) bool {
	return target == ErrSdkErrorDepositClaimFeeExceeded
}

type SdkErrorMissingUtxo struct {
	Tx   string
	Vout uint32
}

func NewSdkErrorMissingUtxo(
	tx string,
	vout uint32,
) *SdkError {
	return &SdkError{err: &SdkErrorMissingUtxo{
		Tx:   tx,
		Vout: vout}}
}

func (e SdkErrorMissingUtxo) destroy() {
	FfiDestroyerString{}.Destroy(e.Tx)
	FfiDestroyerUint32{}.Destroy(e.Vout)
}

func (err SdkErrorMissingUtxo) Error() string {
	return fmt.Sprint("MissingUtxo",
		": ",

		"Tx=",
		err.Tx,
		", ",
		"Vout=",
		err.Vout,
	)
}

func (self SdkErrorMissingUtxo) Is(target error) bool {
	return target == ErrSdkErrorMissingUtxo
}

type SdkErrorLnurlError struct {
	Field0 string
}

func NewSdkErrorLnurlError(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorLnurlError{
		Field0: var0}}
}

func (e SdkErrorLnurlError) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorLnurlError) Error() string {
	return fmt.Sprint("LnurlError",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorLnurlError) Is(target error) bool {
	return target == ErrSdkErrorLnurlError
}

type SdkErrorGeneric struct {
	Field0 string
}

func NewSdkErrorGeneric(
	var0 string,
) *SdkError {
	return &SdkError{err: &SdkErrorGeneric{
		Field0: var0}}
}

func (e SdkErrorGeneric) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err SdkErrorGeneric) Error() string {
	return fmt.Sprint("Generic",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self SdkErrorGeneric) Is(target error) bool {
	return target == ErrSdkErrorGeneric
}

type FfiConverterSdkError struct{}

var FfiConverterSdkErrorINSTANCE = FfiConverterSdkError{}

func (c FfiConverterSdkError) Lift(eb RustBufferI) *SdkError {
	return LiftFromRustBuffer[*SdkError](c, eb)
}

func (c FfiConverterSdkError) Lower(value *SdkError) C.RustBuffer {
	return LowerIntoRustBuffer[*SdkError](c, value)
}

func (c FfiConverterSdkError) Read(reader io.Reader) *SdkError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &SdkError{&SdkErrorSparkError{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &SdkError{&SdkErrorInvalidUuid{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &SdkError{&SdkErrorInvalidInput{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 4:
		return &SdkError{&SdkErrorNetworkError{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 5:
		return &SdkError{&SdkErrorStorageError{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 6:
		return &SdkError{&SdkErrorChainServiceError{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 7:
		return &SdkError{&SdkErrorDepositClaimFeeExceeded{
			Tx:        FfiConverterStringINSTANCE.Read(reader),
			Vout:      FfiConverterUint32INSTANCE.Read(reader),
			MaxFee:    FfiConverterFeeINSTANCE.Read(reader),
			ActualFee: FfiConverterUint64INSTANCE.Read(reader),
		}}
	case 8:
		return &SdkError{&SdkErrorMissingUtxo{
			Tx:   FfiConverterStringINSTANCE.Read(reader),
			Vout: FfiConverterUint32INSTANCE.Read(reader),
		}}
	case 9:
		return &SdkError{&SdkErrorLnurlError{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 10:
		return &SdkError{&SdkErrorGeneric{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterSdkError.Read()", errorID))
	}
}

func (c FfiConverterSdkError) Write(writer io.Writer, value *SdkError) {
	switch variantValue := value.err.(type) {
	case *SdkErrorSparkError:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorInvalidUuid:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorInvalidInput:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorNetworkError:
		writeInt32(writer, 4)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorStorageError:
		writeInt32(writer, 5)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorChainServiceError:
		writeInt32(writer, 6)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorDepositClaimFeeExceeded:
		writeInt32(writer, 7)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Tx)
		FfiConverterUint32INSTANCE.Write(writer, variantValue.Vout)
		FfiConverterFeeINSTANCE.Write(writer, variantValue.MaxFee)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.ActualFee)
	case *SdkErrorMissingUtxo:
		writeInt32(writer, 8)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Tx)
		FfiConverterUint32INSTANCE.Write(writer, variantValue.Vout)
	case *SdkErrorLnurlError:
		writeInt32(writer, 9)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *SdkErrorGeneric:
		writeInt32(writer, 10)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterSdkError.Write", value))
	}
}

type FfiDestroyerSdkError struct{}

func (_ FfiDestroyerSdkError) Destroy(value *SdkError) {
	switch variantValue := value.err.(type) {
	case SdkErrorSparkError:
		variantValue.destroy()
	case SdkErrorInvalidUuid:
		variantValue.destroy()
	case SdkErrorInvalidInput:
		variantValue.destroy()
	case SdkErrorNetworkError:
		variantValue.destroy()
	case SdkErrorStorageError:
		variantValue.destroy()
	case SdkErrorChainServiceError:
		variantValue.destroy()
	case SdkErrorDepositClaimFeeExceeded:
		variantValue.destroy()
	case SdkErrorMissingUtxo:
		variantValue.destroy()
	case SdkErrorLnurlError:
		variantValue.destroy()
	case SdkErrorGeneric:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerSdkError.Destroy", value))
	}
}

// Events emitted by the SDK
type SdkEvent interface {
	Destroy()
}

// Emitted when the wallet has been synchronized with the network
type SdkEventSynced struct {
}

func (e SdkEventSynced) Destroy() {
}

// Emitted when the wallet failed to claim some deposits
type SdkEventClaimDepositsFailed struct {
	UnclaimedDeposits []DepositInfo
}

func (e SdkEventClaimDepositsFailed) Destroy() {
	FfiDestroyerSequenceDepositInfo{}.Destroy(e.UnclaimedDeposits)
}

type SdkEventClaimDepositsSucceeded struct {
	ClaimedDeposits []DepositInfo
}

func (e SdkEventClaimDepositsSucceeded) Destroy() {
	FfiDestroyerSequenceDepositInfo{}.Destroy(e.ClaimedDeposits)
}

type SdkEventPaymentSucceeded struct {
	Payment Payment
}

func (e SdkEventPaymentSucceeded) Destroy() {
	FfiDestroyerPayment{}.Destroy(e.Payment)
}

type FfiConverterSdkEvent struct{}

var FfiConverterSdkEventINSTANCE = FfiConverterSdkEvent{}

func (c FfiConverterSdkEvent) Lift(rb RustBufferI) SdkEvent {
	return LiftFromRustBuffer[SdkEvent](c, rb)
}

func (c FfiConverterSdkEvent) Lower(value SdkEvent) C.RustBuffer {
	return LowerIntoRustBuffer[SdkEvent](c, value)
}
func (FfiConverterSdkEvent) Read(reader io.Reader) SdkEvent {
	id := readInt32(reader)
	switch id {
	case 1:
		return SdkEventSynced{}
	case 2:
		return SdkEventClaimDepositsFailed{
			FfiConverterSequenceDepositInfoINSTANCE.Read(reader),
		}
	case 3:
		return SdkEventClaimDepositsSucceeded{
			FfiConverterSequenceDepositInfoINSTANCE.Read(reader),
		}
	case 4:
		return SdkEventPaymentSucceeded{
			FfiConverterPaymentINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSdkEvent.Read()", id))
	}
}

func (FfiConverterSdkEvent) Write(writer io.Writer, value SdkEvent) {
	switch variant_value := value.(type) {
	case SdkEventSynced:
		writeInt32(writer, 1)
	case SdkEventClaimDepositsFailed:
		writeInt32(writer, 2)
		FfiConverterSequenceDepositInfoINSTANCE.Write(writer, variant_value.UnclaimedDeposits)
	case SdkEventClaimDepositsSucceeded:
		writeInt32(writer, 3)
		FfiConverterSequenceDepositInfoINSTANCE.Write(writer, variant_value.ClaimedDeposits)
	case SdkEventPaymentSucceeded:
		writeInt32(writer, 4)
		FfiConverterPaymentINSTANCE.Write(writer, variant_value.Payment)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSdkEvent.Write", value))
	}
}

type FfiDestroyerSdkEvent struct{}

func (_ FfiDestroyerSdkEvent) Destroy(value SdkEvent) {
	value.Destroy()
}

type SendPaymentMethod interface {
	Destroy()
}
type SendPaymentMethodBitcoinAddress struct {
	Address  breez_sdk_common.BitcoinAddressDetails
	FeeQuote SendOnchainFeeQuote
}

func (e SendPaymentMethodBitcoinAddress) Destroy() {
	breez_sdk_common.FfiDestroyerBitcoinAddressDetails{}.Destroy(e.Address)
	FfiDestroyerSendOnchainFeeQuote{}.Destroy(e.FeeQuote)
}

type SendPaymentMethodBolt11Invoice struct {
	InvoiceDetails       breez_sdk_common.Bolt11InvoiceDetails
	SparkTransferFeeSats *uint64
	LightningFeeSats     uint64
}

func (e SendPaymentMethodBolt11Invoice) Destroy() {
	breez_sdk_common.FfiDestroyerBolt11InvoiceDetails{}.Destroy(e.InvoiceDetails)
	FfiDestroyerOptionalUint64{}.Destroy(e.SparkTransferFeeSats)
	FfiDestroyerUint64{}.Destroy(e.LightningFeeSats)
}

type SendPaymentMethodSparkAddress struct {
	Address string
	FeeSats uint64
}

func (e SendPaymentMethodSparkAddress) Destroy() {
	FfiDestroyerString{}.Destroy(e.Address)
	FfiDestroyerUint64{}.Destroy(e.FeeSats)
}

type FfiConverterSendPaymentMethod struct{}

var FfiConverterSendPaymentMethodINSTANCE = FfiConverterSendPaymentMethod{}

func (c FfiConverterSendPaymentMethod) Lift(rb RustBufferI) SendPaymentMethod {
	return LiftFromRustBuffer[SendPaymentMethod](c, rb)
}

func (c FfiConverterSendPaymentMethod) Lower(value SendPaymentMethod) C.RustBuffer {
	return LowerIntoRustBuffer[SendPaymentMethod](c, value)
}
func (FfiConverterSendPaymentMethod) Read(reader io.Reader) SendPaymentMethod {
	id := readInt32(reader)
	switch id {
	case 1:
		return SendPaymentMethodBitcoinAddress{
			breez_sdk_common.FfiConverterBitcoinAddressDetailsINSTANCE.Read(reader),
			FfiConverterSendOnchainFeeQuoteINSTANCE.Read(reader),
		}
	case 2:
		return SendPaymentMethodBolt11Invoice{
			breez_sdk_common.FfiConverterBolt11InvoiceDetailsINSTANCE.Read(reader),
			FfiConverterOptionalUint64INSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 3:
		return SendPaymentMethodSparkAddress{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSendPaymentMethod.Read()", id))
	}
}

func (FfiConverterSendPaymentMethod) Write(writer io.Writer, value SendPaymentMethod) {
	switch variant_value := value.(type) {
	case SendPaymentMethodBitcoinAddress:
		writeInt32(writer, 1)
		breez_sdk_common.FfiConverterBitcoinAddressDetailsINSTANCE.Write(writer, variant_value.Address)
		FfiConverterSendOnchainFeeQuoteINSTANCE.Write(writer, variant_value.FeeQuote)
	case SendPaymentMethodBolt11Invoice:
		writeInt32(writer, 2)
		breez_sdk_common.FfiConverterBolt11InvoiceDetailsINSTANCE.Write(writer, variant_value.InvoiceDetails)
		FfiConverterOptionalUint64INSTANCE.Write(writer, variant_value.SparkTransferFeeSats)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.LightningFeeSats)
	case SendPaymentMethodSparkAddress:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Address)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.FeeSats)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSendPaymentMethod.Write", value))
	}
}

type FfiDestroyerSendPaymentMethod struct{}

func (_ FfiDestroyerSendPaymentMethod) Destroy(value SendPaymentMethod) {
	value.Destroy()
}

type SendPaymentOptions interface {
	Destroy()
}
type SendPaymentOptionsBitcoinAddress struct {
	ConfirmationSpeed OnchainConfirmationSpeed
}

func (e SendPaymentOptionsBitcoinAddress) Destroy() {
	FfiDestroyerOnchainConfirmationSpeed{}.Destroy(e.ConfirmationSpeed)
}

type SendPaymentOptionsBolt11Invoice struct {
	UseSpark bool
}

func (e SendPaymentOptionsBolt11Invoice) Destroy() {
	FfiDestroyerBool{}.Destroy(e.UseSpark)
}

type FfiConverterSendPaymentOptions struct{}

var FfiConverterSendPaymentOptionsINSTANCE = FfiConverterSendPaymentOptions{}

func (c FfiConverterSendPaymentOptions) Lift(rb RustBufferI) SendPaymentOptions {
	return LiftFromRustBuffer[SendPaymentOptions](c, rb)
}

func (c FfiConverterSendPaymentOptions) Lower(value SendPaymentOptions) C.RustBuffer {
	return LowerIntoRustBuffer[SendPaymentOptions](c, value)
}
func (FfiConverterSendPaymentOptions) Read(reader io.Reader) SendPaymentOptions {
	id := readInt32(reader)
	switch id {
	case 1:
		return SendPaymentOptionsBitcoinAddress{
			FfiConverterOnchainConfirmationSpeedINSTANCE.Read(reader),
		}
	case 2:
		return SendPaymentOptionsBolt11Invoice{
			FfiConverterBoolINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSendPaymentOptions.Read()", id))
	}
}

func (FfiConverterSendPaymentOptions) Write(writer io.Writer, value SendPaymentOptions) {
	switch variant_value := value.(type) {
	case SendPaymentOptionsBitcoinAddress:
		writeInt32(writer, 1)
		FfiConverterOnchainConfirmationSpeedINSTANCE.Write(writer, variant_value.ConfirmationSpeed)
	case SendPaymentOptionsBolt11Invoice:
		writeInt32(writer, 2)
		FfiConverterBoolINSTANCE.Write(writer, variant_value.UseSpark)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSendPaymentOptions.Write", value))
	}
}

type FfiDestroyerSendPaymentOptions struct{}

func (_ FfiDestroyerSendPaymentOptions) Destroy(value SendPaymentOptions) {
	value.Destroy()
}

// Errors that can occur during storage operations
type StorageError struct {
	err error
}

// Convience method to turn *StorageError into error
// Avoiding treating nil pointer as non nil error interface
func (err *StorageError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err StorageError) Error() string {
	return fmt.Sprintf("StorageError: %s", err.err.Error())
}

func (err StorageError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrStorageErrorImplementation = fmt.Errorf("StorageErrorImplementation")
var ErrStorageErrorInitializationError = fmt.Errorf("StorageErrorInitializationError")
var ErrStorageErrorSerialization = fmt.Errorf("StorageErrorSerialization")

// Variant structs
type StorageErrorImplementation struct {
	Field0 string
}

func NewStorageErrorImplementation(
	var0 string,
) *StorageError {
	return &StorageError{err: &StorageErrorImplementation{
		Field0: var0}}
}

func (e StorageErrorImplementation) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err StorageErrorImplementation) Error() string {
	return fmt.Sprint("Implementation",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self StorageErrorImplementation) Is(target error) bool {
	return target == ErrStorageErrorImplementation
}

// Database initialization error
type StorageErrorInitializationError struct {
	Field0 string
}

// Database initialization error
func NewStorageErrorInitializationError(
	var0 string,
) *StorageError {
	return &StorageError{err: &StorageErrorInitializationError{
		Field0: var0}}
}

func (e StorageErrorInitializationError) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err StorageErrorInitializationError) Error() string {
	return fmt.Sprint("InitializationError",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self StorageErrorInitializationError) Is(target error) bool {
	return target == ErrStorageErrorInitializationError
}

type StorageErrorSerialization struct {
	Field0 string
}

func NewStorageErrorSerialization(
	var0 string,
) *StorageError {
	return &StorageError{err: &StorageErrorSerialization{
		Field0: var0}}
}

func (e StorageErrorSerialization) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err StorageErrorSerialization) Error() string {
	return fmt.Sprint("Serialization",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self StorageErrorSerialization) Is(target error) bool {
	return target == ErrStorageErrorSerialization
}

type FfiConverterStorageError struct{}

var FfiConverterStorageErrorINSTANCE = FfiConverterStorageError{}

func (c FfiConverterStorageError) Lift(eb RustBufferI) *StorageError {
	return LiftFromRustBuffer[*StorageError](c, eb)
}

func (c FfiConverterStorageError) Lower(value *StorageError) C.RustBuffer {
	return LowerIntoRustBuffer[*StorageError](c, value)
}

func (c FfiConverterStorageError) Read(reader io.Reader) *StorageError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &StorageError{&StorageErrorImplementation{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &StorageError{&StorageErrorInitializationError{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &StorageError{&StorageErrorSerialization{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterStorageError.Read()", errorID))
	}
}

func (c FfiConverterStorageError) Write(writer io.Writer, value *StorageError) {
	switch variantValue := value.err.(type) {
	case *StorageErrorImplementation:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *StorageErrorInitializationError:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *StorageErrorSerialization:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterStorageError.Write", value))
	}
}

type FfiDestroyerStorageError struct{}

func (_ FfiDestroyerStorageError) Destroy(value *StorageError) {
	switch variantValue := value.err.(type) {
	case StorageErrorImplementation:
		variantValue.destroy()
	case StorageErrorInitializationError:
		variantValue.destroy()
	case StorageErrorSerialization:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerStorageError.Destroy", value))
	}
}

type UpdateDepositPayload interface {
	Destroy()
}
type UpdateDepositPayloadClaimError struct {
	Error DepositClaimError
}

func (e UpdateDepositPayloadClaimError) Destroy() {
	FfiDestroyerDepositClaimError{}.Destroy(e.Error)
}

type UpdateDepositPayloadRefund struct {
	RefundTxid string
	RefundTx   string
}

func (e UpdateDepositPayloadRefund) Destroy() {
	FfiDestroyerString{}.Destroy(e.RefundTxid)
	FfiDestroyerString{}.Destroy(e.RefundTx)
}

type FfiConverterUpdateDepositPayload struct{}

var FfiConverterUpdateDepositPayloadINSTANCE = FfiConverterUpdateDepositPayload{}

func (c FfiConverterUpdateDepositPayload) Lift(rb RustBufferI) UpdateDepositPayload {
	return LiftFromRustBuffer[UpdateDepositPayload](c, rb)
}

func (c FfiConverterUpdateDepositPayload) Lower(value UpdateDepositPayload) C.RustBuffer {
	return LowerIntoRustBuffer[UpdateDepositPayload](c, value)
}
func (FfiConverterUpdateDepositPayload) Read(reader io.Reader) UpdateDepositPayload {
	id := readInt32(reader)
	switch id {
	case 1:
		return UpdateDepositPayloadClaimError{
			FfiConverterDepositClaimErrorINSTANCE.Read(reader),
		}
	case 2:
		return UpdateDepositPayloadRefund{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterStringINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterUpdateDepositPayload.Read()", id))
	}
}

func (FfiConverterUpdateDepositPayload) Write(writer io.Writer, value UpdateDepositPayload) {
	switch variant_value := value.(type) {
	case UpdateDepositPayloadClaimError:
		writeInt32(writer, 1)
		FfiConverterDepositClaimErrorINSTANCE.Write(writer, variant_value.Error)
	case UpdateDepositPayloadRefund:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variant_value.RefundTxid)
		FfiConverterStringINSTANCE.Write(writer, variant_value.RefundTx)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterUpdateDepositPayload.Write", value))
	}
}

type FfiDestroyerUpdateDepositPayload struct{}

func (_ FfiDestroyerUpdateDepositPayload) Destroy(value UpdateDepositPayload) {
	value.Destroy()
}

// Trait for event listeners
type EventListener interface {

	// Called when an event occurs
	OnEvent(event SdkEvent)
}

type FfiConverterCallbackInterfaceEventListener struct {
	handleMap *concurrentHandleMap[EventListener]
}

var FfiConverterCallbackInterfaceEventListenerINSTANCE = FfiConverterCallbackInterfaceEventListener{
	handleMap: newConcurrentHandleMap[EventListener](),
}

func (c FfiConverterCallbackInterfaceEventListener) Lift(handle uint64) EventListener {
	val, ok := c.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	return val
}

func (c FfiConverterCallbackInterfaceEventListener) Read(reader io.Reader) EventListener {
	return c.Lift(readUint64(reader))
}

func (c FfiConverterCallbackInterfaceEventListener) Lower(value EventListener) C.uint64_t {
	return C.uint64_t(c.handleMap.insert(value))
}

func (c FfiConverterCallbackInterfaceEventListener) Write(writer io.Writer, value EventListener) {
	writeUint64(writer, uint64(c.Lower(value)))
}

type FfiDestroyerCallbackInterfaceEventListener struct{}

func (FfiDestroyerCallbackInterfaceEventListener) Destroy(value EventListener) {}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceEventListenerMethod0
func breez_sdk_spark_cgo_dispatchCallbackInterfaceEventListenerMethod0(uniffiHandle C.uint64_t, event C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterCallbackInterfaceEventListenerINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.OnEvent(
		FfiConverterSdkEventINSTANCE.Lift(GoRustBuffer{
			inner: event,
		}),
	)

}

var UniffiVTableCallbackInterfaceEventListenerINSTANCE = C.UniffiVTableCallbackInterfaceEventListener{
	onEvent: (C.UniffiCallbackInterfaceEventListenerMethod0)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceEventListenerMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceEventListenerFree),
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceEventListenerFree
func breez_sdk_spark_cgo_dispatchCallbackInterfaceEventListenerFree(handle C.uint64_t) {
	FfiConverterCallbackInterfaceEventListenerINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterCallbackInterfaceEventListener) register() {
	C.uniffi_breez_sdk_spark_fn_init_callback_vtable_eventlistener(&UniffiVTableCallbackInterfaceEventListenerINSTANCE)
}

type Logger interface {
	Log(l LogEntry)
}

type FfiConverterCallbackInterfaceLogger struct {
	handleMap *concurrentHandleMap[Logger]
}

var FfiConverterCallbackInterfaceLoggerINSTANCE = FfiConverterCallbackInterfaceLogger{
	handleMap: newConcurrentHandleMap[Logger](),
}

func (c FfiConverterCallbackInterfaceLogger) Lift(handle uint64) Logger {
	val, ok := c.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	return val
}

func (c FfiConverterCallbackInterfaceLogger) Read(reader io.Reader) Logger {
	return c.Lift(readUint64(reader))
}

func (c FfiConverterCallbackInterfaceLogger) Lower(value Logger) C.uint64_t {
	return C.uint64_t(c.handleMap.insert(value))
}

func (c FfiConverterCallbackInterfaceLogger) Write(writer io.Writer, value Logger) {
	writeUint64(writer, uint64(c.Lower(value)))
}

type FfiDestroyerCallbackInterfaceLogger struct{}

func (FfiDestroyerCallbackInterfaceLogger) Destroy(value Logger) {}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceLoggerMethod0
func breez_sdk_spark_cgo_dispatchCallbackInterfaceLoggerMethod0(uniffiHandle C.uint64_t, l C.RustBuffer, uniffiOutReturn *C.void, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterCallbackInterfaceLoggerINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	uniffiObj.Log(
		FfiConverterLogEntryINSTANCE.Lift(GoRustBuffer{
			inner: l,
		}),
	)

}

var UniffiVTableCallbackInterfaceLoggerINSTANCE = C.UniffiVTableCallbackInterfaceLogger{
	log: (C.UniffiCallbackInterfaceLoggerMethod0)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceLoggerMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.breez_sdk_spark_cgo_dispatchCallbackInterfaceLoggerFree),
}

//export breez_sdk_spark_cgo_dispatchCallbackInterfaceLoggerFree
func breez_sdk_spark_cgo_dispatchCallbackInterfaceLoggerFree(handle C.uint64_t) {
	FfiConverterCallbackInterfaceLoggerINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterCallbackInterfaceLogger) register() {
	C.uniffi_breez_sdk_spark_fn_init_callback_vtable_logger(&UniffiVTableCallbackInterfaceLoggerINSTANCE)
}

type FfiConverterOptionalUint32 struct{}

var FfiConverterOptionalUint32INSTANCE = FfiConverterOptionalUint32{}

func (c FfiConverterOptionalUint32) Lift(rb RustBufferI) *uint32 {
	return LiftFromRustBuffer[*uint32](c, rb)
}

func (_ FfiConverterOptionalUint32) Read(reader io.Reader) *uint32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint32) Lower(value *uint32) C.RustBuffer {
	return LowerIntoRustBuffer[*uint32](c, value)
}

func (_ FfiConverterOptionalUint32) Write(writer io.Writer, value *uint32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint32 struct{}

func (_ FfiDestroyerOptionalUint32) Destroy(value *uint32) {
	if value != nil {
		FfiDestroyerUint32{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) C.RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterOptionalBool struct{}

var FfiConverterOptionalBoolINSTANCE = FfiConverterOptionalBool{}

func (c FfiConverterOptionalBool) Lift(rb RustBufferI) *bool {
	return LiftFromRustBuffer[*bool](c, rb)
}

func (_ FfiConverterOptionalBool) Read(reader io.Reader) *bool {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBoolINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBool) Lower(value *bool) C.RustBuffer {
	return LowerIntoRustBuffer[*bool](c, value)
}

func (_ FfiConverterOptionalBool) Write(writer io.Writer, value *bool) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBoolINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBool struct{}

func (_ FfiDestroyerOptionalBool) Destroy(value *bool) {
	if value != nil {
		FfiDestroyerBool{}.Destroy(*value)
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterOptionalCredentials struct{}

var FfiConverterOptionalCredentialsINSTANCE = FfiConverterOptionalCredentials{}

func (c FfiConverterOptionalCredentials) Lift(rb RustBufferI) *Credentials {
	return LiftFromRustBuffer[*Credentials](c, rb)
}

func (_ FfiConverterOptionalCredentials) Read(reader io.Reader) *Credentials {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCredentialsINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCredentials) Lower(value *Credentials) C.RustBuffer {
	return LowerIntoRustBuffer[*Credentials](c, value)
}

func (_ FfiConverterOptionalCredentials) Write(writer io.Writer, value *Credentials) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCredentialsINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCredentials struct{}

func (_ FfiDestroyerOptionalCredentials) Destroy(value *Credentials) {
	if value != nil {
		FfiDestroyerCredentials{}.Destroy(*value)
	}
}

type FfiConverterOptionalLnurlPayInfo struct{}

var FfiConverterOptionalLnurlPayInfoINSTANCE = FfiConverterOptionalLnurlPayInfo{}

func (c FfiConverterOptionalLnurlPayInfo) Lift(rb RustBufferI) *LnurlPayInfo {
	return LiftFromRustBuffer[*LnurlPayInfo](c, rb)
}

func (_ FfiConverterOptionalLnurlPayInfo) Read(reader io.Reader) *LnurlPayInfo {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterLnurlPayInfoINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalLnurlPayInfo) Lower(value *LnurlPayInfo) C.RustBuffer {
	return LowerIntoRustBuffer[*LnurlPayInfo](c, value)
}

func (_ FfiConverterOptionalLnurlPayInfo) Write(writer io.Writer, value *LnurlPayInfo) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterLnurlPayInfoINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalLnurlPayInfo struct{}

func (_ FfiDestroyerOptionalLnurlPayInfo) Destroy(value *LnurlPayInfo) {
	if value != nil {
		FfiDestroyerLnurlPayInfo{}.Destroy(*value)
	}
}

type FfiConverterOptionalDepositClaimError struct{}

var FfiConverterOptionalDepositClaimErrorINSTANCE = FfiConverterOptionalDepositClaimError{}

func (c FfiConverterOptionalDepositClaimError) Lift(rb RustBufferI) *DepositClaimError {
	return LiftFromRustBuffer[*DepositClaimError](c, rb)
}

func (_ FfiConverterOptionalDepositClaimError) Read(reader io.Reader) *DepositClaimError {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterDepositClaimErrorINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalDepositClaimError) Lower(value *DepositClaimError) C.RustBuffer {
	return LowerIntoRustBuffer[*DepositClaimError](c, value)
}

func (_ FfiConverterOptionalDepositClaimError) Write(writer io.Writer, value *DepositClaimError) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterDepositClaimErrorINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalDepositClaimError struct{}

func (_ FfiDestroyerOptionalDepositClaimError) Destroy(value *DepositClaimError) {
	if value != nil {
		FfiDestroyerDepositClaimError{}.Destroy(*value)
	}
}

type FfiConverterOptionalFee struct{}

var FfiConverterOptionalFeeINSTANCE = FfiConverterOptionalFee{}

func (c FfiConverterOptionalFee) Lift(rb RustBufferI) *Fee {
	return LiftFromRustBuffer[*Fee](c, rb)
}

func (_ FfiConverterOptionalFee) Read(reader io.Reader) *Fee {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterFeeINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalFee) Lower(value *Fee) C.RustBuffer {
	return LowerIntoRustBuffer[*Fee](c, value)
}

func (_ FfiConverterOptionalFee) Write(writer io.Writer, value *Fee) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterFeeINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalFee struct{}

func (_ FfiDestroyerOptionalFee) Destroy(value *Fee) {
	if value != nil {
		FfiDestroyerFee{}.Destroy(*value)
	}
}

type FfiConverterOptionalPaymentDetails struct{}

var FfiConverterOptionalPaymentDetailsINSTANCE = FfiConverterOptionalPaymentDetails{}

func (c FfiConverterOptionalPaymentDetails) Lift(rb RustBufferI) *PaymentDetails {
	return LiftFromRustBuffer[*PaymentDetails](c, rb)
}

func (_ FfiConverterOptionalPaymentDetails) Read(reader io.Reader) *PaymentDetails {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterPaymentDetailsINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalPaymentDetails) Lower(value *PaymentDetails) C.RustBuffer {
	return LowerIntoRustBuffer[*PaymentDetails](c, value)
}

func (_ FfiConverterOptionalPaymentDetails) Write(writer io.Writer, value *PaymentDetails) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterPaymentDetailsINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalPaymentDetails struct{}

func (_ FfiDestroyerOptionalPaymentDetails) Destroy(value *PaymentDetails) {
	if value != nil {
		FfiDestroyerPaymentDetails{}.Destroy(*value)
	}
}

type FfiConverterOptionalSendPaymentOptions struct{}

var FfiConverterOptionalSendPaymentOptionsINSTANCE = FfiConverterOptionalSendPaymentOptions{}

func (c FfiConverterOptionalSendPaymentOptions) Lift(rb RustBufferI) *SendPaymentOptions {
	return LiftFromRustBuffer[*SendPaymentOptions](c, rb)
}

func (_ FfiConverterOptionalSendPaymentOptions) Read(reader io.Reader) *SendPaymentOptions {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSendPaymentOptionsINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSendPaymentOptions) Lower(value *SendPaymentOptions) C.RustBuffer {
	return LowerIntoRustBuffer[*SendPaymentOptions](c, value)
}

func (_ FfiConverterOptionalSendPaymentOptions) Write(writer io.Writer, value *SendPaymentOptions) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSendPaymentOptionsINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSendPaymentOptions struct{}

func (_ FfiDestroyerOptionalSendPaymentOptions) Destroy(value *SendPaymentOptions) {
	if value != nil {
		FfiDestroyerSendPaymentOptions{}.Destroy(*value)
	}
}

type FfiConverterOptionalCallbackInterfaceLogger struct{}

var FfiConverterOptionalCallbackInterfaceLoggerINSTANCE = FfiConverterOptionalCallbackInterfaceLogger{}

func (c FfiConverterOptionalCallbackInterfaceLogger) Lift(rb RustBufferI) *Logger {
	return LiftFromRustBuffer[*Logger](c, rb)
}

func (_ FfiConverterOptionalCallbackInterfaceLogger) Read(reader io.Reader) *Logger {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCallbackInterfaceLoggerINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCallbackInterfaceLogger) Lower(value *Logger) C.RustBuffer {
	return LowerIntoRustBuffer[*Logger](c, value)
}

func (_ FfiConverterOptionalCallbackInterfaceLogger) Write(writer io.Writer, value *Logger) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCallbackInterfaceLoggerINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCallbackInterfaceLogger struct{}

func (_ FfiDestroyerOptionalCallbackInterfaceLogger) Destroy(value *Logger) {
	if value != nil {
		FfiDestroyerCallbackInterfaceLogger{}.Destroy(*value)
	}
}

type FfiConverterOptionalSuccessAction struct{}

var FfiConverterOptionalSuccessActionINSTANCE = FfiConverterOptionalSuccessAction{}

func (c FfiConverterOptionalSuccessAction) Lift(rb RustBufferI) *breez_sdk_common.SuccessAction {
	return LiftFromRustBuffer[*breez_sdk_common.SuccessAction](c, rb)
}

func (_ FfiConverterOptionalSuccessAction) Read(reader io.Reader) *breez_sdk_common.SuccessAction {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := breez_sdk_common.FfiConverterSuccessActionINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSuccessAction) Lower(value *breez_sdk_common.SuccessAction) C.RustBuffer {
	return LowerIntoRustBuffer[*breez_sdk_common.SuccessAction](c, value)
}

func (_ FfiConverterOptionalSuccessAction) Write(writer io.Writer, value *breez_sdk_common.SuccessAction) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		breez_sdk_common.FfiConverterSuccessActionINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSuccessAction struct{}

func (_ FfiDestroyerOptionalSuccessAction) Destroy(value *breez_sdk_common.SuccessAction) {
	if value != nil {
		breez_sdk_common.FfiDestroyerSuccessAction{}.Destroy(*value)
	}
}

type FfiConverterOptionalSuccessActionProcessed struct{}

var FfiConverterOptionalSuccessActionProcessedINSTANCE = FfiConverterOptionalSuccessActionProcessed{}

func (c FfiConverterOptionalSuccessActionProcessed) Lift(rb RustBufferI) *breez_sdk_common.SuccessActionProcessed {
	return LiftFromRustBuffer[*breez_sdk_common.SuccessActionProcessed](c, rb)
}

func (_ FfiConverterOptionalSuccessActionProcessed) Read(reader io.Reader) *breez_sdk_common.SuccessActionProcessed {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := breez_sdk_common.FfiConverterSuccessActionProcessedINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSuccessActionProcessed) Lower(value *breez_sdk_common.SuccessActionProcessed) C.RustBuffer {
	return LowerIntoRustBuffer[*breez_sdk_common.SuccessActionProcessed](c, value)
}

func (_ FfiConverterOptionalSuccessActionProcessed) Write(writer io.Writer, value *breez_sdk_common.SuccessActionProcessed) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		breez_sdk_common.FfiConverterSuccessActionProcessedINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSuccessActionProcessed struct{}

func (_ FfiDestroyerOptionalSuccessActionProcessed) Destroy(value *breez_sdk_common.SuccessActionProcessed) {
	if value != nil {
		breez_sdk_common.FfiDestroyerSuccessActionProcessed{}.Destroy(*value)
	}
}

type FfiConverterSequenceDepositInfo struct{}

var FfiConverterSequenceDepositInfoINSTANCE = FfiConverterSequenceDepositInfo{}

func (c FfiConverterSequenceDepositInfo) Lift(rb RustBufferI) []DepositInfo {
	return LiftFromRustBuffer[[]DepositInfo](c, rb)
}

func (c FfiConverterSequenceDepositInfo) Read(reader io.Reader) []DepositInfo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]DepositInfo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterDepositInfoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceDepositInfo) Lower(value []DepositInfo) C.RustBuffer {
	return LowerIntoRustBuffer[[]DepositInfo](c, value)
}

func (c FfiConverterSequenceDepositInfo) Write(writer io.Writer, value []DepositInfo) {
	if len(value) > math.MaxInt32 {
		panic("[]DepositInfo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterDepositInfoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceDepositInfo struct{}

func (FfiDestroyerSequenceDepositInfo) Destroy(sequence []DepositInfo) {
	for _, value := range sequence {
		FfiDestroyerDepositInfo{}.Destroy(value)
	}
}

type FfiConverterSequencePayment struct{}

var FfiConverterSequencePaymentINSTANCE = FfiConverterSequencePayment{}

func (c FfiConverterSequencePayment) Lift(rb RustBufferI) []Payment {
	return LiftFromRustBuffer[[]Payment](c, rb)
}

func (c FfiConverterSequencePayment) Read(reader io.Reader) []Payment {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Payment, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterPaymentINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequencePayment) Lower(value []Payment) C.RustBuffer {
	return LowerIntoRustBuffer[[]Payment](c, value)
}

func (c FfiConverterSequencePayment) Write(writer io.Writer, value []Payment) {
	if len(value) > math.MaxInt32 {
		panic("[]Payment is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterPaymentINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequencePayment struct{}

func (FfiDestroyerSequencePayment) Destroy(sequence []Payment) {
	for _, value := range sequence {
		FfiDestroyerPayment{}.Destroy(value)
	}
}

type FfiConverterSequenceUtxo struct{}

var FfiConverterSequenceUtxoINSTANCE = FfiConverterSequenceUtxo{}

func (c FfiConverterSequenceUtxo) Lift(rb RustBufferI) []Utxo {
	return LiftFromRustBuffer[[]Utxo](c, rb)
}

func (c FfiConverterSequenceUtxo) Read(reader io.Reader) []Utxo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Utxo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUtxoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUtxo) Lower(value []Utxo) C.RustBuffer {
	return LowerIntoRustBuffer[[]Utxo](c, value)
}

func (c FfiConverterSequenceUtxo) Write(writer io.Writer, value []Utxo) {
	if len(value) > math.MaxInt32 {
		panic("[]Utxo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUtxoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUtxo struct{}

func (FfiDestroyerSequenceUtxo) Destroy(sequence []Utxo) {
	for _, value := range sequence {
		FfiDestroyerUtxo{}.Destroy(value)
	}
}

const (
	uniffiRustFuturePollReady      int8 = 0
	uniffiRustFuturePollMaybeReady int8 = 1
)

type rustFuturePollFunc func(C.uint64_t, C.UniffiRustFutureContinuationCallback, C.uint64_t)
type rustFutureCompleteFunc[T any] func(C.uint64_t, *C.RustCallStatus) T
type rustFutureFreeFunc func(C.uint64_t)

//export breez_sdk_spark_uniffiFutureContinuationCallback
func breez_sdk_spark_uniffiFutureContinuationCallback(data C.uint64_t, pollResult C.int8_t) {
	h := cgo.Handle(uintptr(data))
	waiter := h.Value().(chan int8)
	waiter <- int8(pollResult)
}

func uniffiRustCallAsync[E any, T any, F any](
	errConverter BufReader[*E],
	completeFunc rustFutureCompleteFunc[F],
	liftFunc func(F) T,
	rustFuture C.uint64_t,
	pollFunc rustFuturePollFunc,
	freeFunc rustFutureFreeFunc,
) (T, *E) {
	defer freeFunc(rustFuture)

	pollResult := int8(-1)
	waiter := make(chan int8, 1)

	chanHandle := cgo.NewHandle(waiter)
	defer chanHandle.Delete()

	for pollResult != uniffiRustFuturePollReady {
		pollFunc(
			rustFuture,
			(C.UniffiRustFutureContinuationCallback)(C.breez_sdk_spark_uniffiFutureContinuationCallback),
			C.uint64_t(chanHandle),
		)
		pollResult = <-waiter
	}

	var goValue T
	var ffiValue F
	var err *E

	ffiValue, err = rustCallWithError(errConverter, func(status *C.RustCallStatus) F {
		return completeFunc(rustFuture, status)
	})
	if err != nil {
		return goValue, err
	}
	return liftFunc(ffiValue), nil
}

//export breez_sdk_spark_uniffiFreeGorutine
func breez_sdk_spark_uniffiFreeGorutine(data C.uint64_t) {
	handle := cgo.Handle(uintptr(data))
	defer handle.Delete()

	guard := handle.Value().(chan struct{})
	guard <- struct{}{}
}

// Connects to the Spark network using the provided configuration and mnemonic.
//
// # Arguments
//
// * `request` - The connection request object
//
// # Returns
//
// Result containing either the initialized `BreezSdk` or an `SdkError`
func Connect(request ConnectRequest) (*BreezSdk, error) {
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_breez_sdk_spark_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *BreezSdk {
			return FfiConverterBreezSdkINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_func_connect(FfiConverterConnectRequestINSTANCE.Lower(request)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_pointer(handle)
		},
	)

	return res, err
}

func DefaultConfig(network Network) Config {
	return FfiConverterConfigINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_breez_sdk_spark_fn_func_default_config(FfiConverterNetworkINSTANCE.Lower(network), _uniffiStatus),
		}
	}))
}

func DefaultStorage(dataDir string) (Storage, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SdkError](FfiConverterSdkError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_breez_sdk_spark_fn_func_default_storage(FfiConverterStringINSTANCE.Lower(dataDir), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Storage
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStorageINSTANCE.Lift(_uniffiRV), nil
	}
}

func InitLogging(logDir *string, appLogger *Logger, logFilter *string) error {
	_, _uniffiErr := rustCallWithError[SdkError](FfiConverterSdkError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_breez_sdk_spark_fn_func_init_logging(FfiConverterOptionalStringINSTANCE.Lower(logDir), FfiConverterOptionalCallbackInterfaceLoggerINSTANCE.Lower(appLogger), FfiConverterOptionalStringINSTANCE.Lower(logFilter), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func Parse(input string) (breez_sdk_common.InputType, error) {
	res, err := uniffiRustCallAsync[SdkError](
		FfiConverterSdkErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_spark_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) breez_sdk_common.InputType {
			return breez_sdk_common.FfiConverterInputTypeINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_spark_fn_func_parse(FfiConverterStringINSTANCE.Lower(input)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_spark_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}
