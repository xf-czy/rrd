package rrd

/*
#include <stdlib.h>
#include <rrd.h>
#include "rrdfunc.h"
*/
import "C"
import (
	"reflect"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

type cstring C.char

func newCstring(s string) *cstring {
	cs := C.malloc(C.size_t(len(s) + 1))
	buf := (*[1<<31 - 1]byte)(cs)[:len(s)+1]
	copy(buf, s)
	buf[len(s)] = 0
	return (*cstring)(cs)
}

func (cs *cstring) Free() {
	if cs != nil {
		C.free(unsafe.Pointer(cs))
	}
}

func (cs *cstring) String() string {
	buf := (*[1<<31 - 1]byte)(unsafe.Pointer(cs))
	for n, b := range buf {
		if b == 0 {
			return string(buf[:n])
		}
	}
	panic("rrd: bad C string")
}

var mutex sync.Mutex

func makeArgs(args []string) []*C.char {
	ret := make([]*C.char, len(args))
	for i, s := range args {
		ret[i] = C.CString(s)
	}
	return ret
}

func freeCString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

func freeArgs(cArgs []*C.char) {
	for _, s := range cArgs {
		freeCString(s)
	}
}

func makeError(e *C.char) error {
	var null *C.char
	if e == null {
		return nil
	}
	defer freeCString(e)
	return Error(C.GoString(e))
}

func (c *Creator) create() error {
	filename := C.CString(c.filename)
	defer freeCString(filename)
	args := makeArgs(c.args)
	defer freeArgs(args)

	e := C.rrdCreate(
		filename,
		C.ulong(c.step),
		C.time_t(c.start.Unix()),
		C.int(len(args)),
		&args[0],
	)
	return makeError(e)
}

func (u *Updater) update(args []*cstring) error {
	e := C.rrdUpdate(
		(*C.char)(u.filename),
		(*C.char)(u.template),
		C.int(len(args)),
		(**C.char)(unsafe.Pointer(&args[0])),
	)
	return makeError(e)
}

var (
	graphv = C.CString("graphv")
	xport  = C.CString("xport")

	oStart           = C.CString("-s")
	oEnd             = C.CString("-e")
	oTitle           = C.CString("-t")
	oVlabel          = C.CString("-v")
	oWidth           = C.CString("-w")
	oHeight          = C.CString("-h")
	oUpperLimit      = C.CString("-u")
	oLowerLimit      = C.CString("-l")
	oRigid           = C.CString("-r")
	oAltAutoscale    = C.CString("-A")
	oAltAutoscaleMin = C.CString("-J")
	oAltAutoscaleMax = C.CString("-M")
	oNoGridFit       = C.CString("-N")

	oLogarithmic   = C.CString("-o")
	oUnitsExponent = C.CString("-X")
	oUnitsLength   = C.CString("-L")

	oRightAxis      = C.CString("--right-axis")
	oRightAxisLabel = C.CString("--right-axis-label")

	oDaemon = C.CString("--daemon")

	oBorder = C.CString("--border")

	oNoLegend = C.CString("-g")

	oLazy = C.CString("-z")

	oColor = C.CString("-c")

	oSlopeMode   = C.CString("-E")
	oImageFormat = C.CString("-a")
	oInterlaced  = C.CString("-i")

	oBase      = C.CString("-b")
	oWatermark = C.CString("-W")

	oStep    = C.CString("--step")
	oMaxRows = C.CString("-m")
)

func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'e', 10, 64)
}

func ftoc(f float64) *C.char {
	return C.CString(ftoa(f))
}

func i64toa(i int64) string {
	return strconv.FormatInt(i, 10)
}

func i64toc(i int64) *C.char {
	return C.CString(i64toa(i))
}

func u64toa(u uint64) string {
	return strconv.FormatUint(u, 10)
}

func u64toc(u uint64) *C.char {
	return C.CString(u64toa(u))
}

func utoc(u uint) *C.char {
	return u64toc(uint64(u))
}

func (e *Exporter) makeArgs(start, end time.Time, step time.Duration) []*C.char {
	args := []*C.char{
		xport,
		oStart, i64toc(start.Unix()),
		oEnd, i64toc(end.Unix()),
		oStep, i64toc(int64(step.Seconds())),
	}
	if e.maxRows != 0 {
		args = append(args, oMaxRows, utoc(e.maxRows))
	}
	if e.daemon != "" {
		args = append(args, oDaemon, C.CString(e.daemon))
	}
	return append(args, makeArgs(e.args)...)
}

