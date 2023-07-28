package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"math/rand"
	"net"
	"os"
	"sort"
	"testing"
	"time"

	. "github.com/essentialkaos/check"
)

// ////////////////////////////////////////////////////////////////////////////////// //

type timeoutError struct{}

type errReader struct{}

// ////////////////////////////////////////////////////////////////////////////////// //

func Test(t *testing.T) { TestingT(t) }

type RedySuite struct {
	c *Client
}

// ////////////////////////////////////////////////////////////////////////////////// //

var _ = Suite(&RedySuite{})

// ////////////////////////////////////////////////////////////////////////////////// //

func (rs *RedySuite) SetUpSuite(c *C) {
	redisIP, ok := os.LookupEnv("REDIS_IP")

	if !ok {
		redisIP = "127.0.0.1"
	}

	redisPort, ok := os.LookupEnv("REDIS_PORT")

	if !ok {
		redisPort = "6379"
	}

	rs.c = &Client{
		Network:      "tcp",
		Addr:         redisIP + ":" + redisPort,
		DialTimeout:  time.Second * 15,
		ReadTimeout:  time.Second * 3,
		WriteTimeout: time.Second * 3,
	}

	err := rs.c.Connect()

	if err != nil {
		c.Fatalf("Fatal error: %v", err)
	}
}

func (rs *RedySuite) TestConnectionError(c *C) {
	rc := &Client{
		Network:   "tcp",
		Addr:      "127.0.0.255:60000",
		TLSConfig: &tls.Config{},
	}

	resp := rc.Cmd("PING")
	c.Assert(resp.Err, NotNil)

	resp = rc.PipeResp()
	c.Assert(resp.Err, NotNil)

	err := rc.Connect()
	c.Assert(err, NotNil)
}

func (rs *RedySuite) TestCmd(c *C) {
	r := rs.c.Cmd("ECHO", "TEST1234")
	respStr, err := r.Str()
	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	c.Assert(r.HasType(STR), Equals, true)
	c.Assert(err, IsNil)
	c.Assert(respStr, Equals, "TEST1234")

	r = rs.c.Cmd("UNKNOWN_COMMAND")
	c.Assert(r, NotNil)
	c.Assert(r.Err, NotNil)
	c.Assert(r.HasType(ERR_REDIS), Equals, true)

	key, val := randString(12), randString(64)
	r = rs.c.Cmd("SADD", key, val)
	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	r = rs.c.Cmd("GET", key)
	c.Assert(r, NotNil)
	c.Assert(r.Err, NotNil)
	respStr, err = r.Str()
	c.Assert(respStr, Equals, "")
	c.Assert(err, NotNil)

	key = randString(12)
	args := map[string]any{
		"someBytes":  []byte("blah"),
		"someString": "foo",
		"someInt":    10,
		"someBool":   false,
	}

	r = rs.c.Cmd("HMSET", key, args)
	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	r = rs.c.Cmd("HMGET", key, "someBytes", "someString", "someInt", "someBool")
	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	respList, err := r.List()
	c.Assert(err, IsNil)
	c.Assert(respList, DeepEquals, []string{"blah", "foo", "10", "0"})
}

func (rs *RedySuite) TestPipeline(c *C) {
	// Do this multiple times to make sure pipeline resetting happens correctly
	for i := 0; i < 3; i++ {
		rs.c.PipeAppend("ECHO", "foo")
		rs.c.PipeAppend("ECHO", "bar")
		rs.c.PipeAppend("ECHO", "zot")

		val, err := rs.c.PipeResp().Str()
		c.Assert(err, IsNil)
		c.Assert(val, Equals, "foo")

		val, err = rs.c.PipeResp().Str()
		c.Assert(err, IsNil)
		c.Assert(val, Equals, "bar")

		val, err = rs.c.PipeResp().Str()
		c.Assert(err, IsNil)
		c.Assert(val, Equals, "zot")

		r := rs.c.PipeResp()
		c.Assert(r, NotNil)
		c.Assert(r.HasType(ERR_REDIS), Equals, true)
		c.Assert(r.Err, Equals, ErrEmptyPipeline)
	}

	rs.c.PipeAppend("ECHO", "foo")
	rs.c.PipeAppend("ECHO", "bar")

	pending, complete := rs.c.PipeClear()
	c.Assert(pending, Equals, 2)
	c.Assert(complete, Equals, 0)

	rs.c.PipeAppend("ECHO", "foo")
	rs.c.PipeAppend("ECHO", "bar")

	val, err := rs.c.PipeResp().Str()
	c.Assert(err, IsNil)
	c.Assert(val, Equals, "foo")

	pending, complete = rs.c.PipeClear()
	c.Assert(pending, Equals, 0)
	c.Assert(complete, Equals, 1)
}

