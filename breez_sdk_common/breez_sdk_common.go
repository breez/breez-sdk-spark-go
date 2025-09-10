package breez_sdk_common

// #include <breez_sdk_common.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
		C.ffi_breez_sdk_common_rustbuffer_free(cb.inner, status)
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
		return C.ffi_breez_sdk_common_rustbuffer_from_bytes(foreign, status)
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

	FfiConverterRestClientINSTANCE.register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_breez_sdk_common_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("breez_sdk_common: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_common_checksum_method_restclient_get()
		})
		if checksum != 32450 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_common: uniffi_breez_sdk_common_checksum_method_restclient_get: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_breez_sdk_common_checksum_method_restclient_post()
		})
		if checksum != 14213 {
			// If this happens try cleaning and rebuilding your project
			panic("breez_sdk_common: uniffi_breez_sdk_common_checksum_method_restclient_post: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint16 struct{}

var FfiConverterUint16INSTANCE = FfiConverterUint16{}

func (FfiConverterUint16) Lower(value uint16) C.uint16_t {
	return C.uint16_t(value)
}

func (FfiConverterUint16) Write(writer io.Writer, value uint16) {
	writeUint16(writer, value)
}

func (FfiConverterUint16) Lift(value C.uint16_t) uint16 {
	return uint16(value)
}

func (FfiConverterUint16) Read(reader io.Reader) uint16 {
	return readUint16(reader)
}

type FfiDestroyerUint16 struct{}

func (FfiDestroyerUint16) Destroy(_ uint16) {}

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

type FfiConverterFloat64 struct{}

var FfiConverterFloat64INSTANCE = FfiConverterFloat64{}

func (FfiConverterFloat64) Lower(value float64) C.double {
	return C.double(value)
}

func (FfiConverterFloat64) Write(writer io.Writer, value float64) {
	writeFloat64(writer, value)
}

func (FfiConverterFloat64) Lift(value C.double) float64 {
	return float64(value)
}

func (FfiConverterFloat64) Read(reader io.Reader) float64 {
	return readFloat64(reader)
}

type FfiDestroyerFloat64 struct{}

func (FfiDestroyerFloat64) Destroy(_ float64) {}

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

type RestClient interface {
	// Makes a GET request and logs on DEBUG.
	// ### Arguments
	// - `url`: the URL on which GET will be called
	// - `headers`: optional headers that will be set on the request
	Get(url string, headers *map[string]string) (RestResponse, error)
	// Makes a POST request, and logs on DEBUG.
	// ### Arguments
	// - `url`: the URL on which POST will be called
	// - `headers`: the optional POST headers
	// - `body`: the optional POST body
	Post(url string, headers *map[string]string, body *string) (RestResponse, error)
}
type RestClientImpl struct {
	ffiObject FfiObject
}

// Makes a GET request and logs on DEBUG.
// ### Arguments
// - `url`: the URL on which GET will be called
// - `headers`: optional headers that will be set on the request
func (_self *RestClientImpl) Get(url string, headers *map[string]string) (RestResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("RestClient")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[ServiceConnectivityError](
		FfiConverterServiceConnectivityErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_common_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) RestResponse {
			return FfiConverterRestResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_common_fn_method_restclient_get(
			_pointer, FfiConverterStringINSTANCE.Lower(url), FfiConverterOptionalMapStringStringINSTANCE.Lower(headers)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_common_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_common_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}

// Makes a POST request, and logs on DEBUG.
// ### Arguments
// - `url`: the URL on which POST will be called
// - `headers`: the optional POST headers
// - `body`: the optional POST body
func (_self *RestClientImpl) Post(url string, headers *map[string]string, body *string) (RestResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("RestClient")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[ServiceConnectivityError](
		FfiConverterServiceConnectivityErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_breez_sdk_common_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) RestResponse {
			return FfiConverterRestResponseINSTANCE.Lift(ffi)
		},
		C.uniffi_breez_sdk_common_fn_method_restclient_post(
			_pointer, FfiConverterStringINSTANCE.Lower(url), FfiConverterOptionalMapStringStringINSTANCE.Lower(headers), FfiConverterOptionalStringINSTANCE.Lower(body)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_breez_sdk_common_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_breez_sdk_common_rust_future_free_rust_buffer(handle)
		},
	)

	return res, err
}
func (object *RestClientImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterRestClient struct {
	handleMap *concurrentHandleMap[RestClient]
}

var FfiConverterRestClientINSTANCE = FfiConverterRestClient{
	handleMap: newConcurrentHandleMap[RestClient](),
}

func (c FfiConverterRestClient) Lift(pointer unsafe.Pointer) RestClient {
	result := &RestClientImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_breez_sdk_common_fn_clone_restclient(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_breez_sdk_common_fn_free_restclient(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*RestClientImpl).Destroy)
	return result
}

func (c FfiConverterRestClient) Read(reader io.Reader) RestClient {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterRestClient) Lower(value RestClient) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterRestClient) Write(writer io.Writer, value RestClient) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerRestClient struct{}

func (_ FfiDestroyerRestClient) Destroy(value RestClient) {
	if val, ok := value.(*RestClientImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *RestClientImpl")
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

//export breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientMethod0
func breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientMethod0(uniffiHandle C.uint64_t, url C.RustBuffer, headers C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterRestClientINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_common_uniffiFreeGorutine),
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
			uniffiObj.Get(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: url,
				}),
				FfiConverterOptionalMapStringStringINSTANCE.Lift(GoRustBuffer{
					inner: headers,
				}),
			)

		if err != nil {
			var actualError *ServiceConnectivityError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterServiceConnectivityErrorINSTANCE.Lower(actualError),
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

		*uniffiOutReturn = FfiConverterRestResponseINSTANCE.Lower(res)
	}()
}

//export breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientMethod1
func breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientMethod1(uniffiHandle C.uint64_t, url C.RustBuffer, headers C.RustBuffer, body C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteRustBuffer, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterRestClientINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructRustBuffer, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.breez_sdk_common_uniffiFreeGorutine),
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
			uniffiObj.Post(
				FfiConverterStringINSTANCE.Lift(GoRustBuffer{
					inner: url,
				}),
				FfiConverterOptionalMapStringStringINSTANCE.Lift(GoRustBuffer{
					inner: headers,
				}),
				FfiConverterOptionalStringINSTANCE.Lift(GoRustBuffer{
					inner: body,
				}),
			)

		if err != nil {
			var actualError *ServiceConnectivityError
			if errors.As(err, &actualError) {
				if actualError != nil {
					*callStatus = C.RustCallStatus{
						code:     C.int8_t(uniffiCallbackResultError),
						errorBuf: FfiConverterServiceConnectivityErrorINSTANCE.Lower(actualError),
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

		*uniffiOutReturn = FfiConverterRestResponseINSTANCE.Lower(res)
	}()
}

var UniffiVTableCallbackInterfaceRestClientINSTANCE = C.UniffiVTableCallbackInterfaceRestClient{
	get:  (C.UniffiCallbackInterfaceRestClientMethod0)(C.breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientMethod0),
	post: (C.UniffiCallbackInterfaceRestClientMethod1)(C.breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientMethod1),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientFree),
}

//export breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientFree
func breez_sdk_common_cgo_dispatchCallbackInterfaceRestClientFree(handle C.uint64_t) {
	FfiConverterRestClientINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterRestClient) register() {
	C.uniffi_breez_sdk_common_fn_init_callback_vtable_restclient(&UniffiVTableCallbackInterfaceRestClientINSTANCE)
}

// Payload of the AES success action, as received from the LNURL endpoint
//
// See [`AesSuccessActionDataDecrypted`] for a similar wrapper containing the decrypted payload
type AesSuccessActionData struct {
	// Contents description, up to 144 characters
	Description string
	// Base64, AES-encrypted data where encryption key is payment preimage, up to 4kb of characters
	Ciphertext string
	// Base64, initialization vector, exactly 24 characters
	Iv string
}

func (r *AesSuccessActionData) Destroy() {
	FfiDestroyerString{}.Destroy(r.Description)
	FfiDestroyerString{}.Destroy(r.Ciphertext)
	FfiDestroyerString{}.Destroy(r.Iv)
}

type FfiConverterAesSuccessActionData struct{}

var FfiConverterAesSuccessActionDataINSTANCE = FfiConverterAesSuccessActionData{}

func (c FfiConverterAesSuccessActionData) Lift(rb RustBufferI) AesSuccessActionData {
	return LiftFromRustBuffer[AesSuccessActionData](c, rb)
}

func (c FfiConverterAesSuccessActionData) Read(reader io.Reader) AesSuccessActionData {
	return AesSuccessActionData{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterAesSuccessActionData) Lower(value AesSuccessActionData) C.RustBuffer {
	return LowerIntoRustBuffer[AesSuccessActionData](c, value)
}

func (c FfiConverterAesSuccessActionData) Write(writer io.Writer, value AesSuccessActionData) {
	FfiConverterStringINSTANCE.Write(writer, value.Description)
	FfiConverterStringINSTANCE.Write(writer, value.Ciphertext)
	FfiConverterStringINSTANCE.Write(writer, value.Iv)
}

type FfiDestroyerAesSuccessActionData struct{}

func (_ FfiDestroyerAesSuccessActionData) Destroy(value AesSuccessActionData) {
	value.Destroy()
}

// Wrapper for the decrypted [`AesSuccessActionData`] payload
type AesSuccessActionDataDecrypted struct {
	// Contents description, up to 144 characters
	Description string
	// Decrypted content
	Plaintext string
}

func (r *AesSuccessActionDataDecrypted) Destroy() {
	FfiDestroyerString{}.Destroy(r.Description)
	FfiDestroyerString{}.Destroy(r.Plaintext)
}

type FfiConverterAesSuccessActionDataDecrypted struct{}

var FfiConverterAesSuccessActionDataDecryptedINSTANCE = FfiConverterAesSuccessActionDataDecrypted{}

func (c FfiConverterAesSuccessActionDataDecrypted) Lift(rb RustBufferI) AesSuccessActionDataDecrypted {
	return LiftFromRustBuffer[AesSuccessActionDataDecrypted](c, rb)
}

func (c FfiConverterAesSuccessActionDataDecrypted) Read(reader io.Reader) AesSuccessActionDataDecrypted {
	return AesSuccessActionDataDecrypted{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterAesSuccessActionDataDecrypted) Lower(value AesSuccessActionDataDecrypted) C.RustBuffer {
	return LowerIntoRustBuffer[AesSuccessActionDataDecrypted](c, value)
}

func (c FfiConverterAesSuccessActionDataDecrypted) Write(writer io.Writer, value AesSuccessActionDataDecrypted) {
	FfiConverterStringINSTANCE.Write(writer, value.Description)
	FfiConverterStringINSTANCE.Write(writer, value.Plaintext)
}

type FfiDestroyerAesSuccessActionDataDecrypted struct{}

func (_ FfiDestroyerAesSuccessActionDataDecrypted) Destroy(value AesSuccessActionDataDecrypted) {
	value.Destroy()
}

type Bip21Details struct {
	AmountSat      *uint64
	AssetId        *string
	Uri            string
	Extras         []Bip21Extra
	Label          *string
	Message        *string
	PaymentMethods []InputType
}

func (r *Bip21Details) Destroy() {
	FfiDestroyerOptionalUint64{}.Destroy(r.AmountSat)
	FfiDestroyerOptionalString{}.Destroy(r.AssetId)
	FfiDestroyerString{}.Destroy(r.Uri)
	FfiDestroyerSequenceBip21Extra{}.Destroy(r.Extras)
	FfiDestroyerOptionalString{}.Destroy(r.Label)
	FfiDestroyerOptionalString{}.Destroy(r.Message)
	FfiDestroyerSequenceInputType{}.Destroy(r.PaymentMethods)
}

type FfiConverterBip21Details struct{}

var FfiConverterBip21DetailsINSTANCE = FfiConverterBip21Details{}

func (c FfiConverterBip21Details) Lift(rb RustBufferI) Bip21Details {
	return LiftFromRustBuffer[Bip21Details](c, rb)
}

func (c FfiConverterBip21Details) Read(reader io.Reader) Bip21Details {
	return Bip21Details{
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterSequenceBip21ExtraINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterSequenceInputTypeINSTANCE.Read(reader),
	}
}

func (c FfiConverterBip21Details) Lower(value Bip21Details) C.RustBuffer {
	return LowerIntoRustBuffer[Bip21Details](c, value)
}

func (c FfiConverterBip21Details) Write(writer io.Writer, value Bip21Details) {
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.AmountSat)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.AssetId)
	FfiConverterStringINSTANCE.Write(writer, value.Uri)
	FfiConverterSequenceBip21ExtraINSTANCE.Write(writer, value.Extras)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Label)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Message)
	FfiConverterSequenceInputTypeINSTANCE.Write(writer, value.PaymentMethods)
}

type FfiDestroyerBip21Details struct{}

func (_ FfiDestroyerBip21Details) Destroy(value Bip21Details) {
	value.Destroy()
}

type Bip21Extra struct {
	Key   string
	Value string
}

func (r *Bip21Extra) Destroy() {
	FfiDestroyerString{}.Destroy(r.Key)
	FfiDestroyerString{}.Destroy(r.Value)
}

type FfiConverterBip21Extra struct{}

var FfiConverterBip21ExtraINSTANCE = FfiConverterBip21Extra{}

func (c FfiConverterBip21Extra) Lift(rb RustBufferI) Bip21Extra {
	return LiftFromRustBuffer[Bip21Extra](c, rb)
}

func (c FfiConverterBip21Extra) Read(reader io.Reader) Bip21Extra {
	return Bip21Extra{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterBip21Extra) Lower(value Bip21Extra) C.RustBuffer {
	return LowerIntoRustBuffer[Bip21Extra](c, value)
}

func (c FfiConverterBip21Extra) Write(writer io.Writer, value Bip21Extra) {
	FfiConverterStringINSTANCE.Write(writer, value.Key)
	FfiConverterStringINSTANCE.Write(writer, value.Value)
}

type FfiDestroyerBip21Extra struct{}

func (_ FfiDestroyerBip21Extra) Destroy(value Bip21Extra) {
	value.Destroy()
}

type BitcoinAddressDetails struct {
	Address string
	Network BitcoinNetwork
	Source  PaymentRequestSource
}

func (r *BitcoinAddressDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Address)
	FfiDestroyerBitcoinNetwork{}.Destroy(r.Network)
	FfiDestroyerPaymentRequestSource{}.Destroy(r.Source)
}

type FfiConverterBitcoinAddressDetails struct{}

var FfiConverterBitcoinAddressDetailsINSTANCE = FfiConverterBitcoinAddressDetails{}

func (c FfiConverterBitcoinAddressDetails) Lift(rb RustBufferI) BitcoinAddressDetails {
	return LiftFromRustBuffer[BitcoinAddressDetails](c, rb)
}

func (c FfiConverterBitcoinAddressDetails) Read(reader io.Reader) BitcoinAddressDetails {
	return BitcoinAddressDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterBitcoinNetworkINSTANCE.Read(reader),
		FfiConverterPaymentRequestSourceINSTANCE.Read(reader),
	}
}