// Fetch retrieves data from RRD file.
func Fetch(filename, cf string, start, end time.Time, step time.Duration) (FetchResult, error) {
	fn := C.CString(filename)
	defer freeCString(fn)
	cCf := C.CString(cf)
	defer freeCString(cCf)
	cStart := C.time_t(start.Unix())
	cEnd := C.time_t(end.Unix())
	cStep := C.ulong(step.Seconds())
	var (
		ret      C.int
		cDsCnt   C.ulong
		cDsNames **C.char
		cData    *C.double
	)
	err := makeError(C.rrdFetch(&ret, fn, cCf, &cStart, &cEnd, &cStep, &cDsCnt, &cDsNames, &cData))
	if err != nil {
		return FetchResult{filename, cf, start, end, step, nil, 0, nil}, err
	}

	start = time.Unix(int64(cStart), 0)
	end = time.Unix(int64(cEnd), 0)
	step = time.Duration(cStep) * time.Second
	dsCnt := int(cDsCnt)

	dsNames := make([]string, dsCnt)
	for i := 0; i < dsCnt; i++ {
		dsName := C.arrayGetCString(cDsNames, C.int(i))
		dsNames[i] = C.GoString(dsName)
		C.free(unsafe.Pointer(dsName))
	}
	C.free(unsafe.Pointer(cDsNames))

	rowCnt := (int(cEnd)-int(cStart))/int(cStep) + 1
	valuesLen := dsCnt * rowCnt
	var values []float64
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&values)))
	sliceHeader.Cap = valuesLen
	sliceHeader.Len = valuesLen
	sliceHeader.Data = uintptr(unsafe.Pointer(cData))
	return FetchResult{filename, cf, start, end, step, dsNames, rowCnt, values}, nil
}

// FreeValues free values memory allocated by C.
func (r *FetchResult) FreeValues() {
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&r.values)))
	C.free(unsafe.Pointer(sliceHeader.Data))
}

// Values returns copy of internal array of values.
func (r *FetchResult) Values() []float64 {
	return append([]float64{}, r.values...)
}

// Export data from RRD file(s)
func (e *Exporter) xport(start, end time.Time, step time.Duration) (XportResult, error) {
	cStart := C.time_t(start.Unix())
	cEnd := C.time_t(end.Unix())
	cStep := C.ulong(step.Seconds())
	args := e.makeArgs(start, end, step)

	mutex.Lock()
	defer mutex.Unlock()

	var (
		ret      C.int
		cXSize   C.int
		cColCnt  C.ulong
		cLegends **C.char
		cData    *C.double
	)
	err := makeError(C.rrdXport(
		&ret,
		C.int(len(args)),
		&args[0],
		&cXSize, &cStart, &cEnd, &cStep, &cColCnt, &cLegends, &cData,
	))
	if err != nil {
		return XportResult{start, end, step, nil, 0, nil}, err
	}

	start = time.Unix(int64(cStart), 0)
	end = time.Unix(int64(cEnd), 0)
	step = time.Duration(cStep) * time.Second
	colCnt := int(cColCnt)

	legends := make([]string, colCnt)
	for i := 0; i < colCnt; i++ {
		legend := C.arrayGetCString(cLegends, C.int(i))
		legends[i] = C.GoString(legend)
		C.free(unsafe.Pointer(legend))
	}
	C.free(unsafe.Pointer(cLegends))

	rowCnt := (int(cEnd) - int(cStart)) / int(cStep) //+ 1 // FIXED: + 1 added extra uninitialized value
	valuesLen := colCnt * rowCnt
	values := make([]float64, valuesLen)
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&values)))
	sliceHeader.Cap = valuesLen
	sliceHeader.Len = valuesLen
	sliceHeader.Data = uintptr(unsafe.Pointer(cData))
	return XportResult{start, end, step, legends, rowCnt, values}, nil
}

// FreeValues free values memory allocated by C.
func (r *XportResult) FreeValues() {
	sliceHeader := (*reflect.SliceHeader)((unsafe.Pointer(&r.values)))
	C.free(unsafe.Pointer(sliceHeader.Data))
}
