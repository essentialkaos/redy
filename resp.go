package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Different RespTypes. You can check if a message is of one or more types using
// the IsType method on Resp
const (
	SimpleStr RespType = 1 << iota
	BulkStr
	Int
	Array
	Nil

	IOErr
	RedisErr

	Str = SimpleStr | BulkStr
	Err = IOErr | RedisErr
)

// ////////////////////////////////////////////////////////////////////////////////// //

// RespType is a field on every Resp which indicates the type of the data it
// contains
type RespType uint8

// Resp represents a single response or message being sent to/from a redis
// server. Each Resp has a type (see RespType and IsType) and a value. Values
// can be retrieved using any of the casting methods on this type (e.g. Str)
type Resp struct {
	Err error

	typ RespType
	val interface{}
}

// RespReader is a wrapper around an io.Reader which will read Resp messages off
// of the io.Reader
type RespReader struct {
	r *bufio.Reader
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Errors
var (
	ErrBadType  = errors.New("Wrong type")
	ErrParse    = errors.New("Parse error")
	ErrNotStr   = errors.New("Couldn't convert response to string")
	ErrNotInt   = errors.New("Couldn't convert response to int")
	ErrNotArray = errors.New("Couldn't convert response to array")
	ErrNotMap   = errors.New("Couldn't convert response to map (reply has odd number of elements)")
	ErrRespNil  = errors.New("Response is nil")
)

// ////////////////////////////////////////////////////////////////////////////////// //

var (
	delim    = []byte{'\r', '\n'}
	delimEnd = byte('\n')
)

var (
	prefixStr    = []byte{'+'}
	prefixErr    = []byte{'-'}
	prefixInt    = []byte{':'}
	prefixBulk   = []byte{'$'}
	prefixArray  = []byte{'*'}
	nilFormatted = []byte("$-1\r\n")
)

var typeOfBytes = reflect.TypeOf([]byte(nil))

// ////////////////////////////////////////////////////////////////////////////////// //

// NewRespReader creates and returns a new RespReader which will read from the
// given io.Reader
func NewRespReader(r io.Reader) *RespReader {
	br, ok := r.(*bufio.Reader)

	if !ok {
		br = bufio.NewReader(r)
	}

	return &RespReader{br}
}

// Read attempts to read a message object from the given io.Reader, parse
// it, and return a Resp representing it
func (r *RespReader) Read() *Resp {
	resp, err := bufioReadResp(r.r)

	if err != nil {
		resp = errToResp(IOErr, err)
	}

	return &resp
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Bytes returns a byte slice representing the value of the Resp. Only valid for
// a Resp of type Str
func (r *Resp) Bytes() ([]byte, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	if r.IsType(Nil) {
		return nil, ErrRespNil
	} else if !r.IsType(Str) {
		return nil, ErrBadType
	}

	b, ok := r.val.([]byte)

	if ok {
		return b, nil
	}

	return nil, ErrNotStr
}

// Str is a wrapper around Bytes which returns the result as a string instead of
// a byte slice
func (r *Resp) Str() (string, error) {
	b, err := r.Bytes()

	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Int returns an int representing the value of the Resp
func (r *Resp) Int() (int, error) {
	i, err := r.Int64()
	return int(i), err
}

// Int64 returns an int64 representing the value of the Resp
func (r *Resp) Int64() (int64, error) {
	switch {
	case r.Err != nil:
		return 0, r.Err
	case r.IsType(Nil):
		return 0, ErrRespNil
	}

	i, ok := r.val.(int64)

	if ok {
		return i, nil
	}

	s, err := r.Str()

	if err != nil {
		return 0, err
	}

	i, err = strconv.ParseInt(s, 10, 64)

	if err != nil {
		return 0, err
	}

	return i, nil
}

// Float64 returns a float64 representing the value of the Resp
func (r *Resp) Float64() (float64, error) {
	if r.Err != nil {
		return 0.0, r.Err
	}

	b, ok := r.val.([]byte)

	if !ok {
		return 0.0, ErrNotStr
	}

	f, err := strconv.ParseFloat(string(b), 64)

	if err != nil {
		return 0.0, err
	}

	return f, nil
}

// Array returns the Resp slice encompassed by this Resp. Only valid for a Resp
// of type Array
func (r *Resp) Array() ([]*Resp, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	a, ok := r.val.([]Resp)

	if !ok {
		return nil, ErrNotArray
	}

	ac := make([]*Resp, len(a))

	for i := range a {
		ac[i] = &a[i]
	}

	return ac, nil
}

// List is a wrapper around Array which returns the result as a list of strings,
// calling Str() on each Resp which Array returns. Any errors encountered are
// immediately returned. Any Nil replies are interpreted as empty strings
func (r *Resp) List() ([]string, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	a, ok := r.val.([]Resp)

	if !ok {
		return nil, ErrNotArray
	}

	list := make([]string, len(a))

	for i := range a {
		if a[i].IsType(Nil) {
			list[i] = ""
			continue
		}

		s, err := a[i].Str()

		if err != nil {
			return nil, err
		}

		list[i] = s
	}

	return list, nil
}

// ListBytes is a wrapper around Array which returns the result as a list of
// byte slices, calling Bytes() on each Resp which Array returns. Any errors
// encountered are immediately returned. Any Nil replies are interpreted as nil
func (r *Resp) ListBytes() ([][]byte, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	a, ok := r.val.([]Resp)

	if !ok {
		return nil, ErrNotArray
	}

	list := make([][]byte, len(a))

	for i := range a {
		if a[i].IsType(Nil) {
			list[i] = nil
			continue
		}

		b, err := a[i].Bytes()

		if err != nil {
			return nil, err
		}

		list[i] = b
	}

	return list, nil
}

// Map is a wrapper around Array which returns the result as a map of strings,
// calling Str() on alternating key/values for the map. All value fields of type
// Nil will be treated as empty strings, keys must all be of type Str
func (r *Resp) Map() (map[string]string, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	a, ok := r.val.([]Resp)

	if !ok {
		return nil, ErrNotArray
	}

	if len(a)%2 != 0 {
		return nil, ErrNotMap
	}

	m := make(map[string]string)

	for {
		if len(a) == 0 {
			return m, nil
		}

		k, v := a[0], a[1]
		a = a[2:]

		ks, err := k.Str()

		if err != nil {
			return nil, err
		}

		if v.IsType(Nil) {
			m[ks] = ""
			continue
		}

		vs, err := v.Str()

		if err != nil {
			return nil, err
		}

		m[ks] = vs
	}
}

// String returns a string representation of the Resp. This method is for
// debugging, use Str() for reading a Str reply
func (r *Resp) String() string {
	switch r.typ {
	case RedisErr:
		return fmt.Sprintf("Resp(RedisErr %s)", r.Err)

	case IOErr:
		return fmt.Sprintf("Resp(IOErr %s)", r.Err)

	case BulkStr, SimpleStr:
		return fmt.Sprintf("Resp(Str %q)", string(r.val.([]byte)))

	case Int:
		return fmt.Sprintf("Resp(Int %d)", r.val.(int64))

	case Nil:
		return "Resp(Nil)"

	case Array:
		return arrayToString(r)

	default:
		return "Resp(Unknown)"
	}
}

// IsType returns whether or or not the reply is of a given type
func (r *Resp) IsType(t RespType) bool {
	return r.typ&t > 0
}

// ////////////////////////////////////////////////////////////////////////////////// //

func bufioReadResp(r *bufio.Reader) (Resp, error) {
	b, err := r.Peek(1)

	if err != nil {
		return Resp{}, err
	}

	switch b[0] {
	case prefixStr[0]:
		return readSimpleStr(r)

	case prefixErr[0]:
		return readError(r)

	case prefixInt[0]:
		return readInt(r)

	case prefixBulk[0]:
		return readBulkStr(r)

	case prefixArray[0]:
		return readArray(r)

	default:
		return Resp{}, ErrBadType
	}
}

func readSimpleStr(r *bufio.Reader) (Resp, error) {
	b, err := r.ReadBytes(delimEnd)

	if err != nil {
		return Resp{}, err
	}

	return Resp{nil, SimpleStr, b[1 : len(b)-2]}, nil
}

func readError(r *bufio.Reader) (Resp, error) {
	b, err := r.ReadBytes(delimEnd)

	if err != nil {
		return Resp{}, err
	}

	err = errors.New(string(b[1 : len(b)-2]))

	return errToResp(RedisErr, err), nil
}

func readInt(r *bufio.Reader) (Resp, error) {
	b, err := r.ReadBytes(delimEnd)

	if err != nil {
		return Resp{}, err
	}

	i, err := strconv.ParseInt(string(b[1:len(b)-2]), 10, 64)

	if err != nil {
		return Resp{}, ErrParse
	}

	return Resp{nil, Int, i}, nil
}

func readBulkStr(r *bufio.Reader) (Resp, error) {
	b, err := r.ReadBytes(delimEnd)

	if err != nil {
		return Resp{}, err
	}

	size, err := strconv.ParseInt(string(b[1:len(b)-2]), 10, 64)

	switch {
	case err != nil:
		return Resp{}, ErrParse
	case size < 0:
		return Resp{nil, Nil, nil}, nil
	}

	data := make([]byte, size)
	b2 := data

	var n int

	for len(b2) > 0 {
		n, err = r.Read(b2)

		if err != nil {
			return Resp{}, err
		}

		b2 = b2[n:]
	}

	// There's a hanging \r\n there, gotta read past it
	trail := make([]byte, 2)

	for i := 0; i < 2; i++ {
		c, err := r.ReadByte()

		if err != nil {
			return Resp{}, err
		}

		trail[i] = c
	}

	return Resp{typ: BulkStr, val: data}, nil
}

func readArray(r *bufio.Reader) (Resp, error) {
	b, err := r.ReadBytes(delimEnd)

	if err != nil {
		return Resp{}, err
	}

	size, err := strconv.ParseInt(string(b[1:len(b)-2]), 10, 64)

	switch {
	case err != nil:
		return Resp{}, ErrParse
	case size < 0:
		return Resp{nil, Nil, nil}, nil
	}

	data := make([]Resp, size)

	for i := range data {
		m, err := bufioReadResp(r)

		if err != nil {
			return Resp{}, err
		}

		data[i] = m
	}

	return Resp{typ: Array, val: data}, nil
}

func flatten(m interface{}) []interface{} {
	t := reflect.TypeOf(m)

	if t == typeOfBytes {
		return []interface{}{m}
	}

	switch t.Kind() {
	case reflect.Slice:
		return flattenSlice(m)

	case reflect.Map:
		return flattenMap(m)

	default:
		return []interface{}{m}
	}
}

func flattenSlice(m interface{}) []interface{} {
	rm := reflect.ValueOf(m)
	l := rm.Len()
	ret := make([]interface{}, 0, l)

	for i := 0; i < l; i++ {
		ret = append(ret, flatten(rm.Index(i).Interface())...)
	}

	return ret
}

func flattenedLength(mm ...interface{}) int {
	var total int

	for _, m := range mm {
		switch m.(type) {
		case []byte, string, bool, nil, int, int8, int16, int32, int64, uint,
			uint8, uint16, uint32, uint64, float32, float64, error:
			total++

		case Resp:
			total += flattenedLength(m.(Resp).val)

		case *Resp:
			total += flattenedLength(m.(*Resp).val)

		default:
			t := reflect.TypeOf(m)

			switch t.Kind() {
			case reflect.Slice:
				total += flattenedSliceLength(m)

			case reflect.Map:
				total += flattenedMapLength(m)

			default:
				total++
			}
		}
	}

	return total
}

func flattenedSliceLength(m interface{}) int {
	var total int

	rm := reflect.ValueOf(m)
	l := rm.Len()

	for i := 0; i < l; i++ {
		total += flattenedLength(rm.Index(i).Interface())
	}

	return total
}

func flattenedMapLength(m interface{}) int {
	var total int

	rm := reflect.ValueOf(m)
	keys := rm.MapKeys()

	for _, k := range keys {
		kv := k.Interface()
		vv := rm.MapIndex(k).Interface()
		total += flattenedLength(kv)
		total += flattenedLength(vv)
	}

	return total
}

func flattenMap(m interface{}) []interface{} {
	rm := reflect.ValueOf(m)
	l := rm.Len() * 2
	keys := rm.MapKeys()
	ret := make([]interface{}, 0, l)

	for _, k := range keys {
		kv := k.Interface()
		vv := rm.MapIndex(k).Interface()
		ret = append(ret, flatten(kv)...)
		ret = append(ret, flatten(vv)...)
	}

	return ret
}

func writeTo(w io.Writer, buf []byte, m interface{}, forceString, noArrayHeader bool) (int, error) {
	switch mt := m.(type) {
	case []byte:
		return writeBytes(w, buf, mt)

	case string:
		return writeStr(w, buf, mt)

	case bool:
		return writeBool(w, buf, mt)

	case nil:
		return writeNil(w, buf, forceString)

	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return writeInt(w, buf, forceString, intv(mt))

	case float32:
		return writeFloat(w, buf, float64(mt))

	case float64:
		return writeFloat(w, buf, mt)

	case error:
		return writeError(w, buf, forceString, mt)

	case *Resp:
		return writeTo(w, buf, mt.val, forceString, noArrayHeader)

	case Resp:
		return writeTo(w, buf, mt.val, forceString, noArrayHeader)

	case []interface{}:
		return writeInterface(w, buf, mt, forceString, noArrayHeader)

	default:
		switch reflect.TypeOf(m).Kind() {
		case reflect.Slice:
			return writeSlice(w, buf, mt, forceString, noArrayHeader)

		case reflect.Map:
			return writeMap(w, buf, mt, forceString, noArrayHeader)
		}
	}

	return writeBytes(w, buf, []byte(fmt.Sprint(m)))
}

func writeBytesHelper(w io.Writer, b []byte, lastWritten int, lastErr error) (int, error) {
	if lastErr != nil {
		return lastWritten, lastErr
	}

	i, err := w.Write(b)

	return i + lastWritten, err
}

func writeArrayHeader(w io.Writer, buf []byte, l int) (int, error) {
	buf = strconv.AppendInt(buf, int64(l), 10)

	var err error
	var written int

	written, err = writeBytesHelper(w, prefixArray, written, err)
	written, err = writeBytesHelper(w, buf, written, err)
	written, err = writeBytesHelper(w, delim, written, err)

	return written, err
}

func writeBytes(w io.Writer, buf, b []byte) (int, error) {
	var err error
	var written int

	buf = strconv.AppendInt(buf[:0], int64(len(b)), 10)

	written, err = writeBytesHelper(w, prefixBulk, written, err)
	written, err = writeBytesHelper(w, buf, written, err)
	written, err = writeBytesHelper(w, delim, written, err)
	written, err = writeBytesHelper(w, b, written, err)
	written, err = writeBytesHelper(w, delim, written, err)

	return written, err
}

func writeStr(w io.Writer, buf []byte, s string) (int, error) {
	sbuf := append(buf[:0], s...)
	buf = sbuf[len(sbuf):]

	return writeBytes(w, buf, sbuf)
}

func writeBool(w io.Writer, buf []byte, b bool) (int, error) {
	buf = buf[:0]

	switch b {
	case true:
		buf = append(buf, '1')
	default:
		buf = append(buf, '0')
	}

	return writeBytes(w, buf[1:], buf[:1])
}

func writeNil(w io.Writer, buf []byte, forceString bool) (int, error) {
	if forceString {
		return writeBytes(w, buf, nil)
	}

	return w.Write(nilFormatted)
}

func writeInt(w io.Writer, buf []byte, forceString bool, i int) (int, error) {
	buf = strconv.AppendInt(buf[:0], int64(i), 10)

	if forceString {
		return writeBytes(w, buf[len(buf):], buf)
	}

	var err error
	var written int

	written, err = writeBytesHelper(w, prefixInt, written, err)
	written, err = writeBytesHelper(w, buf, written, err)
	written, err = writeBytesHelper(w, delim, written, err)

	return written, err
}

func writeFloat(w io.Writer, buf []byte, f float64) (int, error) {
	buf = strconv.AppendFloat(buf[:0], f, 'f', -1, 64)
	return writeBytes(w, buf[len(buf):], buf)
}

func writeError(w io.Writer, buf []byte, forceString bool, e error) (int, error) {
	errData := []byte(e.Error())

	if forceString {
		return writeBytes(w, buf, errData)
	}

	var err error
	var written int

	written, err = writeBytesHelper(w, prefixErr, written, err)
	written, err = writeBytesHelper(w, errData, written, err)
	written, err = writeBytesHelper(w, delim, written, err)

	return written, err
}

func writeInterface(w io.Writer, buf []byte, mt []interface{}, forceString, noArrayHeader bool) (int, error) {
	var totalWritten int

	l := len(mt)

	if !noArrayHeader {
		written, err := writeArrayHeader(w, buf, l)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}
	}

	for i := 0; i < l; i++ {
		written, err := writeTo(w, buf, mt[i], forceString, noArrayHeader)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}
	}

	return totalWritten, nil
}

func writeSlice(w io.Writer, buf []byte, mt interface{}, forceString, noArrayHeader bool) (int, error) {
	rm := reflect.ValueOf(mt)
	l := rm.Len()

	var err error
	var totalWritten, written int

	if !noArrayHeader {
		written, err = writeArrayHeader(w, buf, l)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}
	}

	for i := 0; i < l; i++ {
		vv := rm.Index(i).Interface()

		written, err = writeTo(w, buf, vv, forceString, noArrayHeader)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}
	}

	return totalWritten, nil
}