func (c FfiConverterBitcoinAddressDetails) Lower(value BitcoinAddressDetails) C.RustBuffer {
	return LowerIntoRustBuffer[BitcoinAddressDetails](c, value)
}

func (c FfiConverterBitcoinAddressDetails) Write(writer io.Writer, value BitcoinAddressDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Address)
	FfiConverterBitcoinNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterPaymentRequestSourceINSTANCE.Write(writer, value.Source)
}

type FfiDestroyerBitcoinAddressDetails struct{}

func (_ FfiDestroyerBitcoinAddressDetails) Destroy(value BitcoinAddressDetails) {
	value.Destroy()
}

type Bolt11Invoice struct {
	Bolt11 string
	Source PaymentRequestSource
}

func (r *Bolt11Invoice) Destroy() {
	FfiDestroyerString{}.Destroy(r.Bolt11)
	FfiDestroyerPaymentRequestSource{}.Destroy(r.Source)
}

type FfiConverterBolt11Invoice struct{}

var FfiConverterBolt11InvoiceINSTANCE = FfiConverterBolt11Invoice{}

func (c FfiConverterBolt11Invoice) Lift(rb RustBufferI) Bolt11Invoice {
	return LiftFromRustBuffer[Bolt11Invoice](c, rb)
}

func (c FfiConverterBolt11Invoice) Read(reader io.Reader) Bolt11Invoice {
	return Bolt11Invoice{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterPaymentRequestSourceINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt11Invoice) Lower(value Bolt11Invoice) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt11Invoice](c, value)
}

func (c FfiConverterBolt11Invoice) Write(writer io.Writer, value Bolt11Invoice) {
	FfiConverterStringINSTANCE.Write(writer, value.Bolt11)
	FfiConverterPaymentRequestSourceINSTANCE.Write(writer, value.Source)
}

type FfiDestroyerBolt11Invoice struct{}

func (_ FfiDestroyerBolt11Invoice) Destroy(value Bolt11Invoice) {
	value.Destroy()
}

type Bolt11InvoiceDetails struct {
	AmountMsat              *uint64
	Description             *string
	DescriptionHash         *string
	Expiry                  uint64
	Invoice                 Bolt11Invoice
	MinFinalCltvExpiryDelta uint64
	Network                 BitcoinNetwork
	PayeePubkey             string
	PaymentHash             string
	PaymentSecret           string
	RoutingHints            []Bolt11RouteHint
	Timestamp               uint64
}

func (r *Bolt11InvoiceDetails) Destroy() {
	FfiDestroyerOptionalUint64{}.Destroy(r.AmountMsat)
	FfiDestroyerOptionalString{}.Destroy(r.Description)
	FfiDestroyerOptionalString{}.Destroy(r.DescriptionHash)
	FfiDestroyerUint64{}.Destroy(r.Expiry)
	FfiDestroyerBolt11Invoice{}.Destroy(r.Invoice)
	FfiDestroyerUint64{}.Destroy(r.MinFinalCltvExpiryDelta)
	FfiDestroyerBitcoinNetwork{}.Destroy(r.Network)
	FfiDestroyerString{}.Destroy(r.PayeePubkey)
	FfiDestroyerString{}.Destroy(r.PaymentHash)
	FfiDestroyerString{}.Destroy(r.PaymentSecret)
	FfiDestroyerSequenceBolt11RouteHint{}.Destroy(r.RoutingHints)
	FfiDestroyerUint64{}.Destroy(r.Timestamp)
}

type FfiConverterBolt11InvoiceDetails struct{}

var FfiConverterBolt11InvoiceDetailsINSTANCE = FfiConverterBolt11InvoiceDetails{}

func (c FfiConverterBolt11InvoiceDetails) Lift(rb RustBufferI) Bolt11InvoiceDetails {
	return LiftFromRustBuffer[Bolt11InvoiceDetails](c, rb)
}

func (c FfiConverterBolt11InvoiceDetails) Read(reader io.Reader) Bolt11InvoiceDetails {
	return Bolt11InvoiceDetails{
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterBolt11InvoiceINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterBitcoinNetworkINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterSequenceBolt11RouteHintINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt11InvoiceDetails) Lower(value Bolt11InvoiceDetails) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt11InvoiceDetails](c, value)
}

func (c FfiConverterBolt11InvoiceDetails) Write(writer io.Writer, value Bolt11InvoiceDetails) {
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.AmountMsat)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Description)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.DescriptionHash)
	FfiConverterUint64INSTANCE.Write(writer, value.Expiry)
	FfiConverterBolt11InvoiceINSTANCE.Write(writer, value.Invoice)
	FfiConverterUint64INSTANCE.Write(writer, value.MinFinalCltvExpiryDelta)
	FfiConverterBitcoinNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterStringINSTANCE.Write(writer, value.PayeePubkey)
	FfiConverterStringINSTANCE.Write(writer, value.PaymentHash)
	FfiConverterStringINSTANCE.Write(writer, value.PaymentSecret)
	FfiConverterSequenceBolt11RouteHintINSTANCE.Write(writer, value.RoutingHints)
	FfiConverterUint64INSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerBolt11InvoiceDetails struct{}

func (_ FfiDestroyerBolt11InvoiceDetails) Destroy(value Bolt11InvoiceDetails) {
	value.Destroy()
}

type Bolt11RouteHint struct {
	Hops []Bolt11RouteHintHop
}

func (r *Bolt11RouteHint) Destroy() {
	FfiDestroyerSequenceBolt11RouteHintHop{}.Destroy(r.Hops)
}

type FfiConverterBolt11RouteHint struct{}

var FfiConverterBolt11RouteHintINSTANCE = FfiConverterBolt11RouteHint{}

func (c FfiConverterBolt11RouteHint) Lift(rb RustBufferI) Bolt11RouteHint {
	return LiftFromRustBuffer[Bolt11RouteHint](c, rb)
}

func (c FfiConverterBolt11RouteHint) Read(reader io.Reader) Bolt11RouteHint {
	return Bolt11RouteHint{
		FfiConverterSequenceBolt11RouteHintHopINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt11RouteHint) Lower(value Bolt11RouteHint) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt11RouteHint](c, value)
}

func (c FfiConverterBolt11RouteHint) Write(writer io.Writer, value Bolt11RouteHint) {
	FfiConverterSequenceBolt11RouteHintHopINSTANCE.Write(writer, value.Hops)
}

type FfiDestroyerBolt11RouteHint struct{}

func (_ FfiDestroyerBolt11RouteHint) Destroy(value Bolt11RouteHint) {
	value.Destroy()
}

type Bolt11RouteHintHop struct {
	// The `node_id` of the non-target end of the route
	SrcNodeId string
	// The `short_channel_id` of this channel
	ShortChannelId string
	// The fees which must be paid to use this channel
	FeesBaseMsat               uint32
	FeesProportionalMillionths uint32
	// The difference in CLTV values between this node and the next node.
	CltvExpiryDelta uint16
	// The minimum value, in msat, which must be relayed to the next hop.
	HtlcMinimumMsat *uint64
	// The maximum value in msat available for routing with a single HTLC.
	HtlcMaximumMsat *uint64
}

func (r *Bolt11RouteHintHop) Destroy() {
	FfiDestroyerString{}.Destroy(r.SrcNodeId)
	FfiDestroyerString{}.Destroy(r.ShortChannelId)
	FfiDestroyerUint32{}.Destroy(r.FeesBaseMsat)
	FfiDestroyerUint32{}.Destroy(r.FeesProportionalMillionths)
	FfiDestroyerUint16{}.Destroy(r.CltvExpiryDelta)
	FfiDestroyerOptionalUint64{}.Destroy(r.HtlcMinimumMsat)
	FfiDestroyerOptionalUint64{}.Destroy(r.HtlcMaximumMsat)
}

type FfiConverterBolt11RouteHintHop struct{}

var FfiConverterBolt11RouteHintHopINSTANCE = FfiConverterBolt11RouteHintHop{}

func (c FfiConverterBolt11RouteHintHop) Lift(rb RustBufferI) Bolt11RouteHintHop {
	return LiftFromRustBuffer[Bolt11RouteHintHop](c, rb)
}

func (c FfiConverterBolt11RouteHintHop) Read(reader io.Reader) Bolt11RouteHintHop {
	return Bolt11RouteHintHop{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint16INSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt11RouteHintHop) Lower(value Bolt11RouteHintHop) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt11RouteHintHop](c, value)
}

func (c FfiConverterBolt11RouteHintHop) Write(writer io.Writer, value Bolt11RouteHintHop) {
	FfiConverterStringINSTANCE.Write(writer, value.SrcNodeId)
	FfiConverterStringINSTANCE.Write(writer, value.ShortChannelId)
	FfiConverterUint32INSTANCE.Write(writer, value.FeesBaseMsat)
	FfiConverterUint32INSTANCE.Write(writer, value.FeesProportionalMillionths)
	FfiConverterUint16INSTANCE.Write(writer, value.CltvExpiryDelta)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.HtlcMinimumMsat)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.HtlcMaximumMsat)
}

type FfiDestroyerBolt11RouteHintHop struct{}

func (_ FfiDestroyerBolt11RouteHintHop) Destroy(value Bolt11RouteHintHop) {
	value.Destroy()
}

type Bolt12Invoice struct {
	Invoice string
	Source  PaymentRequestSource
}

func (r *Bolt12Invoice) Destroy() {
	FfiDestroyerString{}.Destroy(r.Invoice)
	FfiDestroyerPaymentRequestSource{}.Destroy(r.Source)
}

type FfiConverterBolt12Invoice struct{}

var FfiConverterBolt12InvoiceINSTANCE = FfiConverterBolt12Invoice{}

func (c FfiConverterBolt12Invoice) Lift(rb RustBufferI) Bolt12Invoice {
	return LiftFromRustBuffer[Bolt12Invoice](c, rb)
}

func (c FfiConverterBolt12Invoice) Read(reader io.Reader) Bolt12Invoice {
	return Bolt12Invoice{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterPaymentRequestSourceINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt12Invoice) Lower(value Bolt12Invoice) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt12Invoice](c, value)
}

func (c FfiConverterBolt12Invoice) Write(writer io.Writer, value Bolt12Invoice) {
	FfiConverterStringINSTANCE.Write(writer, value.Invoice)
	FfiConverterPaymentRequestSourceINSTANCE.Write(writer, value.Source)
}

type FfiDestroyerBolt12Invoice struct{}

func (_ FfiDestroyerBolt12Invoice) Destroy(value Bolt12Invoice) {
	value.Destroy()
}

type Bolt12InvoiceDetails struct {
	AmountMsat uint64
	Invoice    Bolt12Invoice
}

func (r *Bolt12InvoiceDetails) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.AmountMsat)
	FfiDestroyerBolt12Invoice{}.Destroy(r.Invoice)
}

type FfiConverterBolt12InvoiceDetails struct{}

var FfiConverterBolt12InvoiceDetailsINSTANCE = FfiConverterBolt12InvoiceDetails{}

func (c FfiConverterBolt12InvoiceDetails) Lift(rb RustBufferI) Bolt12InvoiceDetails {
	return LiftFromRustBuffer[Bolt12InvoiceDetails](c, rb)
}

func (c FfiConverterBolt12InvoiceDetails) Read(reader io.Reader) Bolt12InvoiceDetails {
	return Bolt12InvoiceDetails{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterBolt12InvoiceINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt12InvoiceDetails) Lower(value Bolt12InvoiceDetails) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt12InvoiceDetails](c, value)
}

func (c FfiConverterBolt12InvoiceDetails) Write(writer io.Writer, value Bolt12InvoiceDetails) {
	FfiConverterUint64INSTANCE.Write(writer, value.AmountMsat)
	FfiConverterBolt12InvoiceINSTANCE.Write(writer, value.Invoice)
}

type FfiDestroyerBolt12InvoiceDetails struct{}

func (_ FfiDestroyerBolt12InvoiceDetails) Destroy(value Bolt12InvoiceDetails) {
	value.Destroy()
}

type Bolt12InvoiceRequestDetails struct {
}

func (r *Bolt12InvoiceRequestDetails) Destroy() {
}

type FfiConverterBolt12InvoiceRequestDetails struct{}

var FfiConverterBolt12InvoiceRequestDetailsINSTANCE = FfiConverterBolt12InvoiceRequestDetails{}

func (c FfiConverterBolt12InvoiceRequestDetails) Lift(rb RustBufferI) Bolt12InvoiceRequestDetails {
	return LiftFromRustBuffer[Bolt12InvoiceRequestDetails](c, rb)
}

func (c FfiConverterBolt12InvoiceRequestDetails) Read(reader io.Reader) Bolt12InvoiceRequestDetails {
	return Bolt12InvoiceRequestDetails{}
}

func (c FfiConverterBolt12InvoiceRequestDetails) Lower(value Bolt12InvoiceRequestDetails) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt12InvoiceRequestDetails](c, value)
}

func (c FfiConverterBolt12InvoiceRequestDetails) Write(writer io.Writer, value Bolt12InvoiceRequestDetails) {
}

type FfiDestroyerBolt12InvoiceRequestDetails struct{}

func (_ FfiDestroyerBolt12InvoiceRequestDetails) Destroy(value Bolt12InvoiceRequestDetails) {
	value.Destroy()
}

type Bolt12Offer struct {
	Offer  string
	Source PaymentRequestSource
}

func (r *Bolt12Offer) Destroy() {
	FfiDestroyerString{}.Destroy(r.Offer)
	FfiDestroyerPaymentRequestSource{}.Destroy(r.Source)
}

type FfiConverterBolt12Offer struct{}

var FfiConverterBolt12OfferINSTANCE = FfiConverterBolt12Offer{}

func (c FfiConverterBolt12Offer) Lift(rb RustBufferI) Bolt12Offer {
	return LiftFromRustBuffer[Bolt12Offer](c, rb)
}

func (c FfiConverterBolt12Offer) Read(reader io.Reader) Bolt12Offer {
	return Bolt12Offer{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterPaymentRequestSourceINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt12Offer) Lower(value Bolt12Offer) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt12Offer](c, value)
}