func (rs *RedySuite) TestReconnect(c *C) {
	rs.c.Close()
	err := rs.c.Connect()

	c.Assert(err, IsNil)
}

func (rs *RedySuite) TestLastCritical(c *C) {
	rc := &Client{
		Addr: rs.c.Addr,
	}

	err := rc.Connect()
	c.Assert(err, IsNil)

	err = rc.Cmd("UNKNOWN_COMMAND").Err
	c.Assert(err, NotNil)
	c.Assert(rc.LastCritical, IsNil)

	rc.Close()

	err = rc.Cmd("UNKNOWN_COMMAND").Err
	c.Assert(err, NotNil)
	c.Assert(rc.LastCritical, NotNil)
}

func (rs *RedySuite) TestRespRead(c *C) {
	var r *Resp
	var err error

	r = pretendRead("")
	c.Assert(r.Err, NotNil)

	// Simple string
	r = pretendRead("+TEST1234\r\n")
	c.Assert(r.HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("TEST1234"))
	s, err := r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "TEST1234")
	c.Assert(r.String(), Equals, "Resp(Str \"TEST1234\")")

	// Empty simple string
	r = pretendRead("+\r\n")
	c.Assert(r.HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val, DeepEquals, []byte(""))
	s, err = r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "")
	c.Assert(r.String(), Equals, "Resp(Str \"\")")

	// Error
	r = pretendRead("-TEST1234\r\n")
	c.Assert(r.HasType(ERR_REDIS), Equals, true)
	c.Assert(r.val, DeepEquals, errors.New("TEST1234"))
	c.Assert(r.Err.Error(), Equals, "TEST1234")
	c.Assert(r.String(), Equals, "Resp(RedisErr \"TEST1234\")")

	// Empty error
	r = pretendRead("-\r\n")
	c.Assert(r.HasType(ERR_REDIS), Equals, true)
	c.Assert(r.val, DeepEquals, errors.New(""))
	c.Assert(r.Err.Error(), Equals, "")
	c.Assert(r.String(), Equals, "Resp(RedisErr \"\")")

	// Int
	r = pretendRead(":1024\r\n")
	c.Assert(r.HasType(INT), Equals, true)
	c.Assert(r.val, Equals, int64(1024))
	i, err := r.Int()
	c.Assert(err, IsNil)
	c.Assert(i, Equals, 1024)
	c.Assert(r.String(), Equals, "Resp(Int 1024)")

	// Check int max
	maxInt = 10
	i, err = r.Int()
	c.Assert(err, IsNil)
	c.Assert(i, Equals, 10)
	maxInt = int(^uint(0) >> 1)

	// Int (from string)
	r = pretendRead("+50\r\n")
	c.Assert(r.HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("50"))
	i, err = r.Int()
	c.Assert(err, IsNil)
	c.Assert(i, Equals, 50)
	i64, err := r.Int64()
	c.Assert(err, IsNil)
	c.Assert(i64, Equals, int64(50))
	f, err := r.Float64()
	c.Assert(err, IsNil)
	c.Assert(f, Equals, 50.0)
	c.Assert(r.String(), Equals, "Resp(Str \"50\")")

	// Int (from string, can't parse)
	r = pretendRead("+TEST1234\r\n")
	c.Assert(r.HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("TEST1234"))
	_, err = r.Int()
	c.Assert(err, NotNil)
	c.Assert(r.String(), Equals, "Resp(Str \"TEST1234\")")

	// Bulk string
	r = pretendRead("$8\r\nTEST1234\r\n")
	c.Assert(r.HasType(STR_BULK), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("TEST1234"))
	s, err = r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "TEST1234")
	c.Assert(r.String(), Equals, "Resp(BulkStr \"TEST1234\")")

	// Empty bulk string
	r = pretendRead("$0\r\n\r\n")
	c.Assert(r.HasType(STR_BULK), Equals, true)
	c.Assert(r.val, DeepEquals, []byte(""))
	s, err = r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "")
	c.Assert(r.String(), Equals, "Resp(BulkStr \"\")")

	// Nil bulk string
	r = pretendRead("$-1\r\n")
	c.Assert(r.HasType(NIL), Equals, true)
	c.Assert(r.String(), Equals, "Resp(Nil)")

	// Array
	r = pretendRead("*2\r\n+TEST\r\n+1234\r\n")
	c.Assert(r.HasType(ARRAY), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 2)
	c.Assert(r.val.([]Resp)[0].HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("TEST"))
	c.Assert(r.val.([]Resp)[1].HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("1234"))

	l, err := r.List()
	c.Assert(err, IsNil)
	c.Assert(l, DeepEquals, []string{"TEST", "1234"})

	lb, err := r.ListBytes()
	c.Assert(err, IsNil)
	c.Assert(lb, DeepEquals, [][]byte{[]byte("TEST"), []byte("1234")})

	m, err := r.Map()
	c.Assert(err, IsNil)
	c.Assert(m, DeepEquals, map[string]string{"TEST": "1234"})
	c.Assert(r.String(), Equals, "Resp(0:Resp(Str \"TEST\") 1:Resp(Str \"1234\"))")

	// Empty Array
	r = pretendRead("*0\r\n")
	c.Assert(r.HasType(ARRAY), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 0)
	c.Assert(r.String(), Equals, "Resp(Empty Array)")

	// Nil Array
	r = pretendRead("*-1\r\n")
	c.Assert(r.HasType(NIL), Equals, true)
	c.Assert(r.String(), Equals, "Resp(Nil)")

	// Embedded Array
	r = pretendRead("*3\r\n+TEST\r\n+1234\r\n*2\r\n+STUB\r\n+5678\r\n")
	c.Assert(r.String(), Equals, "Resp(0:Resp(Str \"TEST\") 1:Resp(Str \"1234\") 2:Resp(0:Resp(Str \"STUB\") 1:Resp(Str \"5678\")))")
	c.Assert(r.HasType(ARRAY), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 3)
	c.Assert(r.val.([]Resp)[0].HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("TEST"))
	c.Assert(r.val.([]Resp)[1].HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("1234"))
	r = &r.val.([]Resp)[2]
	c.Assert(r.HasType(ARRAY), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 2)
	c.Assert(r.val.([]Resp)[0].HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("STUB"))
	c.Assert(r.val.([]Resp)[1].HasType(STR_SIMPLE), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("5678"))

	// Test that two bulks in a row read correctly
	r = pretendRead("*2\r\n$4\r\nTEST\r\n$4\r\n1234\r\n")
	c.Assert(r.String(), Equals, "Resp(0:Resp(BulkStr \"TEST\") 1:Resp(BulkStr \"1234\"))")
	c.Assert(r.HasType(ARRAY), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 2)
	c.Assert(r.val.([]Resp)[0].HasType(STR_BULK), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("TEST"))
	c.Assert(r.val.([]Resp)[1].HasType(STR_BULK), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("1234"))

	r = &Resp{}
	c.Assert(r.String(), Equals, "Resp(Unknown)")

	r = &Resp{typ: ERR_IO, Err: errors.New("IOERR")}
	c.Assert(r.String(), Equals, "Resp(ErrIO \"IOERR\")")
}