func writeMap(w io.Writer, buf []byte, mt interface{}, forceString, noArrayHeader bool) (int, error) {
	rm := reflect.ValueOf(mt)
	l := rm.Len() * 2

	var err error
	var totalWritten, written int

	if !noArrayHeader {
		written, err = writeArrayHeader(w, buf, l)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}
	}

	for _, k := range rm.MapKeys() {
		kv := k.Interface()

		written, err = writeTo(w, buf, kv, forceString, noArrayHeader)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}

		vv := rm.MapIndex(k).Interface()
		written, err = writeTo(w, buf, vv, forceString, noArrayHeader)
		totalWritten += written

		if err != nil {
			return totalWritten, err
		}
	}

	return totalWritten, nil
}

func errToResp(t RespType, err error) Resp {
	return Resp{err, t, err}
}

func arrayToString(resp *Resp) string {
	var kids string

	for _, r := range resp.val.([]Resp) {
		kids = " " + r.String()
	}

	if kids == "" {
		return "Resp(Empty Array)"
	}

	return fmt.Sprintf("Resp(%s)", kids[1:])
}

func isTimeout(resp *Resp) bool {
	if resp.IsType(IOErr) {
		t, ok := resp.Err.(*net.OpError)
		return ok && t.Timeout()
	}

	return false
}

func intv(v interface{}) int {
	switch vt := v.(type) {
	case int:
		return vt
	case int8:
		return int(vt)
	case int16:
		return int(vt)
	case int32:
		return int(vt)
	case int64:
		return int(vt)
	case uint:
		return int(vt)
	case uint8:
		return int(vt)
	case uint16:
		return int(vt)
	case uint32:
		return int(vt)
	case uint64:
		return int(vt)
	default:
		return -1
	}
}