func (c FfiConverterBolt12Offer) Write(writer io.Writer, value Bolt12Offer) {
	FfiConverterStringINSTANCE.Write(writer, value.Offer)
	FfiConverterPaymentRequestSourceINSTANCE.Write(writer, value.Source)
}

type FfiDestroyerBolt12Offer struct{}

func (_ FfiDestroyerBolt12Offer) Destroy(value Bolt12Offer) {
	value.Destroy()
}

type Bolt12OfferBlindedPath struct {
	BlindedHops []string
}

func (r *Bolt12OfferBlindedPath) Destroy() {
	FfiDestroyerSequenceString{}.Destroy(r.BlindedHops)
}

type FfiConverterBolt12OfferBlindedPath struct{}

var FfiConverterBolt12OfferBlindedPathINSTANCE = FfiConverterBolt12OfferBlindedPath{}

func (c FfiConverterBolt12OfferBlindedPath) Lift(rb RustBufferI) Bolt12OfferBlindedPath {
	return LiftFromRustBuffer[Bolt12OfferBlindedPath](c, rb)
}

func (c FfiConverterBolt12OfferBlindedPath) Read(reader io.Reader) Bolt12OfferBlindedPath {
	return Bolt12OfferBlindedPath{
		FfiConverterSequenceStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt12OfferBlindedPath) Lower(value Bolt12OfferBlindedPath) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt12OfferBlindedPath](c, value)
}

func (c FfiConverterBolt12OfferBlindedPath) Write(writer io.Writer, value Bolt12OfferBlindedPath) {
	FfiConverterSequenceStringINSTANCE.Write(writer, value.BlindedHops)
}

type FfiDestroyerBolt12OfferBlindedPath struct{}

func (_ FfiDestroyerBolt12OfferBlindedPath) Destroy(value Bolt12OfferBlindedPath) {
	value.Destroy()
}

type Bolt12OfferDetails struct {
	AbsoluteExpiry *uint64
	Chains         []string
	Description    *string
	Issuer         *string
	MinAmount      *Amount
	Offer          Bolt12Offer
	Paths          []Bolt12OfferBlindedPath
	SigningPubkey  *string
}

func (r *Bolt12OfferDetails) Destroy() {
	FfiDestroyerOptionalUint64{}.Destroy(r.AbsoluteExpiry)
	FfiDestroyerSequenceString{}.Destroy(r.Chains)
	FfiDestroyerOptionalString{}.Destroy(r.Description)
	FfiDestroyerOptionalString{}.Destroy(r.Issuer)
	FfiDestroyerOptionalAmount{}.Destroy(r.MinAmount)
	FfiDestroyerBolt12Offer{}.Destroy(r.Offer)
	FfiDestroyerSequenceBolt12OfferBlindedPath{}.Destroy(r.Paths)
	FfiDestroyerOptionalString{}.Destroy(r.SigningPubkey)
}

type FfiConverterBolt12OfferDetails struct{}

var FfiConverterBolt12OfferDetailsINSTANCE = FfiConverterBolt12OfferDetails{}

func (c FfiConverterBolt12OfferDetails) Lift(rb RustBufferI) Bolt12OfferDetails {
	return LiftFromRustBuffer[Bolt12OfferDetails](c, rb)
}

func (c FfiConverterBolt12OfferDetails) Read(reader io.Reader) Bolt12OfferDetails {
	return Bolt12OfferDetails{
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterSequenceStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalAmountINSTANCE.Read(reader),
		FfiConverterBolt12OfferINSTANCE.Read(reader),
		FfiConverterSequenceBolt12OfferBlindedPathINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterBolt12OfferDetails) Lower(value Bolt12OfferDetails) C.RustBuffer {
	return LowerIntoRustBuffer[Bolt12OfferDetails](c, value)
}

func (c FfiConverterBolt12OfferDetails) Write(writer io.Writer, value Bolt12OfferDetails) {
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.AbsoluteExpiry)
	FfiConverterSequenceStringINSTANCE.Write(writer, value.Chains)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Description)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Issuer)
	FfiConverterOptionalAmountINSTANCE.Write(writer, value.MinAmount)
	FfiConverterBolt12OfferINSTANCE.Write(writer, value.Offer)
	FfiConverterSequenceBolt12OfferBlindedPathINSTANCE.Write(writer, value.Paths)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.SigningPubkey)
}

type FfiDestroyerBolt12OfferDetails struct{}

func (_ FfiDestroyerBolt12OfferDetails) Destroy(value Bolt12OfferDetails) {
	value.Destroy()
}

// Details about a supported currency in the fiat rate feed
type CurrencyInfo struct {
	Name            string
	FractionSize    uint32
	Spacing         *uint32
	Symbol          *Symbol
	UniqSymbol      *Symbol
	LocalizedName   []LocalizedName
	LocaleOverrides []LocaleOverrides
}

func (r *CurrencyInfo) Destroy() {
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerUint32{}.Destroy(r.FractionSize)
	FfiDestroyerOptionalUint32{}.Destroy(r.Spacing)
	FfiDestroyerOptionalSymbol{}.Destroy(r.Symbol)
	FfiDestroyerOptionalSymbol{}.Destroy(r.UniqSymbol)
	FfiDestroyerSequenceLocalizedName{}.Destroy(r.LocalizedName)
	FfiDestroyerSequenceLocaleOverrides{}.Destroy(r.LocaleOverrides)
}

type FfiConverterCurrencyInfo struct{}

var FfiConverterCurrencyInfoINSTANCE = FfiConverterCurrencyInfo{}

func (c FfiConverterCurrencyInfo) Lift(rb RustBufferI) CurrencyInfo {
	return LiftFromRustBuffer[CurrencyInfo](c, rb)
}

func (c FfiConverterCurrencyInfo) Read(reader io.Reader) CurrencyInfo {
	return CurrencyInfo{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
		FfiConverterOptionalSymbolINSTANCE.Read(reader),
		FfiConverterOptionalSymbolINSTANCE.Read(reader),
		FfiConverterSequenceLocalizedNameINSTANCE.Read(reader),
		FfiConverterSequenceLocaleOverridesINSTANCE.Read(reader),
	}
}

func (c FfiConverterCurrencyInfo) Lower(value CurrencyInfo) C.RustBuffer {
	return LowerIntoRustBuffer[CurrencyInfo](c, value)
}

func (c FfiConverterCurrencyInfo) Write(writer io.Writer, value CurrencyInfo) {
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterUint32INSTANCE.Write(writer, value.FractionSize)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Spacing)
	FfiConverterOptionalSymbolINSTANCE.Write(writer, value.Symbol)
	FfiConverterOptionalSymbolINSTANCE.Write(writer, value.UniqSymbol)
	FfiConverterSequenceLocalizedNameINSTANCE.Write(writer, value.LocalizedName)
	FfiConverterSequenceLocaleOverridesINSTANCE.Write(writer, value.LocaleOverrides)
}

type FfiDestroyerCurrencyInfo struct{}

func (_ FfiDestroyerCurrencyInfo) Destroy(value CurrencyInfo) {
	value.Destroy()
}

// Wrapper around the [`CurrencyInfo`] of a fiat currency
type FiatCurrency struct {
	Id   string
	Info CurrencyInfo
}

func (r *FiatCurrency) Destroy() {
	FfiDestroyerString{}.Destroy(r.Id)
	FfiDestroyerCurrencyInfo{}.Destroy(r.Info)
}

type FfiConverterFiatCurrency struct{}

var FfiConverterFiatCurrencyINSTANCE = FfiConverterFiatCurrency{}

func (c FfiConverterFiatCurrency) Lift(rb RustBufferI) FiatCurrency {
	return LiftFromRustBuffer[FiatCurrency](c, rb)
}

func (c FfiConverterFiatCurrency) Read(reader io.Reader) FiatCurrency {
	return FiatCurrency{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterCurrencyInfoINSTANCE.Read(reader),
	}
}

func (c FfiConverterFiatCurrency) Lower(value FiatCurrency) C.RustBuffer {
	return LowerIntoRustBuffer[FiatCurrency](c, value)
}

func (c FfiConverterFiatCurrency) Write(writer io.Writer, value FiatCurrency) {
	FfiConverterStringINSTANCE.Write(writer, value.Id)
	FfiConverterCurrencyInfoINSTANCE.Write(writer, value.Info)
}

type FfiDestroyerFiatCurrency struct{}

func (_ FfiDestroyerFiatCurrency) Destroy(value FiatCurrency) {
	value.Destroy()
}

type LightningAddressDetails struct {
	Address    string
	PayRequest LnurlPayRequestDetails
}

func (r *LightningAddressDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Address)
	FfiDestroyerLnurlPayRequestDetails{}.Destroy(r.PayRequest)
}

type FfiConverterLightningAddressDetails struct{}

var FfiConverterLightningAddressDetailsINSTANCE = FfiConverterLightningAddressDetails{}

func (c FfiConverterLightningAddressDetails) Lift(rb RustBufferI) LightningAddressDetails {
	return LiftFromRustBuffer[LightningAddressDetails](c, rb)
}

func (c FfiConverterLightningAddressDetails) Read(reader io.Reader) LightningAddressDetails {
	return LightningAddressDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterLnurlPayRequestDetailsINSTANCE.Read(reader),
	}
}

func (c FfiConverterLightningAddressDetails) Lower(value LightningAddressDetails) C.RustBuffer {
	return LowerIntoRustBuffer[LightningAddressDetails](c, value)
}

func (c FfiConverterLightningAddressDetails) Write(writer io.Writer, value LightningAddressDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Address)
	FfiConverterLnurlPayRequestDetailsINSTANCE.Write(writer, value.PayRequest)
}

type FfiDestroyerLightningAddressDetails struct{}

func (_ FfiDestroyerLightningAddressDetails) Destroy(value LightningAddressDetails) {
	value.Destroy()
}

// Wrapped in a [`LnurlAuth`], this is the result of [`parse`] when given a LNURL-auth endpoint.
//
// It represents the endpoint's parameters for the LNURL workflow.
//
// See <https://github.com/lnurl/luds/blob/luds/04.md>
type LnurlAuthRequestDetails struct {
	// Hex encoded 32 bytes of challenge
	K1 string
	// When available, one of: register, login, link, auth
	Action *string
	// Indicates the domain of the LNURL-auth service, to be shown to the user when asking for
	// auth confirmation, as per LUD-04 spec.
	Domain string
	// Indicates the URL of the LNURL-auth service, including the query arguments. This will be
	// extended with the signed challenge and the linking key, then called in the second step of the workflow.
	Url string
}

func (r *LnurlAuthRequestDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.K1)
	FfiDestroyerOptionalString{}.Destroy(r.Action)
	FfiDestroyerString{}.Destroy(r.Domain)
	FfiDestroyerString{}.Destroy(r.Url)
}

type FfiConverterLnurlAuthRequestDetails struct{}

var FfiConverterLnurlAuthRequestDetailsINSTANCE = FfiConverterLnurlAuthRequestDetails{}

func (c FfiConverterLnurlAuthRequestDetails) Lift(rb RustBufferI) LnurlAuthRequestDetails {
	return LiftFromRustBuffer[LnurlAuthRequestDetails](c, rb)
}

func (c FfiConverterLnurlAuthRequestDetails) Read(reader io.Reader) LnurlAuthRequestDetails {
	return LnurlAuthRequestDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlAuthRequestDetails) Lower(value LnurlAuthRequestDetails) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlAuthRequestDetails](c, value)
}

func (c FfiConverterLnurlAuthRequestDetails) Write(writer io.Writer, value LnurlAuthRequestDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.K1)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Action)
	FfiConverterStringINSTANCE.Write(writer, value.Domain)
	FfiConverterStringINSTANCE.Write(writer, value.Url)
}

type FfiDestroyerLnurlAuthRequestDetails struct{}

func (_ FfiDestroyerLnurlAuthRequestDetails) Destroy(value LnurlAuthRequestDetails) {
	value.Destroy()
}

// Wrapped in a [`LnUrlError`], this represents a LNURL-endpoint error.
type LnurlErrorDetails struct {
	Reason string
}

func (r *LnurlErrorDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Reason)
}

type FfiConverterLnurlErrorDetails struct{}

var FfiConverterLnurlErrorDetailsINSTANCE = FfiConverterLnurlErrorDetails{}

func (c FfiConverterLnurlErrorDetails) Lift(rb RustBufferI) LnurlErrorDetails {
	return LiftFromRustBuffer[LnurlErrorDetails](c, rb)
}

func (c FfiConverterLnurlErrorDetails) Read(reader io.Reader) LnurlErrorDetails {
	return LnurlErrorDetails{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlErrorDetails) Lower(value LnurlErrorDetails) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlErrorDetails](c, value)
}

func (c FfiConverterLnurlErrorDetails) Write(writer io.Writer, value LnurlErrorDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Reason)
}

type FfiDestroyerLnurlErrorDetails struct{}

func (_ FfiDestroyerLnurlErrorDetails) Destroy(value LnurlErrorDetails) {
	value.Destroy()
}

type LnurlPayRequestDetails struct {
	Callback string
	// The minimum amount, in millisats, that this LNURL-pay endpoint accepts
	MinSendable uint64
	// The maximum amount, in millisats, that this LNURL-pay endpoint accepts
	MaxSendable uint64
	// As per LUD-06, `metadata` is a raw string (e.g. a json representation of the inner map).
	// Use `metadata_vec()` to get the parsed items.
	MetadataStr string
	// The comment length accepted by this endpoint
	//
	// See <https://github.com/lnurl/luds/blob/luds/12.md>
	CommentAllowed uint16
	// Indicates the domain of the LNURL-pay service, to be shown to the user when asking for
	// payment input, as per LUD-06 spec.
	//
	// Note: this is not the domain of the callback, but the domain of the LNURL-pay endpoint.
	Domain string
	Url    string
	// Optional lightning address if that was used to resolve the lnurl.
	Address *string
	// Value indicating whether the recipient supports Nostr Zaps through NIP-57.
	//
	// See <https://github.com/nostr-protocol/nips/blob/master/57.md>
	AllowsNostr bool
	// Optional recipient's lnurl provider's Nostr pubkey for NIP-57. If it exists it should be a
	// valid BIP 340 public key in hex.
	//
	// See <https://github.com/nostr-protocol/nips/blob/master/57.md>
	// See <https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki>
	NostrPubkey *string
}

func (r *LnurlPayRequestDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Callback)
	FfiDestroyerUint64{}.Destroy(r.MinSendable)
	FfiDestroyerUint64{}.Destroy(r.MaxSendable)
	FfiDestroyerString{}.Destroy(r.MetadataStr)
	FfiDestroyerUint16{}.Destroy(r.CommentAllowed)
	FfiDestroyerString{}.Destroy(r.Domain)
	FfiDestroyerString{}.Destroy(r.Url)
	FfiDestroyerOptionalString{}.Destroy(r.Address)
	FfiDestroyerBool{}.Destroy(r.AllowsNostr)
	FfiDestroyerOptionalString{}.Destroy(r.NostrPubkey)
}