func (rs *RedySuite) TestReqEncoding(c *C) {
	r := rs.c.Cmd("ECHO", 1)
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", nil)
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", true)
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", float32(1))
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", float64(1))
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", errors.New("TEST"))
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", &Resp{typ: STR_SIMPLE, val: "TEST"})
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", Resp{typ: STR_SIMPLE, val: "TEST"})
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", []any{1})
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", []int{1})
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("ECHO", time.Now())
	c.Assert(r.Err, IsNil)
}

func (rs *RedySuite) TestRespReadErrors(c *C) {
	r := &Resp{typ: NIL}
	_, err := r.Bytes()
	c.Assert(err, NotNil)
	_, err = r.Int64()
	c.Assert(err, NotNil)

	r = &Resp{typ: ARRAY, val: -1}
	_, err = r.Bytes()
	c.Assert(err, NotNil)
	_, err = r.Float64()
	c.Assert(err, NotNil)
	_, err = r.List()
	c.Assert(err, NotNil)
	_, err = r.ListBytes()
	c.Assert(err, NotNil)
	_, err = r.Map()
	c.Assert(err, NotNil)

	r = &Resp{typ: STR_BULK, val: -1}
	_, err = r.Bytes()
	c.Assert(err, NotNil)

	r = &Resp{Err: errors.New("TEST")}
	_, err = r.Bytes()
	c.Assert(err, NotNil)
	_, err = r.Int64()
	c.Assert(err, NotNil)
	_, err = r.Float64()
	c.Assert(err, NotNil)
	_, err = r.Array()
	c.Assert(err, NotNil)
	_, err = r.List()
	c.Assert(err, NotNil)
	_, err = r.ListBytes()
	c.Assert(err, NotNil)
	_, err = r.Map()
	c.Assert(err, NotNil)

	r = &Resp{typ: STR_BULK, val: "abc"}
	_, err = r.Int64()
	c.Assert(err, NotNil)

	r = &Resp{typ: STR_BULK, val: []byte("abc")}
	_, err = r.Float64()
	c.Assert(err, NotNil)

	r = &Resp{typ: ARRAY, val: []Resp{Resp{typ: NIL}, Resp{typ: STR, val: -1}}}
	_, err = r.List()
	c.Assert(err, NotNil)
	_, err = r.ListBytes()
	c.Assert(err, NotNil)

	r = &Resp{typ: ARRAY, val: []Resp{Resp{typ: NIL}}}
	_, err = r.Map()
	c.Assert(err, NotNil)

	r = &Resp{typ: ARRAY, val: []Resp{
		Resp{typ: STR, val: -1},
		Resp{typ: NIL},
	}}

	_, err = r.Map()
	c.Assert(err, NotNil)

	r = &Resp{typ: ARRAY, val: []Resp{
		Resp{typ: STR, val: []byte("abc")},
		Resp{typ: NIL},
		Resp{typ: STR, val: []byte("abcd")},
		Resp{typ: STR, val: -1},
	}}

	_, err = r.Map()
	c.Assert(err, NotNil)
}