type FfiConverterLnurlPayRequestDetails struct{}

var FfiConverterLnurlPayRequestDetailsINSTANCE = FfiConverterLnurlPayRequestDetails{}

func (c FfiConverterLnurlPayRequestDetails) Lift(rb RustBufferI) LnurlPayRequestDetails {
	return LiftFromRustBuffer[LnurlPayRequestDetails](c, rb)
}

func (c FfiConverterLnurlPayRequestDetails) Read(reader io.Reader) LnurlPayRequestDetails {
	return LnurlPayRequestDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint16INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlPayRequestDetails) Lower(value LnurlPayRequestDetails) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlPayRequestDetails](c, value)
}

func (c FfiConverterLnurlPayRequestDetails) Write(writer io.Writer, value LnurlPayRequestDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Callback)
	FfiConverterUint64INSTANCE.Write(writer, value.MinSendable)
	FfiConverterUint64INSTANCE.Write(writer, value.MaxSendable)
	FfiConverterStringINSTANCE.Write(writer, value.MetadataStr)
	FfiConverterUint16INSTANCE.Write(writer, value.CommentAllowed)
	FfiConverterStringINSTANCE.Write(writer, value.Domain)
	FfiConverterStringINSTANCE.Write(writer, value.Url)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Address)
	FfiConverterBoolINSTANCE.Write(writer, value.AllowsNostr)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.NostrPubkey)
}

type FfiDestroyerLnurlPayRequestDetails struct{}

func (_ FfiDestroyerLnurlPayRequestDetails) Destroy(value LnurlPayRequestDetails) {
	value.Destroy()
}

type LnurlWithdrawRequestDetails struct {
	Callback           string
	K1                 string
	DefaultDescription string
	// The minimum amount, in millisats, that this LNURL-withdraw endpoint accepts
	MinWithdrawable uint64
	// The maximum amount, in millisats, that this LNURL-withdraw endpoint accepts
	MaxWithdrawable uint64
}

func (r *LnurlWithdrawRequestDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Callback)
	FfiDestroyerString{}.Destroy(r.K1)
	FfiDestroyerString{}.Destroy(r.DefaultDescription)
	FfiDestroyerUint64{}.Destroy(r.MinWithdrawable)
	FfiDestroyerUint64{}.Destroy(r.MaxWithdrawable)
}

type FfiConverterLnurlWithdrawRequestDetails struct{}

var FfiConverterLnurlWithdrawRequestDetailsINSTANCE = FfiConverterLnurlWithdrawRequestDetails{}

func (c FfiConverterLnurlWithdrawRequestDetails) Lift(rb RustBufferI) LnurlWithdrawRequestDetails {
	return LiftFromRustBuffer[LnurlWithdrawRequestDetails](c, rb)
}

func (c FfiConverterLnurlWithdrawRequestDetails) Read(reader io.Reader) LnurlWithdrawRequestDetails {
	return LnurlWithdrawRequestDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterLnurlWithdrawRequestDetails) Lower(value LnurlWithdrawRequestDetails) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlWithdrawRequestDetails](c, value)
}

func (c FfiConverterLnurlWithdrawRequestDetails) Write(writer io.Writer, value LnurlWithdrawRequestDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Callback)
	FfiConverterStringINSTANCE.Write(writer, value.K1)
	FfiConverterStringINSTANCE.Write(writer, value.DefaultDescription)
	FfiConverterUint64INSTANCE.Write(writer, value.MinWithdrawable)
	FfiConverterUint64INSTANCE.Write(writer, value.MaxWithdrawable)
}

type FfiDestroyerLnurlWithdrawRequestDetails struct{}

func (_ FfiDestroyerLnurlWithdrawRequestDetails) Destroy(value LnurlWithdrawRequestDetails) {
	value.Destroy()
}

// Locale-specific settings for the representation of a currency
type LocaleOverrides struct {
	Locale  string
	Spacing *uint32
	Symbol  Symbol
}

func (r *LocaleOverrides) Destroy() {
	FfiDestroyerString{}.Destroy(r.Locale)
	FfiDestroyerOptionalUint32{}.Destroy(r.Spacing)
	FfiDestroyerSymbol{}.Destroy(r.Symbol)
}

type FfiConverterLocaleOverrides struct{}

var FfiConverterLocaleOverridesINSTANCE = FfiConverterLocaleOverrides{}

func (c FfiConverterLocaleOverrides) Lift(rb RustBufferI) LocaleOverrides {
	return LiftFromRustBuffer[LocaleOverrides](c, rb)
}

func (c FfiConverterLocaleOverrides) Read(reader io.Reader) LocaleOverrides {
	return LocaleOverrides{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
		FfiConverterSymbolINSTANCE.Read(reader),
	}
}

func (c FfiConverterLocaleOverrides) Lower(value LocaleOverrides) C.RustBuffer {
	return LowerIntoRustBuffer[LocaleOverrides](c, value)
}

func (c FfiConverterLocaleOverrides) Write(writer io.Writer, value LocaleOverrides) {
	FfiConverterStringINSTANCE.Write(writer, value.Locale)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Spacing)
	FfiConverterSymbolINSTANCE.Write(writer, value.Symbol)
}

type FfiDestroyerLocaleOverrides struct{}

func (_ FfiDestroyerLocaleOverrides) Destroy(value LocaleOverrides) {
	value.Destroy()
}

// Localized name of a currency
type LocalizedName struct {
	Locale string
	Name   string
}

func (r *LocalizedName) Destroy() {
	FfiDestroyerString{}.Destroy(r.Locale)
	FfiDestroyerString{}.Destroy(r.Name)
}

type FfiConverterLocalizedName struct{}

var FfiConverterLocalizedNameINSTANCE = FfiConverterLocalizedName{}

func (c FfiConverterLocalizedName) Lift(rb RustBufferI) LocalizedName {
	return LiftFromRustBuffer[LocalizedName](c, rb)
}

func (c FfiConverterLocalizedName) Read(reader io.Reader) LocalizedName {
	return LocalizedName{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterLocalizedName) Lower(value LocalizedName) C.RustBuffer {
	return LowerIntoRustBuffer[LocalizedName](c, value)
}

func (c FfiConverterLocalizedName) Write(writer io.Writer, value LocalizedName) {
	FfiConverterStringINSTANCE.Write(writer, value.Locale)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
}

type FfiDestroyerLocalizedName struct{}

func (_ FfiDestroyerLocalizedName) Destroy(value LocalizedName) {
	value.Destroy()
}

type MessageSuccessActionData struct {
	Message string
}

func (r *MessageSuccessActionData) Destroy() {
	FfiDestroyerString{}.Destroy(r.Message)
}

type FfiConverterMessageSuccessActionData struct{}

var FfiConverterMessageSuccessActionDataINSTANCE = FfiConverterMessageSuccessActionData{}

func (c FfiConverterMessageSuccessActionData) Lift(rb RustBufferI) MessageSuccessActionData {
	return LiftFromRustBuffer[MessageSuccessActionData](c, rb)
}

func (c FfiConverterMessageSuccessActionData) Read(reader io.Reader) MessageSuccessActionData {
	return MessageSuccessActionData{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterMessageSuccessActionData) Lower(value MessageSuccessActionData) C.RustBuffer {
	return LowerIntoRustBuffer[MessageSuccessActionData](c, value)
}

func (c FfiConverterMessageSuccessActionData) Write(writer io.Writer, value MessageSuccessActionData) {
	FfiConverterStringINSTANCE.Write(writer, value.Message)
}

type FfiDestroyerMessageSuccessActionData struct{}

func (_ FfiDestroyerMessageSuccessActionData) Destroy(value MessageSuccessActionData) {
	value.Destroy()
}

type PaymentRequestSource struct {
	Bip21Uri      *string
	Bip353Address *string
}

func (r *PaymentRequestSource) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.Bip21Uri)
	FfiDestroyerOptionalString{}.Destroy(r.Bip353Address)
}

type FfiConverterPaymentRequestSource struct{}

var FfiConverterPaymentRequestSourceINSTANCE = FfiConverterPaymentRequestSource{}

func (c FfiConverterPaymentRequestSource) Lift(rb RustBufferI) PaymentRequestSource {
	return LiftFromRustBuffer[PaymentRequestSource](c, rb)
}

func (c FfiConverterPaymentRequestSource) Read(reader io.Reader) PaymentRequestSource {
	return PaymentRequestSource{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterPaymentRequestSource) Lower(value PaymentRequestSource) C.RustBuffer {
	return LowerIntoRustBuffer[PaymentRequestSource](c, value)
}

func (c FfiConverterPaymentRequestSource) Write(writer io.Writer, value PaymentRequestSource) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Bip21Uri)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Bip353Address)
}

type FfiDestroyerPaymentRequestSource struct{}

func (_ FfiDestroyerPaymentRequestSource) Destroy(value PaymentRequestSource) {
	value.Destroy()
}

// Denominator in an exchange rate
type Rate struct {
	Coin  string
	Value float64
}

func (r *Rate) Destroy() {
	FfiDestroyerString{}.Destroy(r.Coin)
	FfiDestroyerFloat64{}.Destroy(r.Value)
}

type FfiConverterRate struct{}

var FfiConverterRateINSTANCE = FfiConverterRate{}

func (c FfiConverterRate) Lift(rb RustBufferI) Rate {
	return LiftFromRustBuffer[Rate](c, rb)
}

func (c FfiConverterRate) Read(reader io.Reader) Rate {
	return Rate{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterFloat64INSTANCE.Read(reader),
	}
}

func (c FfiConverterRate) Lower(value Rate) C.RustBuffer {
	return LowerIntoRustBuffer[Rate](c, value)
}

func (c FfiConverterRate) Write(writer io.Writer, value Rate) {
	FfiConverterStringINSTANCE.Write(writer, value.Coin)
	FfiConverterFloat64INSTANCE.Write(writer, value.Value)
}

type FfiDestroyerRate struct{}

func (_ FfiDestroyerRate) Destroy(value Rate) {
	value.Destroy()
}

type RestResponse struct {
	Status uint16
	Body   string
}

func (r *RestResponse) Destroy() {
	FfiDestroyerUint16{}.Destroy(r.Status)
	FfiDestroyerString{}.Destroy(r.Body)
}

type FfiConverterRestResponse struct{}

var FfiConverterRestResponseINSTANCE = FfiConverterRestResponse{}

func (c FfiConverterRestResponse) Lift(rb RustBufferI) RestResponse {
	return LiftFromRustBuffer[RestResponse](c, rb)
}

func (c FfiConverterRestResponse) Read(reader io.Reader) RestResponse {
	return RestResponse{
		FfiConverterUint16INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterRestResponse) Lower(value RestResponse) C.RustBuffer {
	return LowerIntoRustBuffer[RestResponse](c, value)
}

func (c FfiConverterRestResponse) Write(writer io.Writer, value RestResponse) {
	FfiConverterUint16INSTANCE.Write(writer, value.Status)
	FfiConverterStringINSTANCE.Write(writer, value.Body)
}

type FfiDestroyerRestResponse struct{}

func (_ FfiDestroyerRestResponse) Destroy(value RestResponse) {
	value.Destroy()
}

type SatsPaymentDetails struct {
	Amount *uint64
}

func (r *SatsPaymentDetails) Destroy() {
	FfiDestroyerOptionalUint64{}.Destroy(r.Amount)
}

type FfiConverterSatsPaymentDetails struct{}

var FfiConverterSatsPaymentDetailsINSTANCE = FfiConverterSatsPaymentDetails{}

func (c FfiConverterSatsPaymentDetails) Lift(rb RustBufferI) SatsPaymentDetails {
	return LiftFromRustBuffer[SatsPaymentDetails](c, rb)
}

func (c FfiConverterSatsPaymentDetails) Read(reader io.Reader) SatsPaymentDetails {
	return SatsPaymentDetails{
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterSatsPaymentDetails) Lower(value SatsPaymentDetails) C.RustBuffer {
	return LowerIntoRustBuffer[SatsPaymentDetails](c, value)
}

func (c FfiConverterSatsPaymentDetails) Write(writer io.Writer, value SatsPaymentDetails) {
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.Amount)
}

type FfiDestroyerSatsPaymentDetails struct{}

func (_ FfiDestroyerSatsPaymentDetails) Destroy(value SatsPaymentDetails) {
	value.Destroy()
}

type SilentPaymentAddressDetails struct {
	Address string
	Network BitcoinNetwork
	Source  PaymentRequestSource
}

func (r *SilentPaymentAddressDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Address)
	FfiDestroyerBitcoinNetwork{}.Destroy(r.Network)
	FfiDestroyerPaymentRequestSource{}.Destroy(r.Source)
}

type FfiConverterSilentPaymentAddressDetails struct{}

var FfiConverterSilentPaymentAddressDetailsINSTANCE = FfiConverterSilentPaymentAddressDetails{}

func (c FfiConverterSilentPaymentAddressDetails) Lift(rb RustBufferI) SilentPaymentAddressDetails {
	return LiftFromRustBuffer[SilentPaymentAddressDetails](c, rb)
}

func (c FfiConverterSilentPaymentAddressDetails) Read(reader io.Reader) SilentPaymentAddressDetails {
	return SilentPaymentAddressDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterBitcoinNetworkINSTANCE.Read(reader),
		FfiConverterPaymentRequestSourceINSTANCE.Read(reader),
	}
}

func (c FfiConverterSilentPaymentAddressDetails) Lower(value SilentPaymentAddressDetails) C.RustBuffer {
	return LowerIntoRustBuffer[SilentPaymentAddressDetails](c, value)
}

func (c FfiConverterSilentPaymentAddressDetails) Write(writer io.Writer, value SilentPaymentAddressDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Address)
	FfiConverterBitcoinNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterPaymentRequestSourceINSTANCE.Write(writer, value.Source)
}

type FfiDestroyerSilentPaymentAddressDetails struct{}

func (_ FfiDestroyerSilentPaymentAddressDetails) Destroy(value SilentPaymentAddressDetails) {
	value.Destroy()
}

type SparkAddress struct {
	IdentityPublicKey  string
	Network            BitcoinNetwork
	SparkInvoiceFields *SparkInvoiceFields
	Signature          *string
}

func (r *SparkAddress) Destroy() {
	FfiDestroyerString{}.Destroy(r.IdentityPublicKey)
	FfiDestroyerBitcoinNetwork{}.Destroy(r.Network)
	FfiDestroyerOptionalSparkInvoiceFields{}.Destroy(r.SparkInvoiceFields)
	FfiDestroyerOptionalString{}.Destroy(r.Signature)
}

type FfiConverterSparkAddress struct{}

var FfiConverterSparkAddressINSTANCE = FfiConverterSparkAddress{}

func (c FfiConverterSparkAddress) Lift(rb RustBufferI) SparkAddress {
	return LiftFromRustBuffer[SparkAddress](c, rb)
}

func (c FfiConverterSparkAddress) Read(reader io.Reader) SparkAddress {
	return SparkAddress{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterBitcoinNetworkINSTANCE.Read(reader),
		FfiConverterOptionalSparkInvoiceFieldsINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterSparkAddress) Lower(value SparkAddress) C.RustBuffer {
	return LowerIntoRustBuffer[SparkAddress](c, value)
}

func (c FfiConverterSparkAddress) Write(writer io.Writer, value SparkAddress) {
	FfiConverterStringINSTANCE.Write(writer, value.IdentityPublicKey)
	FfiConverterBitcoinNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterOptionalSparkInvoiceFieldsINSTANCE.Write(writer, value.SparkInvoiceFields)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Signature)
}

type FfiDestroyerSparkAddress struct{}

func (_ FfiDestroyerSparkAddress) Destroy(value SparkAddress) {
	value.Destroy()
}

type SparkAddressDetails struct {
	Address        string
	DecodedAddress SparkAddress
	Source         PaymentRequestSource
}

func (r *SparkAddressDetails) Destroy() {
	FfiDestroyerString{}.Destroy(r.Address)
	FfiDestroyerSparkAddress{}.Destroy(r.DecodedAddress)
	FfiDestroyerPaymentRequestSource{}.Destroy(r.Source)
}

type FfiConverterSparkAddressDetails struct{}

var FfiConverterSparkAddressDetailsINSTANCE = FfiConverterSparkAddressDetails{}

func (c FfiConverterSparkAddressDetails) Lift(rb RustBufferI) SparkAddressDetails {
	return LiftFromRustBuffer[SparkAddressDetails](c, rb)
}

func (c FfiConverterSparkAddressDetails) Read(reader io.Reader) SparkAddressDetails {
	return SparkAddressDetails{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterSparkAddressINSTANCE.Read(reader),
		FfiConverterPaymentRequestSourceINSTANCE.Read(reader),
	}
}

func (c FfiConverterSparkAddressDetails) Lower(value SparkAddressDetails) C.RustBuffer {
	return LowerIntoRustBuffer[SparkAddressDetails](c, value)
}

func (c FfiConverterSparkAddressDetails) Write(writer io.Writer, value SparkAddressDetails) {
	FfiConverterStringINSTANCE.Write(writer, value.Address)
	FfiConverterSparkAddressINSTANCE.Write(writer, value.DecodedAddress)
	FfiConverterPaymentRequestSourceINSTANCE.Write(writer, value.Source)
}

type FfiDestroyerSparkAddressDetails struct{}

func (_ FfiDestroyerSparkAddressDetails) Destroy(value SparkAddressDetails) {
	value.Destroy()
}

type SparkInvoiceFields struct {
	Id              string
	Version         uint32
	Memo            *string
	SenderPublicKey *string
	ExpiryTime      *uint64
	PaymentType     *SparkAddressPaymentType
}

func (r *SparkInvoiceFields) Destroy() {
	FfiDestroyerString{}.Destroy(r.Id)
	FfiDestroyerUint32{}.Destroy(r.Version)
	FfiDestroyerOptionalString{}.Destroy(r.Memo)
	FfiDestroyerOptionalString{}.Destroy(r.SenderPublicKey)
	FfiDestroyerOptionalUint64{}.Destroy(r.ExpiryTime)
	FfiDestroyerOptionalSparkAddressPaymentType{}.Destroy(r.PaymentType)
}

type FfiConverterSparkInvoiceFields struct{}

var FfiConverterSparkInvoiceFieldsINSTANCE = FfiConverterSparkInvoiceFields{}

func (c FfiConverterSparkInvoiceFields) Lift(rb RustBufferI) SparkInvoiceFields {
	return LiftFromRustBuffer[SparkInvoiceFields](c, rb)
}

func (c FfiConverterSparkInvoiceFields) Read(reader io.Reader) SparkInvoiceFields {
	return SparkInvoiceFields{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalSparkAddressPaymentTypeINSTANCE.Read(reader),
	}
}

func (c FfiConverterSparkInvoiceFields) Lower(value SparkInvoiceFields) C.RustBuffer {
	return LowerIntoRustBuffer[SparkInvoiceFields](c, value)
}

func (c FfiConverterSparkInvoiceFields) Write(writer io.Writer, value SparkInvoiceFields) {
	FfiConverterStringINSTANCE.Write(writer, value.Id)
	FfiConverterUint32INSTANCE.Write(writer, value.Version)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Memo)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.SenderPublicKey)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.ExpiryTime)
	FfiConverterOptionalSparkAddressPaymentTypeINSTANCE.Write(writer, value.PaymentType)
}

type FfiDestroyerSparkInvoiceFields struct{}

func (_ FfiDestroyerSparkInvoiceFields) Destroy(value SparkInvoiceFields) {
	value.Destroy()
}

// Settings for the symbol representation of a currency
type Symbol struct {
	Grapheme *string
	Template *string
	Rtl      *bool
	Position *uint32
}

func (r *Symbol) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.Grapheme)
	FfiDestroyerOptionalString{}.Destroy(r.Template)
	FfiDestroyerOptionalBool{}.Destroy(r.Rtl)
	FfiDestroyerOptionalUint32{}.Destroy(r.Position)
}

type FfiConverterSymbol struct{}

var FfiConverterSymbolINSTANCE = FfiConverterSymbol{}

func (c FfiConverterSymbol) Lift(rb RustBufferI) Symbol {
	return LiftFromRustBuffer[Symbol](c, rb)
}

func (c FfiConverterSymbol) Read(reader io.Reader) Symbol {
	return Symbol{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalBoolINSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterSymbol) Lower(value Symbol) C.RustBuffer {
	return LowerIntoRustBuffer[Symbol](c, value)
}

func (c FfiConverterSymbol) Write(writer io.Writer, value Symbol) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Grapheme)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Template)
	FfiConverterOptionalBoolINSTANCE.Write(writer, value.Rtl)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Position)
}

type FfiDestroyerSymbol struct{}

func (_ FfiDestroyerSymbol) Destroy(value Symbol) {
	value.Destroy()
}

type TokensPaymentDetails struct {
	TokenIdentifier *string
	Amount          *uint64
}

func (r *TokensPaymentDetails) Destroy() {
	FfiDestroyerOptionalString{}.Destroy(r.TokenIdentifier)
	FfiDestroyerOptionalUint64{}.Destroy(r.Amount)
}

type FfiConverterTokensPaymentDetails struct{}

var FfiConverterTokensPaymentDetailsINSTANCE = FfiConverterTokensPaymentDetails{}

func (c FfiConverterTokensPaymentDetails) Lift(rb RustBufferI) TokensPaymentDetails {
	return LiftFromRustBuffer[TokensPaymentDetails](c, rb)
}

func (c FfiConverterTokensPaymentDetails) Read(reader io.Reader) TokensPaymentDetails {
	return TokensPaymentDetails{
		FfiConverterOptionalStringINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTokensPaymentDetails) Lower(value TokensPaymentDetails) C.RustBuffer {
	return LowerIntoRustBuffer[TokensPaymentDetails](c, value)
}

func (c FfiConverterTokensPaymentDetails) Write(writer io.Writer, value TokensPaymentDetails) {
	FfiConverterOptionalStringINSTANCE.Write(writer, value.TokenIdentifier)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.Amount)
}

type FfiDestroyerTokensPaymentDetails struct{}

func (_ FfiDestroyerTokensPaymentDetails) Destroy(value TokensPaymentDetails) {
	value.Destroy()
}

type UrlSuccessActionData struct {
	// Contents description, up to 144 characters
	Description string
	// URL of the success action
	Url string
	// Indicates the success URL domain matches the LNURL callback domain.
	//
	// See <https://github.com/lnurl/luds/blob/luds/09.md>
	MatchesCallbackDomain bool
}

func (r *UrlSuccessActionData) Destroy() {
	FfiDestroyerString{}.Destroy(r.Description)
	FfiDestroyerString{}.Destroy(r.Url)
	FfiDestroyerBool{}.Destroy(r.MatchesCallbackDomain)
}

type FfiConverterUrlSuccessActionData struct{}

var FfiConverterUrlSuccessActionDataINSTANCE = FfiConverterUrlSuccessActionData{}

func (c FfiConverterUrlSuccessActionData) Lift(rb RustBufferI) UrlSuccessActionData {
	return LiftFromRustBuffer[UrlSuccessActionData](c, rb)
}

func (c FfiConverterUrlSuccessActionData) Read(reader io.Reader) UrlSuccessActionData {
	return UrlSuccessActionData{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterUrlSuccessActionData) Lower(value UrlSuccessActionData) C.RustBuffer {
	return LowerIntoRustBuffer[UrlSuccessActionData](c, value)
}

func (c FfiConverterUrlSuccessActionData) Write(writer io.Writer, value UrlSuccessActionData) {
	FfiConverterStringINSTANCE.Write(writer, value.Description)
	FfiConverterStringINSTANCE.Write(writer, value.Url)
	FfiConverterBoolINSTANCE.Write(writer, value.MatchesCallbackDomain)
}

type FfiDestroyerUrlSuccessActionData struct{}

func (_ FfiDestroyerUrlSuccessActionData) Destroy(value UrlSuccessActionData) {
	value.Destroy()
}

// Result of decryption of [`AesSuccessActionData`] payload
type AesSuccessActionDataResult interface {
	Destroy()
}
type AesSuccessActionDataResultDecrypted struct {
	Data AesSuccessActionDataDecrypted
}

func (e AesSuccessActionDataResultDecrypted) Destroy() {
	FfiDestroyerAesSuccessActionDataDecrypted{}.Destroy(e.Data)
}

type AesSuccessActionDataResultErrorStatus struct {
	Reason string
}

func (e AesSuccessActionDataResultErrorStatus) Destroy() {
	FfiDestroyerString{}.Destroy(e.Reason)
}

type FfiConverterAesSuccessActionDataResult struct{}

var FfiConverterAesSuccessActionDataResultINSTANCE = FfiConverterAesSuccessActionDataResult{}

func (c FfiConverterAesSuccessActionDataResult) Lift(rb RustBufferI) AesSuccessActionDataResult {
	return LiftFromRustBuffer[AesSuccessActionDataResult](c, rb)
}

func (c FfiConverterAesSuccessActionDataResult) Lower(value AesSuccessActionDataResult) C.RustBuffer {
	return LowerIntoRustBuffer[AesSuccessActionDataResult](c, value)
}
func (FfiConverterAesSuccessActionDataResult) Read(reader io.Reader) AesSuccessActionDataResult {
	id := readInt32(reader)
	switch id {
	case 1:
		return AesSuccessActionDataResultDecrypted{
			FfiConverterAesSuccessActionDataDecryptedINSTANCE.Read(reader),
		}
	case 2:
		return AesSuccessActionDataResultErrorStatus{
			FfiConverterStringINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterAesSuccessActionDataResult.Read()", id))
	}
}

func (FfiConverterAesSuccessActionDataResult) Write(writer io.Writer, value AesSuccessActionDataResult) {
	switch variant_value := value.(type) {
	case AesSuccessActionDataResultDecrypted:
		writeInt32(writer, 1)
		FfiConverterAesSuccessActionDataDecryptedINSTANCE.Write(writer, variant_value.Data)
	case AesSuccessActionDataResultErrorStatus:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Reason)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterAesSuccessActionDataResult.Write", value))
	}
}

type FfiDestroyerAesSuccessActionDataResult struct{}

func (_ FfiDestroyerAesSuccessActionDataResult) Destroy(value AesSuccessActionDataResult) {
	value.Destroy()
}

type Amount interface {
	Destroy()
}
type AmountBitcoin struct {
	AmountMsat uint64
}

func (e AmountBitcoin) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.AmountMsat)
}

// An amount of currency specified using ISO 4712.
type AmountCurrency struct {
	Iso4217Code      string
	FractionalAmount uint64
}

func (e AmountCurrency) Destroy() {
	FfiDestroyerString{}.Destroy(e.Iso4217Code)
	FfiDestroyerUint64{}.Destroy(e.FractionalAmount)
}

type FfiConverterAmount struct{}

var FfiConverterAmountINSTANCE = FfiConverterAmount{}

func (c FfiConverterAmount) Lift(rb RustBufferI) Amount {
	return LiftFromRustBuffer[Amount](c, rb)
}

func (c FfiConverterAmount) Lower(value Amount) C.RustBuffer {
	return LowerIntoRustBuffer[Amount](c, value)
}
func (FfiConverterAmount) Read(reader io.Reader) Amount {
	id := readInt32(reader)
	switch id {
	case 1:
		return AmountBitcoin{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 2:
		return AmountCurrency{
			FfiConverterStringINSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterAmount.Read()", id))
	}
}

func (FfiConverterAmount) Write(writer io.Writer, value Amount) {
	switch variant_value := value.(type) {
	case AmountBitcoin:
		writeInt32(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.AmountMsat)
	case AmountCurrency:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Iso4217Code)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.FractionalAmount)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterAmount.Write", value))
	}
}

type FfiDestroyerAmount struct{}

func (_ FfiDestroyerAmount) Destroy(value Amount) {
	value.Destroy()
}

type BitcoinNetwork uint

const (
	// Mainnet
	BitcoinNetworkBitcoin  BitcoinNetwork = 1
	BitcoinNetworkTestnet3 BitcoinNetwork = 2
	BitcoinNetworkTestnet4 BitcoinNetwork = 3
	BitcoinNetworkSignet   BitcoinNetwork = 4
	BitcoinNetworkRegtest  BitcoinNetwork = 5
)

type FfiConverterBitcoinNetwork struct{}

var FfiConverterBitcoinNetworkINSTANCE = FfiConverterBitcoinNetwork{}

func (c FfiConverterBitcoinNetwork) Lift(rb RustBufferI) BitcoinNetwork {
	return LiftFromRustBuffer[BitcoinNetwork](c, rb)
}

func (c FfiConverterBitcoinNetwork) Lower(value BitcoinNetwork) C.RustBuffer {
	return LowerIntoRustBuffer[BitcoinNetwork](c, value)
}
func (FfiConverterBitcoinNetwork) Read(reader io.Reader) BitcoinNetwork {
	id := readInt32(reader)
	return BitcoinNetwork(id)
}