func (rs *RedySuite) TestRespReadParseErrors(c *C) {
	rd := bytes.NewBuffer(append(prefixStr, '\n'))
	br := bufio.NewReader(rd)
	_, err := readSimpleStr(br)
	c.Assert(err, NotNil)

	rd = bytes.NewBuffer(append(prefixErr, '\n'))
	br = bufio.NewReader(rd)
	_, err = readError(br)
	c.Assert(err, NotNil)

	rd = bytes.NewBuffer(append(prefixInt, '\n'))
	br = bufio.NewReader(rd)
	_, err = readInt(br)
	c.Assert(err, NotNil)

	rd = bytes.NewBuffer(append(prefixBulk, '\n'))
	br = bufio.NewReader(rd)
	_, err = readBulkStr(br)
	c.Assert(err, NotNil)

	rd = bytes.NewBuffer(append(prefixArray, '\n'))
	br = bufio.NewReader(rd)
	_, err = readArray(br)
	c.Assert(err, NotNil)

	rd = bytes.NewBuffer(append(prefixBulk, []byte("1000000000000000\n")...))
	br = bufio.NewReader(rd)
	_, err = readBulkStr(br)
	c.Assert(err, NotNil)
}

func (rs *RedySuite) TestInfoParser(c *C) {
	r := rs.c.Cmd("INFO")

	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	c.Assert(r.HasType(STR_BULK), Equals, true)

	info, err := ParseInfo(r)

	c.Assert(err, IsNil)
	c.Assert(info, NotNil)

	// Append fake info
	info.Sections["Persistence"].Values["aof_enabled"] = "1"
	info.Sections["Replication"].Values["slave0"] = "ip=123.21.98.33,port=23477,state=online,offset=14177815,lag=351"
	info.Sections["Replication"].Values["replica1"] = "ip=123.21.98.33,port=23477,state=online,offset=14177815,lag=351"

	c.Assert(info.Get("server", "redis_mode"), Equals, "standalone")
	c.Assert(info.Get("server", "unknown1", "unknown2"), Equals, "")
	c.Assert(info.GetI("server", "hz"), Equals, 10)
	c.Assert(info.GetU("server", "hz"), Equals, uint64(10))
	c.Assert(info.GetF("memory", "mem_fragmentation_ratio"), Not(Equals), 0.0)
	c.Assert(info.GetB("replication", "repl_backlog_active"), Equals, false)
	c.Assert(info.GetB("persistence", "aof_enabled"), Equals, true)

	replicaInfo := info.GetReplicaInfo(0)

	c.Assert(replicaInfo, NotNil)
	c.Assert(replicaInfo.IP, Equals, "123.21.98.33")
	c.Assert(replicaInfo.Port, Equals, 23477)
	c.Assert(replicaInfo.State, Equals, "online")
	c.Assert(replicaInfo.Offset, Equals, int64(14177815))
	c.Assert(replicaInfo.Lag, Equals, int64(351))

	replicaInfo = info.GetReplicaInfo(1)
	c.Assert(replicaInfo, NotNil)

	replicaInfo = info.GetReplicaInfo(2)
	c.Assert(replicaInfo, IsNil)

	flatInfo := info.Flatten()

	c.Assert(flatInfo, Not(HasLen), 0)
	c.Assert(flatInfo[0], HasLen, 2)
	c.Assert(flatInfo[0][0], Not(Equals), "")
	c.Assert(flatInfo[0][1], Not(Equals), "")

	// --

	r = &Resp{typ: INT, val: 1}
	_, err = ParseInfo(r)
	c.Assert(err, NotNil)

	// --

	r = &Resp{typ: STR_BULK, val: 1}
	_, err = ParseInfo(r)
	c.Assert(err, NotNil)

	// --

	r = &Resp{typ: STR_BULK, val: []byte("")}
	_, err = ParseInfo(r)
	c.Assert(err, NotNil)

	c.Assert(info.Get("", ""), Equals, "")
	c.Assert(info.Get("abcd", "abcd"), Equals, "")
	c.Assert(info.GetI("", ""), Equals, 0)
	c.Assert(info.GetF("", ""), Equals, 0.0)
	c.Assert(info.GetU("", ""), Equals, uint64(0))

	// --

	r = &Resp{typ: STR_BULK, val: []byte("abcd\nabcd\n#abcd\nabcd")}
	_, err = ParseInfo(r)
	c.Assert(err, IsNil)

	c.Assert(info.Get("", ""), Equals, "")
	c.Assert(info.Get("abcd", "abcd"), Equals, "")
	c.Assert(info.GetI("", ""), Equals, 0)
	c.Assert(info.GetF("", ""), Equals, 0.0)
	c.Assert(info.GetU("", ""), Equals, uint64(0))
}