func (FfiConverterBitcoinNetwork) Write(writer io.Writer, value BitcoinNetwork) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerBitcoinNetwork struct{}

func (_ FfiDestroyerBitcoinNetwork) Destroy(value BitcoinNetwork) {
}

type InputType interface {
	Destroy()
}
type InputTypeBitcoinAddress struct {
	Field0 BitcoinAddressDetails
}

func (e InputTypeBitcoinAddress) Destroy() {
	FfiDestroyerBitcoinAddressDetails{}.Destroy(e.Field0)
}

type InputTypeBolt11Invoice struct {
	Field0 Bolt11InvoiceDetails
}

func (e InputTypeBolt11Invoice) Destroy() {
	FfiDestroyerBolt11InvoiceDetails{}.Destroy(e.Field0)
}

type InputTypeBolt12Invoice struct {
	Field0 Bolt12InvoiceDetails
}

func (e InputTypeBolt12Invoice) Destroy() {
	FfiDestroyerBolt12InvoiceDetails{}.Destroy(e.Field0)
}

type InputTypeBolt12Offer struct {
	Field0 Bolt12OfferDetails
}

func (e InputTypeBolt12Offer) Destroy() {
	FfiDestroyerBolt12OfferDetails{}.Destroy(e.Field0)
}

type InputTypeLightningAddress struct {
	Field0 LightningAddressDetails
}

func (e InputTypeLightningAddress) Destroy() {
	FfiDestroyerLightningAddressDetails{}.Destroy(e.Field0)
}

type InputTypeLnurlPay struct {
	Field0 LnurlPayRequestDetails
}

func (e InputTypeLnurlPay) Destroy() {
	FfiDestroyerLnurlPayRequestDetails{}.Destroy(e.Field0)
}

type InputTypeSilentPaymentAddress struct {
	Field0 SilentPaymentAddressDetails
}

func (e InputTypeSilentPaymentAddress) Destroy() {
	FfiDestroyerSilentPaymentAddressDetails{}.Destroy(e.Field0)
}

type InputTypeLnurlAuth struct {
	Field0 LnurlAuthRequestDetails
}

func (e InputTypeLnurlAuth) Destroy() {
	FfiDestroyerLnurlAuthRequestDetails{}.Destroy(e.Field0)
}

type InputTypeUrl struct {
	Field0 string
}

func (e InputTypeUrl) Destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

type InputTypeBip21 struct {
	Field0 Bip21Details
}

func (e InputTypeBip21) Destroy() {
	FfiDestroyerBip21Details{}.Destroy(e.Field0)
}

type InputTypeBolt12InvoiceRequest struct {
	Field0 Bolt12InvoiceRequestDetails
}

func (e InputTypeBolt12InvoiceRequest) Destroy() {
	FfiDestroyerBolt12InvoiceRequestDetails{}.Destroy(e.Field0)
}

type InputTypeLnurlWithdraw struct {
	Field0 LnurlWithdrawRequestDetails
}

func (e InputTypeLnurlWithdraw) Destroy() {
	FfiDestroyerLnurlWithdrawRequestDetails{}.Destroy(e.Field0)
}

type InputTypeSparkAddress struct {
	Field0 SparkAddressDetails
}

func (e InputTypeSparkAddress) Destroy() {
	FfiDestroyerSparkAddressDetails{}.Destroy(e.Field0)
}

type FfiConverterInputType struct{}

var FfiConverterInputTypeINSTANCE = FfiConverterInputType{}

func (c FfiConverterInputType) Lift(rb RustBufferI) InputType {
	return LiftFromRustBuffer[InputType](c, rb)
}

func (c FfiConverterInputType) Lower(value InputType) C.RustBuffer {
	return LowerIntoRustBuffer[InputType](c, value)
}
func (FfiConverterInputType) Read(reader io.Reader) InputType {
	id := readInt32(reader)
	switch id {
	case 1:
		return InputTypeBitcoinAddress{
			FfiConverterBitcoinAddressDetailsINSTANCE.Read(reader),
		}
	case 2:
		return InputTypeBolt11Invoice{
			FfiConverterBolt11InvoiceDetailsINSTANCE.Read(reader),
		}
	case 3:
		return InputTypeBolt12Invoice{
			FfiConverterBolt12InvoiceDetailsINSTANCE.Read(reader),
		}
	case 4:
		return InputTypeBolt12Offer{
			FfiConverterBolt12OfferDetailsINSTANCE.Read(reader),
		}
	case 5:
		return InputTypeLightningAddress{
			FfiConverterLightningAddressDetailsINSTANCE.Read(reader),
		}
	case 6:
		return InputTypeLnurlPay{
			FfiConverterLnurlPayRequestDetailsINSTANCE.Read(reader),
		}
	case 7:
		return InputTypeSilentPaymentAddress{
			FfiConverterSilentPaymentAddressDetailsINSTANCE.Read(reader),
		}
	case 8:
		return InputTypeLnurlAuth{
			FfiConverterLnurlAuthRequestDetailsINSTANCE.Read(reader),
		}
	case 9:
		return InputTypeUrl{
			FfiConverterStringINSTANCE.Read(reader),
		}
	case 10:
		return InputTypeBip21{
			FfiConverterBip21DetailsINSTANCE.Read(reader),
		}
	case 11:
		return InputTypeBolt12InvoiceRequest{
			FfiConverterBolt12InvoiceRequestDetailsINSTANCE.Read(reader),
		}
	case 12:
		return InputTypeLnurlWithdraw{
			FfiConverterLnurlWithdrawRequestDetailsINSTANCE.Read(reader),
		}
	case 13:
		return InputTypeSparkAddress{
			FfiConverterSparkAddressDetailsINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterInputType.Read()", id))
	}
}

func (FfiConverterInputType) Write(writer io.Writer, value InputType) {
	switch variant_value := value.(type) {
	case InputTypeBitcoinAddress:
		writeInt32(writer, 1)
		FfiConverterBitcoinAddressDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeBolt11Invoice:
		writeInt32(writer, 2)
		FfiConverterBolt11InvoiceDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeBolt12Invoice:
		writeInt32(writer, 3)
		FfiConverterBolt12InvoiceDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeBolt12Offer:
		writeInt32(writer, 4)
		FfiConverterBolt12OfferDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeLightningAddress:
		writeInt32(writer, 5)
		FfiConverterLightningAddressDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeLnurlPay:
		writeInt32(writer, 6)
		FfiConverterLnurlPayRequestDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeSilentPaymentAddress:
		writeInt32(writer, 7)
		FfiConverterSilentPaymentAddressDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeLnurlAuth:
		writeInt32(writer, 8)
		FfiConverterLnurlAuthRequestDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeUrl:
		writeInt32(writer, 9)
		FfiConverterStringINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeBip21:
		writeInt32(writer, 10)
		FfiConverterBip21DetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeBolt12InvoiceRequest:
		writeInt32(writer, 11)
		FfiConverterBolt12InvoiceRequestDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeLnurlWithdraw:
		writeInt32(writer, 12)
		FfiConverterLnurlWithdrawRequestDetailsINSTANCE.Write(writer, variant_value.Field0)
	case InputTypeSparkAddress:
		writeInt32(writer, 13)
		FfiConverterSparkAddressDetailsINSTANCE.Write(writer, variant_value.Field0)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterInputType.Write", value))
	}
}

type FfiDestroyerInputType struct{}

func (_ FfiDestroyerInputType) Destroy(value InputType) {
	value.Destroy()
}

// Contains the result of the entire LNURL interaction, as reported by the LNURL endpoint.
//
// * `Ok` indicates the interaction with the endpoint was valid, and the endpoint
// - started to pay the invoice asynchronously in the case of LNURL-withdraw,
// - verified the client signature in the case of LNURL-auth
// * `Error` indicates a generic issue the LNURL endpoint encountered, including a freetext
// description of the reason.
//
// Both cases are described in LUD-03 <https://github.com/lnurl/luds/blob/luds/03.md> & LUD-04: <https://github.com/lnurl/luds/blob/luds/04.md>
type LnurlCallbackStatus interface {
	Destroy()
}

// On-wire format is: `{"status": "OK"}`
type LnurlCallbackStatusOk struct {
}

func (e LnurlCallbackStatusOk) Destroy() {
}

// On-wire format is: `{"status": "ERROR", "reason": "error details..."}`
type LnurlCallbackStatusErrorStatus struct {
	ErrorDetails LnurlErrorDetails
}

func (e LnurlCallbackStatusErrorStatus) Destroy() {
	FfiDestroyerLnurlErrorDetails{}.Destroy(e.ErrorDetails)
}

type FfiConverterLnurlCallbackStatus struct{}

var FfiConverterLnurlCallbackStatusINSTANCE = FfiConverterLnurlCallbackStatus{}

func (c FfiConverterLnurlCallbackStatus) Lift(rb RustBufferI) LnurlCallbackStatus {
	return LiftFromRustBuffer[LnurlCallbackStatus](c, rb)
}

func (c FfiConverterLnurlCallbackStatus) Lower(value LnurlCallbackStatus) C.RustBuffer {
	return LowerIntoRustBuffer[LnurlCallbackStatus](c, value)
}
func (FfiConverterLnurlCallbackStatus) Read(reader io.Reader) LnurlCallbackStatus {
	id := readInt32(reader)
	switch id {
	case 1:
		return LnurlCallbackStatusOk{}
	case 2:
		return LnurlCallbackStatusErrorStatus{
			FfiConverterLnurlErrorDetailsINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterLnurlCallbackStatus.Read()", id))
	}
}

func (FfiConverterLnurlCallbackStatus) Write(writer io.Writer, value LnurlCallbackStatus) {
	switch variant_value := value.(type) {
	case LnurlCallbackStatusOk:
		writeInt32(writer, 1)
	case LnurlCallbackStatusErrorStatus:
		writeInt32(writer, 2)
		FfiConverterLnurlErrorDetailsINSTANCE.Write(writer, variant_value.ErrorDetails)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterLnurlCallbackStatus.Write", value))
	}
}

type FfiDestroyerLnurlCallbackStatus struct{}

func (_ FfiDestroyerLnurlCallbackStatus) Destroy(value LnurlCallbackStatus) {
	value.Destroy()
}

type ServiceConnectivityError struct {
	err error
}

// Convience method to turn *ServiceConnectivityError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ServiceConnectivityError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ServiceConnectivityError) Error() string {
	return fmt.Sprintf("ServiceConnectivityError: %s", err.err.Error())
}

func (err ServiceConnectivityError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrServiceConnectivityErrorBuilder = fmt.Errorf("ServiceConnectivityErrorBuilder")
var ErrServiceConnectivityErrorRedirect = fmt.Errorf("ServiceConnectivityErrorRedirect")
var ErrServiceConnectivityErrorStatus = fmt.Errorf("ServiceConnectivityErrorStatus")
var ErrServiceConnectivityErrorTimeout = fmt.Errorf("ServiceConnectivityErrorTimeout")
var ErrServiceConnectivityErrorRequest = fmt.Errorf("ServiceConnectivityErrorRequest")
var ErrServiceConnectivityErrorConnect = fmt.Errorf("ServiceConnectivityErrorConnect")
var ErrServiceConnectivityErrorBody = fmt.Errorf("ServiceConnectivityErrorBody")
var ErrServiceConnectivityErrorDecode = fmt.Errorf("ServiceConnectivityErrorDecode")
var ErrServiceConnectivityErrorJson = fmt.Errorf("ServiceConnectivityErrorJson")
var ErrServiceConnectivityErrorOther = fmt.Errorf("ServiceConnectivityErrorOther")

// Variant structs
type ServiceConnectivityErrorBuilder struct {
	Field0 string
}

func NewServiceConnectivityErrorBuilder(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorBuilder{
		Field0: var0}}
}

func (e ServiceConnectivityErrorBuilder) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorBuilder) Error() string {
	return fmt.Sprint("Builder",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorBuilder) Is(target error) bool {
	return target == ErrServiceConnectivityErrorBuilder
}

type ServiceConnectivityErrorRedirect struct {
	Field0 string
}

func NewServiceConnectivityErrorRedirect(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorRedirect{
		Field0: var0}}
}

func (e ServiceConnectivityErrorRedirect) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorRedirect) Error() string {
	return fmt.Sprint("Redirect",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorRedirect) Is(target error) bool {
	return target == ErrServiceConnectivityErrorRedirect
}

type ServiceConnectivityErrorStatus struct {
	Status uint16
	Body   string
}

func NewServiceConnectivityErrorStatus(
	status uint16,
	body string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorStatus{
		Status: status,
		Body:   body}}
}

func (e ServiceConnectivityErrorStatus) destroy() {
	FfiDestroyerUint16{}.Destroy(e.Status)
	FfiDestroyerString{}.Destroy(e.Body)
}

func (err ServiceConnectivityErrorStatus) Error() string {
	return fmt.Sprint("Status",
		": ",

		"Status=",
		err.Status,
		", ",
		"Body=",
		err.Body,
	)
}

func (self ServiceConnectivityErrorStatus) Is(target error) bool {
	return target == ErrServiceConnectivityErrorStatus
}

type ServiceConnectivityErrorTimeout struct {
	Field0 string
}

func NewServiceConnectivityErrorTimeout(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorTimeout{
		Field0: var0}}
}

func (e ServiceConnectivityErrorTimeout) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorTimeout) Error() string {
	return fmt.Sprint("Timeout",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorTimeout) Is(target error) bool {
	return target == ErrServiceConnectivityErrorTimeout
}

type ServiceConnectivityErrorRequest struct {
	Field0 string
}

func NewServiceConnectivityErrorRequest(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorRequest{
		Field0: var0}}
}

func (e ServiceConnectivityErrorRequest) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorRequest) Error() string {
	return fmt.Sprint("Request",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorRequest) Is(target error) bool {
	return target == ErrServiceConnectivityErrorRequest
}

type ServiceConnectivityErrorConnect struct {
	Field0 string
}

func NewServiceConnectivityErrorConnect(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorConnect{
		Field0: var0}}
}

func (e ServiceConnectivityErrorConnect) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorConnect) Error() string {
	return fmt.Sprint("Connect",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorConnect) Is(target error) bool {
	return target == ErrServiceConnectivityErrorConnect
}

type ServiceConnectivityErrorBody struct {
	Field0 string
}

func NewServiceConnectivityErrorBody(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorBody{
		Field0: var0}}
}

func (e ServiceConnectivityErrorBody) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorBody) Error() string {
	return fmt.Sprint("Body",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorBody) Is(target error) bool {
	return target == ErrServiceConnectivityErrorBody
}

type ServiceConnectivityErrorDecode struct {
	Field0 string
}

func NewServiceConnectivityErrorDecode(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorDecode{
		Field0: var0}}
}