func (rs *RedySuite) TestConfigParsers(c *C) {
	var cfg *Config

	c.Assert(cfg.Get("abc"), Equals, "")
	c.Assert(cfg.Get(""), Equals, "")

	fileConf, err := ReadConfig(".tests/test.conf")

	c.Assert(err, NotNil)
	c.Assert(fileConf, IsNil)

	fileConf, err = ReadConfig(".tests/full.conf")

	c.Assert(err, IsNil)
	c.Assert(fileConf, NotNil)

	fcKeepalive := fileConf.Get("tcp-keepalive")
	fcAuth := fileConf.Get("masterauth")
	fcSave := fileConf.Get("save")
	fcLimit := fileConf.Get("client-output-buffer-limit")

	c.Assert(fcKeepalive, Equals, "300")
	c.Assert(fcAuth, Equals, "")

	c.Assert(fcSave, Equals, "3600 1 300 100 60 10000")

	c.Assert(fileConf.Has("tcp-keepalive"), Equals, true)
	c.Assert(fileConf.Has("udp-keepalive"), Equals, false)
	c.Assert(fileConf.Has(""), Equals, false)

	memConf, err := rs.c.GetConfig("SET")

	c.Assert(err, NotNil)
	c.Assert(memConf, IsNil)

	memConf, err = rs.c.GetConfig("PING")

	c.Assert(err, NotNil)
	c.Assert(memConf, IsNil)

	memConf, err = rs.c.GetConfig("CONFIG")

	c.Assert(err, IsNil)
	c.Assert(memConf, NotNil)

	c.Assert(memConf.Get("tcp-keepalive"), Equals, fcKeepalive)
	c.Assert(memConf.Get("masterauth"), Equals, fcAuth)
	c.Assert(memConf.Get("save"), Equals, fcSave)
	c.Assert(memConf.Get("client-output-buffer-limit"), Equals, fcLimit)

	resp := &Resp{typ: STR_SIMPLE, val: ""}
	_, err = parseInMemoryConfig(resp)
	c.Assert(err, NotNil)

	resp = &Resp{typ: ARRAY, val: []Resp{Resp{}, Resp{}, Resp{}}}
	_, err = parseInMemoryConfig(resp)
	c.Assert(err, NotNil)
}

func (rs *RedySuite) TestConfigDiff(c *C) {
	var c1 *Config
	var c2 *Config

	c.Assert(len(c1.Diff(c2)), Equals, 0)

	c1 = &Config{
		Props: []string{"a", "b", "c", "d"},
		Data: map[string][]string{
			"a": []string{"1"},
			"b": []string{"2"},
			"c": []string{"3"},
			"d": []string{"4"},
		},
	}

	c2 = &Config{
		Props: []string{"a", "b", "c", "e"},
		Data: map[string][]string{
			"a": []string{"1"},
			"b": []string{"W"},
			"c": []string{"3"},
			"e": []string{"5"},
		},
	}

	diff := c1.Diff(c2)
	sort.Strings(diff)

	c.Assert(len(diff), Equals, 3)
	c.Assert(diff, DeepEquals, []string{"b", "d", "e"})
}

func (rs *RedySuite) TestFlatten(c *C) {
	s := []int{1, 2, 3, 4}
	fs := flatten(s)
	c.Assert(len(s), Equals, len(fs))
	c.Assert(flattenedSliceLength(s), Equals, len(s))

	m := map[string]int{"a": 1, "b": 2, "c": 3}
	fm := flatten(m)
	c.Assert(len(fm), Equals, len(m)*2)

	b := []byte("test")
	fb := flatten(b)
	c.Assert(len(fb), Equals, 1)

	fl := flattenedLength(
		Resp{val: "A"},
		&Resp{val: "A"},
		[]int{1},
		&timeoutError{},
	)

	c.Assert(flattenedLength(fl), Equals, 1)
}

func (rs *RedySuite) TestRead(c *C) {
	r := bufio.NewReader(&errReader{})

	_, err := readSimpleStr(r)
	c.Assert(err, NotNil)

	_, err = readError(r)
	c.Assert(err, NotNil)

	_, err = readInt(r)
	c.Assert(err, NotNil)

	_, err = readBulkStr(r)
	c.Assert(err, NotNil)

	_, err = readArray(r)
	c.Assert(err, NotNil)
}

func (rs *RedySuite) TestKeyspaceInfoParser(c *C) {
	info := parseDBInfo("keys=22219,expires=20994,avg_ttl=298990394")

	c.Assert(info.Keys, Equals, uint64(22219))
	c.Assert(info.Expires, Equals, uint64(20994))
	c.Assert(info.AvgTTL, Equals, uint64(298990394))

	info = parseDBInfo(" ")

	c.Assert(info.Keys, Equals, uint64(0))
	c.Assert(info.Expires, Equals, uint64(0))
	c.Assert(info.AvgTTL, Equals, uint64(0))
}