func (e ServiceConnectivityErrorDecode) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorDecode) Error() string {
	return fmt.Sprint("Decode",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorDecode) Is(target error) bool {
	return target == ErrServiceConnectivityErrorDecode
}

type ServiceConnectivityErrorJson struct {
	Field0 string
}

func NewServiceConnectivityErrorJson(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorJson{
		Field0: var0}}
}

func (e ServiceConnectivityErrorJson) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorJson) Error() string {
	return fmt.Sprint("Json",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorJson) Is(target error) bool {
	return target == ErrServiceConnectivityErrorJson
}

type ServiceConnectivityErrorOther struct {
	Field0 string
}

func NewServiceConnectivityErrorOther(
	var0 string,
) *ServiceConnectivityError {
	return &ServiceConnectivityError{err: &ServiceConnectivityErrorOther{
		Field0: var0}}
}

func (e ServiceConnectivityErrorOther) destroy() {
	FfiDestroyerString{}.Destroy(e.Field0)
}

func (err ServiceConnectivityErrorOther) Error() string {
	return fmt.Sprint("Other",
		": ",

		"Field0=",
		err.Field0,
	)
}

func (self ServiceConnectivityErrorOther) Is(target error) bool {
	return target == ErrServiceConnectivityErrorOther
}

type FfiConverterServiceConnectivityError struct{}

var FfiConverterServiceConnectivityErrorINSTANCE = FfiConverterServiceConnectivityError{}

func (c FfiConverterServiceConnectivityError) Lift(eb RustBufferI) *ServiceConnectivityError {
	return LiftFromRustBuffer[*ServiceConnectivityError](c, eb)
}

func (c FfiConverterServiceConnectivityError) Lower(value *ServiceConnectivityError) C.RustBuffer {
	return LowerIntoRustBuffer[*ServiceConnectivityError](c, value)
}

func (c FfiConverterServiceConnectivityError) Read(reader io.Reader) *ServiceConnectivityError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &ServiceConnectivityError{&ServiceConnectivityErrorBuilder{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &ServiceConnectivityError{&ServiceConnectivityErrorRedirect{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &ServiceConnectivityError{&ServiceConnectivityErrorStatus{
			Status: FfiConverterUint16INSTANCE.Read(reader),
			Body:   FfiConverterStringINSTANCE.Read(reader),
		}}
	case 4:
		return &ServiceConnectivityError{&ServiceConnectivityErrorTimeout{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 5:
		return &ServiceConnectivityError{&ServiceConnectivityErrorRequest{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 6:
		return &ServiceConnectivityError{&ServiceConnectivityErrorConnect{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 7:
		return &ServiceConnectivityError{&ServiceConnectivityErrorBody{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 8:
		return &ServiceConnectivityError{&ServiceConnectivityErrorDecode{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 9:
		return &ServiceConnectivityError{&ServiceConnectivityErrorJson{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 10:
		return &ServiceConnectivityError{&ServiceConnectivityErrorOther{
			Field0: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterServiceConnectivityError.Read()", errorID))
	}
}

func (c FfiConverterServiceConnectivityError) Write(writer io.Writer, value *ServiceConnectivityError) {
	switch variantValue := value.err.(type) {
	case *ServiceConnectivityErrorBuilder:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorRedirect:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorStatus:
		writeInt32(writer, 3)
		FfiConverterUint16INSTANCE.Write(writer, variantValue.Status)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Body)
	case *ServiceConnectivityErrorTimeout:
		writeInt32(writer, 4)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorRequest:
		writeInt32(writer, 5)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorConnect:
		writeInt32(writer, 6)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorBody:
		writeInt32(writer, 7)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorDecode:
		writeInt32(writer, 8)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorJson:
		writeInt32(writer, 9)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	case *ServiceConnectivityErrorOther:
		writeInt32(writer, 10)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Field0)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterServiceConnectivityError.Write", value))
	}
}

type FfiDestroyerServiceConnectivityError struct{}

func (_ FfiDestroyerServiceConnectivityError) Destroy(value *ServiceConnectivityError) {
	switch variantValue := value.err.(type) {
	case ServiceConnectivityErrorBuilder:
		variantValue.destroy()
	case ServiceConnectivityErrorRedirect:
		variantValue.destroy()
	case ServiceConnectivityErrorStatus:
		variantValue.destroy()
	case ServiceConnectivityErrorTimeout:
		variantValue.destroy()
	case ServiceConnectivityErrorRequest:
		variantValue.destroy()
	case ServiceConnectivityErrorConnect:
		variantValue.destroy()
	case ServiceConnectivityErrorBody:
		variantValue.destroy()
	case ServiceConnectivityErrorDecode:
		variantValue.destroy()
	case ServiceConnectivityErrorJson:
		variantValue.destroy()
	case ServiceConnectivityErrorOther:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerServiceConnectivityError.Destroy", value))
	}
}

type SparkAddressPaymentType interface {
	Destroy()
}
type SparkAddressPaymentTypeTokensPayment struct {
	Field0 TokensPaymentDetails
}

func (e SparkAddressPaymentTypeTokensPayment) Destroy() {
	FfiDestroyerTokensPaymentDetails{}.Destroy(e.Field0)
}

type SparkAddressPaymentTypeSatsPayment struct {
	Field0 SatsPaymentDetails
}

func (e SparkAddressPaymentTypeSatsPayment) Destroy() {
	FfiDestroyerSatsPaymentDetails{}.Destroy(e.Field0)
}

type FfiConverterSparkAddressPaymentType struct{}

var FfiConverterSparkAddressPaymentTypeINSTANCE = FfiConverterSparkAddressPaymentType{}

func (c FfiConverterSparkAddressPaymentType) Lift(rb RustBufferI) SparkAddressPaymentType {
	return LiftFromRustBuffer[SparkAddressPaymentType](c, rb)
}

func (c FfiConverterSparkAddressPaymentType) Lower(value SparkAddressPaymentType) C.RustBuffer {
	return LowerIntoRustBuffer[SparkAddressPaymentType](c, value)
}
func (FfiConverterSparkAddressPaymentType) Read(reader io.Reader) SparkAddressPaymentType {
	id := readInt32(reader)
	switch id {
	case 1:
		return SparkAddressPaymentTypeTokensPayment{
			FfiConverterTokensPaymentDetailsINSTANCE.Read(reader),
		}
	case 2:
		return SparkAddressPaymentTypeSatsPayment{
			FfiConverterSatsPaymentDetailsINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSparkAddressPaymentType.Read()", id))
	}
}

func (FfiConverterSparkAddressPaymentType) Write(writer io.Writer, value SparkAddressPaymentType) {
	switch variant_value := value.(type) {
	case SparkAddressPaymentTypeTokensPayment:
		writeInt32(writer, 1)
		FfiConverterTokensPaymentDetailsINSTANCE.Write(writer, variant_value.Field0)
	case SparkAddressPaymentTypeSatsPayment:
		writeInt32(writer, 2)
		FfiConverterSatsPaymentDetailsINSTANCE.Write(writer, variant_value.Field0)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSparkAddressPaymentType.Write", value))
	}
}

type FfiDestroyerSparkAddressPaymentType struct{}

func (_ FfiDestroyerSparkAddressPaymentType) Destroy(value SparkAddressPaymentType) {
	value.Destroy()
}

// Supported success action types
//
// Receiving any other (unsupported) success action type will result in a failed parsing,
// which will abort the LNURL-pay workflow, as per LUD-09.
type SuccessAction interface {
	Destroy()
}

// AES type, described in LUD-10
type SuccessActionAes struct {
	Data AesSuccessActionData
}

func (e SuccessActionAes) Destroy() {
	FfiDestroyerAesSuccessActionData{}.Destroy(e.Data)
}

// Message type, described in LUD-09
type SuccessActionMessage struct {
	Data MessageSuccessActionData
}

func (e SuccessActionMessage) Destroy() {
	FfiDestroyerMessageSuccessActionData{}.Destroy(e.Data)
}

// URL type, described in LUD-09
type SuccessActionUrl struct {
	Data UrlSuccessActionData
}

func (e SuccessActionUrl) Destroy() {
	FfiDestroyerUrlSuccessActionData{}.Destroy(e.Data)
}

type FfiConverterSuccessAction struct{}

var FfiConverterSuccessActionINSTANCE = FfiConverterSuccessAction{}

func (c FfiConverterSuccessAction) Lift(rb RustBufferI) SuccessAction {
	return LiftFromRustBuffer[SuccessAction](c, rb)
}

func (c FfiConverterSuccessAction) Lower(value SuccessAction) C.RustBuffer {
	return LowerIntoRustBuffer[SuccessAction](c, value)
}
func (FfiConverterSuccessAction) Read(reader io.Reader) SuccessAction {
	id := readInt32(reader)
	switch id {
	case 1:
		return SuccessActionAes{
			FfiConverterAesSuccessActionDataINSTANCE.Read(reader),
		}
	case 2:
		return SuccessActionMessage{
			FfiConverterMessageSuccessActionDataINSTANCE.Read(reader),
		}
	case 3:
		return SuccessActionUrl{
			FfiConverterUrlSuccessActionDataINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSuccessAction.Read()", id))
	}
}

func (FfiConverterSuccessAction) Write(writer io.Writer, value SuccessAction) {
	switch variant_value := value.(type) {
	case SuccessActionAes:
		writeInt32(writer, 1)
		FfiConverterAesSuccessActionDataINSTANCE.Write(writer, variant_value.Data)
	case SuccessActionMessage:
		writeInt32(writer, 2)
		FfiConverterMessageSuccessActionDataINSTANCE.Write(writer, variant_value.Data)
	case SuccessActionUrl:
		writeInt32(writer, 3)
		FfiConverterUrlSuccessActionDataINSTANCE.Write(writer, variant_value.Data)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSuccessAction.Write", value))
	}
}

type FfiDestroyerSuccessAction struct{}

func (_ FfiDestroyerSuccessAction) Destroy(value SuccessAction) {
	value.Destroy()
}

// [`SuccessAction`] where contents are ready to be consumed by the caller
//
// Contents are identical to [`SuccessAction`], except for AES where the ciphertext is decrypted.
type SuccessActionProcessed interface {
	Destroy()
}

// See [`SuccessAction::Aes`] for received payload
//
// See [`AesSuccessActionDataDecrypted`] for decrypted payload
type SuccessActionProcessedAes struct {
	Result AesSuccessActionDataResult
}

func (e SuccessActionProcessedAes) Destroy() {
	FfiDestroyerAesSuccessActionDataResult{}.Destroy(e.Result)
}

// See [`SuccessAction::Message`]
type SuccessActionProcessedMessage struct {
	Data MessageSuccessActionData
}

func (e SuccessActionProcessedMessage) Destroy() {
	FfiDestroyerMessageSuccessActionData{}.Destroy(e.Data)
}

// See [`SuccessAction::Url`]
type SuccessActionProcessedUrl struct {
	Data UrlSuccessActionData
}

func (e SuccessActionProcessedUrl) Destroy() {
	FfiDestroyerUrlSuccessActionData{}.Destroy(e.Data)
}

type FfiConverterSuccessActionProcessed struct{}

var FfiConverterSuccessActionProcessedINSTANCE = FfiConverterSuccessActionProcessed{}

func (c FfiConverterSuccessActionProcessed) Lift(rb RustBufferI) SuccessActionProcessed {
	return LiftFromRustBuffer[SuccessActionProcessed](c, rb)
}

func (c FfiConverterSuccessActionProcessed) Lower(value SuccessActionProcessed) C.RustBuffer {
	return LowerIntoRustBuffer[SuccessActionProcessed](c, value)
}
func (FfiConverterSuccessActionProcessed) Read(reader io.Reader) SuccessActionProcessed {
	id := readInt32(reader)
	switch id {
	case 1:
		return SuccessActionProcessedAes{
			FfiConverterAesSuccessActionDataResultINSTANCE.Read(reader),
		}
	case 2:
		return SuccessActionProcessedMessage{
			FfiConverterMessageSuccessActionDataINSTANCE.Read(reader),
		}
	case 3:
		return SuccessActionProcessedUrl{
			FfiConverterUrlSuccessActionDataINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSuccessActionProcessed.Read()", id))
	}
}

func (FfiConverterSuccessActionProcessed) Write(writer io.Writer, value SuccessActionProcessed) {
	switch variant_value := value.(type) {
	case SuccessActionProcessedAes:
		writeInt32(writer, 1)
		FfiConverterAesSuccessActionDataResultINSTANCE.Write(writer, variant_value.Result)
	case SuccessActionProcessedMessage:
		writeInt32(writer, 2)
		FfiConverterMessageSuccessActionDataINSTANCE.Write(writer, variant_value.Data)
	case SuccessActionProcessedUrl:
		writeInt32(writer, 3)
		FfiConverterUrlSuccessActionDataINSTANCE.Write(writer, variant_value.Data)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSuccessActionProcessed.Write", value))
	}
}

type FfiDestroyerSuccessActionProcessed struct{}

func (_ FfiDestroyerSuccessActionProcessed) Destroy(value SuccessActionProcessed) {
	value.Destroy()
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

type FfiConverterOptionalSparkInvoiceFields struct{}

var FfiConverterOptionalSparkInvoiceFieldsINSTANCE = FfiConverterOptionalSparkInvoiceFields{}

func (c FfiConverterOptionalSparkInvoiceFields) Lift(rb RustBufferI) *SparkInvoiceFields {
	return LiftFromRustBuffer[*SparkInvoiceFields](c, rb)
}

func (_ FfiConverterOptionalSparkInvoiceFields) Read(reader io.Reader) *SparkInvoiceFields {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSparkInvoiceFieldsINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSparkInvoiceFields) Lower(value *SparkInvoiceFields) C.RustBuffer {
	return LowerIntoRustBuffer[*SparkInvoiceFields](c, value)
}

func (_ FfiConverterOptionalSparkInvoiceFields) Write(writer io.Writer, value *SparkInvoiceFields) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSparkInvoiceFieldsINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSparkInvoiceFields struct{}

func (_ FfiDestroyerOptionalSparkInvoiceFields) Destroy(value *SparkInvoiceFields) {
	if value != nil {
		FfiDestroyerSparkInvoiceFields{}.Destroy(*value)
	}
}

type FfiConverterOptionalSymbol struct{}

var FfiConverterOptionalSymbolINSTANCE = FfiConverterOptionalSymbol{}

func (c FfiConverterOptionalSymbol) Lift(rb RustBufferI) *Symbol {
	return LiftFromRustBuffer[*Symbol](c, rb)
}

func (_ FfiConverterOptionalSymbol) Read(reader io.Reader) *Symbol {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSymbolINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSymbol) Lower(value *Symbol) C.RustBuffer {
	return LowerIntoRustBuffer[*Symbol](c, value)
}

func (_ FfiConverterOptionalSymbol) Write(writer io.Writer, value *Symbol) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSymbolINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSymbol struct{}

func (_ FfiDestroyerOptionalSymbol) Destroy(value *Symbol) {
	if value != nil {
		FfiDestroyerSymbol{}.Destroy(*value)
	}
}

type FfiConverterOptionalAmount struct{}

var FfiConverterOptionalAmountINSTANCE = FfiConverterOptionalAmount{}

func (c FfiConverterOptionalAmount) Lift(rb RustBufferI) *Amount {
	return LiftFromRustBuffer[*Amount](c, rb)
}

func (_ FfiConverterOptionalAmount) Read(reader io.Reader) *Amount {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterAmountINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalAmount) Lower(value *Amount) C.RustBuffer {
	return LowerIntoRustBuffer[*Amount](c, value)
}

func (_ FfiConverterOptionalAmount) Write(writer io.Writer, value *Amount) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterAmountINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalAmount struct{}

func (_ FfiDestroyerOptionalAmount) Destroy(value *Amount) {
	if value != nil {
		FfiDestroyerAmount{}.Destroy(*value)
	}
}

type FfiConverterOptionalSparkAddressPaymentType struct{}

var FfiConverterOptionalSparkAddressPaymentTypeINSTANCE = FfiConverterOptionalSparkAddressPaymentType{}

func (c FfiConverterOptionalSparkAddressPaymentType) Lift(rb RustBufferI) *SparkAddressPaymentType {
	return LiftFromRustBuffer[*SparkAddressPaymentType](c, rb)
}

func (_ FfiConverterOptionalSparkAddressPaymentType) Read(reader io.Reader) *SparkAddressPaymentType {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSparkAddressPaymentTypeINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSparkAddressPaymentType) Lower(value *SparkAddressPaymentType) C.RustBuffer {
	return LowerIntoRustBuffer[*SparkAddressPaymentType](c, value)
}

func (_ FfiConverterOptionalSparkAddressPaymentType) Write(writer io.Writer, value *SparkAddressPaymentType) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSparkAddressPaymentTypeINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSparkAddressPaymentType struct{}

func (_ FfiDestroyerOptionalSparkAddressPaymentType) Destroy(value *SparkAddressPaymentType) {
	if value != nil {
		FfiDestroyerSparkAddressPaymentType{}.Destroy(*value)
	}
}

type FfiConverterOptionalMapStringString struct{}

var FfiConverterOptionalMapStringStringINSTANCE = FfiConverterOptionalMapStringString{}

func (c FfiConverterOptionalMapStringString) Lift(rb RustBufferI) *map[string]string {
	return LiftFromRustBuffer[*map[string]string](c, rb)
}

func (_ FfiConverterOptionalMapStringString) Read(reader io.Reader) *map[string]string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterMapStringStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalMapStringString) Lower(value *map[string]string) C.RustBuffer {
	return LowerIntoRustBuffer[*map[string]string](c, value)
}

func (_ FfiConverterOptionalMapStringString) Write(writer io.Writer, value *map[string]string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterMapStringStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalMapStringString struct{}

func (_ FfiDestroyerOptionalMapStringString) Destroy(value *map[string]string) {
	if value != nil {
		FfiDestroyerMapStringString{}.Destroy(*value)
	}
}

type FfiConverterSequenceString struct{}

var FfiConverterSequenceStringINSTANCE = FfiConverterSequenceString{}

func (c FfiConverterSequenceString) Lift(rb RustBufferI) []string {
	return LiftFromRustBuffer[[]string](c, rb)
}

func (c FfiConverterSequenceString) Read(reader io.Reader) []string {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]string, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterStringINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceString) Lower(value []string) C.RustBuffer {
	return LowerIntoRustBuffer[[]string](c, value)
}

func (c FfiConverterSequenceString) Write(writer io.Writer, value []string) {
	if len(value) > math.MaxInt32 {
		panic("[]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterStringINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceString struct{}

func (FfiDestroyerSequenceString) Destroy(sequence []string) {
	for _, value := range sequence {
		FfiDestroyerString{}.Destroy(value)
	}
}

type FfiConverterSequenceBip21Extra struct{}

var FfiConverterSequenceBip21ExtraINSTANCE = FfiConverterSequenceBip21Extra{}

func (c FfiConverterSequenceBip21Extra) Lift(rb RustBufferI) []Bip21Extra {
	return LiftFromRustBuffer[[]Bip21Extra](c, rb)
}

func (c FfiConverterSequenceBip21Extra) Read(reader io.Reader) []Bip21Extra {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Bip21Extra, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBip21ExtraINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBip21Extra) Lower(value []Bip21Extra) C.RustBuffer {
	return LowerIntoRustBuffer[[]Bip21Extra](c, value)
}

func (c FfiConverterSequenceBip21Extra) Write(writer io.Writer, value []Bip21Extra) {
	if len(value) > math.MaxInt32 {
		panic("[]Bip21Extra is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBip21ExtraINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBip21Extra struct{}

func (FfiDestroyerSequenceBip21Extra) Destroy(sequence []Bip21Extra) {
	for _, value := range sequence {
		FfiDestroyerBip21Extra{}.Destroy(value)
	}
}

type FfiConverterSequenceBolt11RouteHint struct{}

var FfiConverterSequenceBolt11RouteHintINSTANCE = FfiConverterSequenceBolt11RouteHint{}

func (c FfiConverterSequenceBolt11RouteHint) Lift(rb RustBufferI) []Bolt11RouteHint {
	return LiftFromRustBuffer[[]Bolt11RouteHint](c, rb)
}

func (c FfiConverterSequenceBolt11RouteHint) Read(reader io.Reader) []Bolt11RouteHint {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Bolt11RouteHint, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBolt11RouteHintINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBolt11RouteHint) Lower(value []Bolt11RouteHint) C.RustBuffer {
	return LowerIntoRustBuffer[[]Bolt11RouteHint](c, value)
}

func (c FfiConverterSequenceBolt11RouteHint) Write(writer io.Writer, value []Bolt11RouteHint) {
	if len(value) > math.MaxInt32 {
		panic("[]Bolt11RouteHint is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBolt11RouteHintINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBolt11RouteHint struct{}

func (FfiDestroyerSequenceBolt11RouteHint) Destroy(sequence []Bolt11RouteHint) {
	for _, value := range sequence {
		FfiDestroyerBolt11RouteHint{}.Destroy(value)
	}
}

type FfiConverterSequenceBolt11RouteHintHop struct{}

var FfiConverterSequenceBolt11RouteHintHopINSTANCE = FfiConverterSequenceBolt11RouteHintHop{}

func (c FfiConverterSequenceBolt11RouteHintHop) Lift(rb RustBufferI) []Bolt11RouteHintHop {
	return LiftFromRustBuffer[[]Bolt11RouteHintHop](c, rb)
}

func (c FfiConverterSequenceBolt11RouteHintHop) Read(reader io.Reader) []Bolt11RouteHintHop {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Bolt11RouteHintHop, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBolt11RouteHintHopINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBolt11RouteHintHop) Lower(value []Bolt11RouteHintHop) C.RustBuffer {
	return LowerIntoRustBuffer[[]Bolt11RouteHintHop](c, value)
}

func (c FfiConverterSequenceBolt11RouteHintHop) Write(writer io.Writer, value []Bolt11RouteHintHop) {
	if len(value) > math.MaxInt32 {
		panic("[]Bolt11RouteHintHop is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBolt11RouteHintHopINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBolt11RouteHintHop struct{}

func (FfiDestroyerSequenceBolt11RouteHintHop) Destroy(sequence []Bolt11RouteHintHop) {
	for _, value := range sequence {
		FfiDestroyerBolt11RouteHintHop{}.Destroy(value)
	}
}

type FfiConverterSequenceBolt12OfferBlindedPath struct{}

var FfiConverterSequenceBolt12OfferBlindedPathINSTANCE = FfiConverterSequenceBolt12OfferBlindedPath{}

func (c FfiConverterSequenceBolt12OfferBlindedPath) Lift(rb RustBufferI) []Bolt12OfferBlindedPath {
	return LiftFromRustBuffer[[]Bolt12OfferBlindedPath](c, rb)
}

func (c FfiConverterSequenceBolt12OfferBlindedPath) Read(reader io.Reader) []Bolt12OfferBlindedPath {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Bolt12OfferBlindedPath, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBolt12OfferBlindedPathINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBolt12OfferBlindedPath) Lower(value []Bolt12OfferBlindedPath) C.RustBuffer {
	return LowerIntoRustBuffer[[]Bolt12OfferBlindedPath](c, value)
}

func (c FfiConverterSequenceBolt12OfferBlindedPath) Write(writer io.Writer, value []Bolt12OfferBlindedPath) {
	if len(value) > math.MaxInt32 {
		panic("[]Bolt12OfferBlindedPath is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBolt12OfferBlindedPathINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBolt12OfferBlindedPath struct{}

func (FfiDestroyerSequenceBolt12OfferBlindedPath) Destroy(sequence []Bolt12OfferBlindedPath) {
	for _, value := range sequence {
		FfiDestroyerBolt12OfferBlindedPath{}.Destroy(value)
	}
}

type FfiConverterSequenceLocaleOverrides struct{}

var FfiConverterSequenceLocaleOverridesINSTANCE = FfiConverterSequenceLocaleOverrides{}

func (c FfiConverterSequenceLocaleOverrides) Lift(rb RustBufferI) []LocaleOverrides {
	return LiftFromRustBuffer[[]LocaleOverrides](c, rb)
}

func (c FfiConverterSequenceLocaleOverrides) Read(reader io.Reader) []LocaleOverrides {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]LocaleOverrides, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterLocaleOverridesINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceLocaleOverrides) Lower(value []LocaleOverrides) C.RustBuffer {
	return LowerIntoRustBuffer[[]LocaleOverrides](c, value)
}

func (c FfiConverterSequenceLocaleOverrides) Write(writer io.Writer, value []LocaleOverrides) {
	if len(value) > math.MaxInt32 {
		panic("[]LocaleOverrides is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterLocaleOverridesINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceLocaleOverrides struct{}

func (FfiDestroyerSequenceLocaleOverrides) Destroy(sequence []LocaleOverrides) {
	for _, value := range sequence {
		FfiDestroyerLocaleOverrides{}.Destroy(value)
	}
}

type FfiConverterSequenceLocalizedName struct{}

var FfiConverterSequenceLocalizedNameINSTANCE = FfiConverterSequenceLocalizedName{}

func (c FfiConverterSequenceLocalizedName) Lift(rb RustBufferI) []LocalizedName {
	return LiftFromRustBuffer[[]LocalizedName](c, rb)
}

func (c FfiConverterSequenceLocalizedName) Read(reader io.Reader) []LocalizedName {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]LocalizedName, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterLocalizedNameINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceLocalizedName) Lower(value []LocalizedName) C.RustBuffer {
	return LowerIntoRustBuffer[[]LocalizedName](c, value)
}

func (c FfiConverterSequenceLocalizedName) Write(writer io.Writer, value []LocalizedName) {
	if len(value) > math.MaxInt32 {
		panic("[]LocalizedName is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterLocalizedNameINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceLocalizedName struct{}

func (FfiDestroyerSequenceLocalizedName) Destroy(sequence []LocalizedName) {
	for _, value := range sequence {
		FfiDestroyerLocalizedName{}.Destroy(value)
	}
}

type FfiConverterSequenceInputType struct{}

var FfiConverterSequenceInputTypeINSTANCE = FfiConverterSequenceInputType{}

func (c FfiConverterSequenceInputType) Lift(rb RustBufferI) []InputType {
	return LiftFromRustBuffer[[]InputType](c, rb)
}

func (c FfiConverterSequenceInputType) Read(reader io.Reader) []InputType {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]InputType, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterInputTypeINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceInputType) Lower(value []InputType) C.RustBuffer {
	return LowerIntoRustBuffer[[]InputType](c, value)
}

func (c FfiConverterSequenceInputType) Write(writer io.Writer, value []InputType) {
	if len(value) > math.MaxInt32 {
		panic("[]InputType is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterInputTypeINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceInputType struct{}

func (FfiDestroyerSequenceInputType) Destroy(sequence []InputType) {
	for _, value := range sequence {
		FfiDestroyerInputType{}.Destroy(value)
	}
}

type FfiConverterMapStringString struct{}

var FfiConverterMapStringStringINSTANCE = FfiConverterMapStringString{}

func (c FfiConverterMapStringString) Lift(rb RustBufferI) map[string]string {
	return LiftFromRustBuffer[map[string]string](c, rb)
}

func (_ FfiConverterMapStringString) Read(reader io.Reader) map[string]string {
	result := make(map[string]string)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterStringINSTANCE.Read(reader)
		value := FfiConverterStringINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapStringString) Lower(value map[string]string) C.RustBuffer {
	return LowerIntoRustBuffer[map[string]string](c, value)
}

func (_ FfiConverterMapStringString) Write(writer io.Writer, mapValue map[string]string) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[string]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterStringINSTANCE.Write(writer, key)
		FfiConverterStringINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapStringString struct{}

func (_ FfiDestroyerMapStringString) Destroy(mapValue map[string]string) {
	for key, value := range mapValue {
		FfiDestroyerString{}.Destroy(key)
		FfiDestroyerString{}.Destroy(value)
	}
}

const (
	uniffiRustFuturePollReady      int8 = 0
	uniffiRustFuturePollMaybeReady int8 = 1
)

type rustFuturePollFunc func(C.uint64_t, C.UniffiRustFutureContinuationCallback, C.uint64_t)
type rustFutureCompleteFunc[T any] func(C.uint64_t, *C.RustCallStatus) T
type rustFutureFreeFunc func(C.uint64_t)

//export breez_sdk_common_uniffiFutureContinuationCallback
func breez_sdk_common_uniffiFutureContinuationCallback(data C.uint64_t, pollResult C.int8_t) {
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
			(C.UniffiRustFutureContinuationCallback)(C.breez_sdk_common_uniffiFutureContinuationCallback),
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

//export breez_sdk_common_uniffiFreeGorutine
func breez_sdk_common_uniffiFreeGorutine(data C.uint64_t) {
	handle := cgo.Handle(uintptr(data))
	defer handle.Delete()

	guard := handle.Value().(chan struct{})
	guard <- struct{}{}
}