func (rs *RedySuite) TestKeyspaceCalculation(c *C) {
	r := rs.c.Cmd("SETEX", "_expires", 100000, "test")
	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)

	r = rs.c.Cmd("INFO")

	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	c.Assert(r.HasType(STR_BULK), Equals, true)

	info, err := ParseInfo(r)

	c.Assert(err, IsNil)
	c.Assert(info, NotNil)

	c.Assert(info.Keyspace.Databases, HasLen, 1)
	c.Assert(info.Keyspace.DBList, HasLen, 1)
	c.Assert(info.Keyspace.Databases[0], Equals, 0)
	c.Assert(info.Keyspace.DBList[0].Keys, Not(Equals), uint64(0))
	c.Assert(info.Keyspace.DBList[0].Expires, Equals, uint64(1))
	c.Assert(info.Keyspace.Keys(), Not(Equals), uint64(0))
	c.Assert(info.Keyspace.Expires(), Not(Equals), uint64(0))
}

func (rs *RedySuite) TestAux(c *C) {
	c.Assert(extractConfValue("abc"), Equals, "abc")

	c.Assert(parseSize("1 MB"), Equals, uint64(1024*1024))
	c.Assert(parseSize("1 M"), Equals, uint64(1000*1000))
	c.Assert(parseSize("2tb"), Equals, uint64(2*1024*1024*1024*1024))
	c.Assert(parseSize("2t"), Equals, uint64(2*1000*1000*1000*1000))
	c.Assert(parseSize("5gB"), Equals, uint64(5*1024*1024*1024))
	c.Assert(parseSize("5g"), Equals, uint64(5*1000*1000*1000))
	c.Assert(parseSize("13kb"), Equals, uint64(13*1024))
	c.Assert(parseSize("13k"), Equals, uint64(13*1000))
	c.Assert(parseSize("512"), Equals, uint64(512))
	c.Assert(parseSize("512b"), Equals, uint64(512))
	c.Assert(parseSize("kb"), Equals, uint64(0))
	c.Assert(parseSize("123!"), Equals, uint64(0))

	c.Assert(intv(int8(2)), Equals, int(2))
	c.Assert(intv(int16(2)), Equals, int(2))
	c.Assert(intv(int32(2)), Equals, int(2))
	c.Assert(intv(int64(2)), Equals, int(2))
	c.Assert(intv(uint(2)), Equals, int(2))
	c.Assert(intv(uint8(2)), Equals, int(2))
	c.Assert(intv(uint16(2)), Equals, int(2))
	c.Assert(intv(uint32(2)), Equals, int(2))
	c.Assert(intv(uint64(2)), Equals, int(2))
	c.Assert(intv(""), Equals, int(-1))

	r := &Resp{typ: ERR_IO, Err: &net.OpError{Err: &os.SyscallError{Err: &timeoutError{}}}}
	c.Assert(isTimeout(r), Equals, true)

	r = &Resp{}
	c.Assert(isTimeout(r), Equals, false)

	buf := bytes.NewBufferString("ABCD")
	rdr := NewRespReader(buf)
	_, err := bufioReadResp(rdr.r)
	c.Assert(err, NotNil)

	c.Assert(readField("", 0, true, ""), Equals, "")
}

// ////////////////////////////////////////////////////////////////////////////////// //

func pretendRead(s string) *Resp {
	buf := bytes.NewBufferString(s)
	return NewRespReader(buf).Read()
}

func randString(length int) string {
	symbols := "QWERTYUIOPASDFGHJKLZXCVBNMqwertyuiopasdfghjklzxcvbnm1234567890"

	if length <= 0 {
		return ""
	}

	symbolsLength := len(symbols)
	result := make([]byte, length)

	rand.Seed(time.Now().UTC().UnixNano())

	for i := 0; i < length; i++ {
		result[i] = symbols[rand.Intn(symbolsLength)]
	}

	return string(result)
}

// ////////////////////////////////////////////////////////////////////////////////// //

func (t *timeoutError) Timeout() bool {
	return true
}

func (t *timeoutError) Error() string {
	return ""
}

func (r *errReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("ERROR")
}
