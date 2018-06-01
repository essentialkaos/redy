package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bytes"
	"errors"
	"math/rand"
	"os"
	"testing"
	"time"

	. "pkg.re/check.v1"
)

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
		Network: "tcp",
		Addr:    "127.0.0.255:60000",
	}

	resp := rc.Cmd("PING")

	c.Assert(resp.Err, NotNil)

	err := rc.Connect()

	c.Assert(err, NotNil)
}

func (rs *RedySuite) TestCmd(c *C) {
	r := rs.c.Cmd("ECHO", "TEST1234")
	respStr, err := r.Str()
	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	c.Assert(r.IsType(Str), Equals, true)
	c.Assert(err, IsNil)
	c.Assert(respStr, Equals, "TEST1234")

	r = rs.c.Cmd("UNKNOWN_COMMAND")
	c.Assert(r, NotNil)
	c.Assert(r.Err, NotNil)
	c.Assert(r.IsType(RedisErr), Equals, true)

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
	args := map[string]interface{}{
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
		c.Assert(r.IsType(RedisErr), Equals, true)
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
	c.Assert(r.IsType(SimpleStr), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("TEST1234"))
	s, err := r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "TEST1234")
	c.Assert(r.String(), Equals, "Resp(Str \"TEST1234\")")

	// Empty simple string
	r = pretendRead("+\r\n")
	c.Assert(r.IsType(SimpleStr), Equals, true)
	c.Assert(r.val, DeepEquals, []byte(""))
	s, err = r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "")
	c.Assert(r.String(), Equals, "Resp(Str \"\")")

	// Error
	r = pretendRead("-TEST1234\r\n")
	c.Assert(r.IsType(RedisErr), Equals, true)
	c.Assert(r.val, DeepEquals, errors.New("TEST1234"))
	c.Assert(r.Err.Error(), Equals, "TEST1234")
	c.Assert(r.String(), Equals, "Resp(RedisErr \"TEST1234\")")

	// Empty error
	r = pretendRead("-\r\n")
	c.Assert(r.IsType(RedisErr), Equals, true)
	c.Assert(r.val, DeepEquals, errors.New(""))
	c.Assert(r.Err.Error(), Equals, "")
	c.Assert(r.String(), Equals, "Resp(RedisErr \"\")")

	// Int
	r = pretendRead(":1024\r\n")
	c.Assert(r.IsType(Int), Equals, true)
	c.Assert(r.val, Equals, int64(1024))
	i, err := r.Int()
	c.Assert(err, IsNil)
	c.Assert(i, Equals, 1024)
	c.Assert(r.String(), Equals, "Resp(Int 1024)")

	// Int (from string)
	r = pretendRead("+50\r\n")
	c.Assert(r.IsType(SimpleStr), Equals, true)
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
	c.Assert(r.IsType(SimpleStr), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("TEST1234"))
	_, err = r.Int()
	c.Assert(err, NotNil)
	c.Assert(r.String(), Equals, "Resp(Str \"TEST1234\")")

	// Bulk string
	r = pretendRead("$8\r\nTEST1234\r\n")
	c.Assert(r.IsType(BulkStr), Equals, true)
	c.Assert(r.val, DeepEquals, []byte("TEST1234"))
	s, err = r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "TEST1234")
	c.Assert(r.String(), Equals, "Resp(BulkStr \"TEST1234\")")

	// Empty bulk string
	r = pretendRead("$0\r\n\r\n")
	c.Assert(r.IsType(BulkStr), Equals, true)
	c.Assert(r.val, DeepEquals, []byte(""))
	s, err = r.Str()
	c.Assert(err, IsNil)
	c.Assert(s, Equals, "")
	c.Assert(r.String(), Equals, "Resp(BulkStr \"\")")

	// Nil bulk string
	r = pretendRead("$-1\r\n")
	c.Assert(r.IsType(Nil), Equals, true)
	c.Assert(r.String(), Equals, "Resp(Nil)")

	// Array
	r = pretendRead("*2\r\n+TEST\r\n+1234\r\n")
	c.Assert(r.IsType(Array), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 2)
	c.Assert(r.val.([]Resp)[0].IsType(SimpleStr), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("TEST"))
	c.Assert(r.val.([]Resp)[1].IsType(SimpleStr), Equals, true)
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
	c.Assert(r.IsType(Array), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 0)
	c.Assert(r.String(), Equals, "Resp(Empty Array)")

	// Nil Array
	r = pretendRead("*-1\r\n")
	c.Assert(r.IsType(Nil), Equals, true)
	c.Assert(r.String(), Equals, "Resp(Nil)")

	// Embedded Array
	r = pretendRead("*3\r\n+TEST\r\n+1234\r\n*2\r\n+STUB\r\n+5678\r\n")
	c.Assert(r.String(), Equals, "Resp(0:Resp(Str \"TEST\") 1:Resp(Str \"1234\") 2:Resp(0:Resp(Str \"STUB\") 1:Resp(Str \"5678\")))")
	c.Assert(r.IsType(Array), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 3)
	c.Assert(r.val.([]Resp)[0].IsType(SimpleStr), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("TEST"))
	c.Assert(r.val.([]Resp)[1].IsType(SimpleStr), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("1234"))
	r = &r.val.([]Resp)[2]
	c.Assert(r.IsType(Array), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 2)
	c.Assert(r.val.([]Resp)[0].IsType(SimpleStr), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("STUB"))
	c.Assert(r.val.([]Resp)[1].IsType(SimpleStr), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("5678"))

	// Test that two bulks in a row read correctly
	r = pretendRead("*2\r\n$4\r\nTEST\r\n$4\r\n1234\r\n")
	c.Assert(r.String(), Equals, "Resp(0:Resp(BulkStr \"TEST\") 1:Resp(BulkStr \"1234\"))")
	c.Assert(r.IsType(Array), Equals, true)
	c.Assert(len(r.val.([]Resp)), Equals, 2)
	c.Assert(r.val.([]Resp)[0].IsType(BulkStr), Equals, true)
	c.Assert(r.val.([]Resp)[0].val, DeepEquals, []byte("TEST"))
	c.Assert(r.val.([]Resp)[1].IsType(BulkStr), Equals, true)
	c.Assert(r.val.([]Resp)[1].val, DeepEquals, []byte("1234"))
}

func (rs *RedySuite) TestInfoParser(c *C) {
	r := rs.c.Cmd("INFO")

	c.Assert(r, NotNil)
	c.Assert(r.Err, IsNil)
	c.Assert(r.IsType(BulkStr), Equals, true)

	info, err := ParseInfo(r)

	c.Assert(err, IsNil)
	c.Assert(info, NotNil)

	c.Assert(info.Get("server", "redis_mode"), Equals, "standalone")
	c.Assert(info.GetI("server", "hz"), Equals, 10)
	c.Assert(info.GetU("server", "hz"), Equals, uint64(10))
	c.Assert(info.GetF("memory", "mem_fragmentation_ratio"), Not(Equals), 0.0)
}

func (rs *RedySuite) TestConfigParsers(c *C) {
	var cfg *Config

	c.Assert(cfg.Get("abc"), Equals, "")
	c.Assert(cfg.Get(""), Equals, "")

	fileConf, err := ReadConfig(".travis/test.conf")

	c.Assert(err, NotNil)
	c.Assert(fileConf, IsNil)

	fileConf, err = ReadConfig(".travis/full.conf")

	c.Assert(err, IsNil)
	c.Assert(fileConf, NotNil)

	fcKeepalive := fileConf.Get("tcp-keepalive")
	fcAuth := fileConf.Get("masterauth")
	fcSave := fileConf.Get("save")
	fcLimit := fileConf.Get("client-output-buffer-limit")

	c.Assert(fcKeepalive, Equals, "300")
	c.Assert(fcAuth, Equals, "")
	c.Assert(fcSave, Equals, "900 1 300 10 60 10000")

	memConf, err := rs.c.GetConfig("ECHO")

	c.Assert(err, NotNil)
	c.Assert(memConf, IsNil)

	memConf, err = rs.c.GetConfig("CONFIG")

	c.Assert(err, IsNil)
	c.Assert(memConf, NotNil)

	c.Assert(memConf.Get("tcp-keepalive"), Equals, fcKeepalive)
	c.Assert(memConf.Get("masterauth"), Equals, fcAuth)
	c.Assert(memConf.Get("save"), Equals, fcSave)
	c.Assert(memConf.Get("client-output-buffer-limit"), Equals, fcLimit)

	resp := &Resp{typ: SimpleStr, val: ""}
	_, err = parseInMemoryConfig(resp)
	c.Assert(err, NotNil)

	resp = &Resp{typ: Array, val: []Resp{Resp{}, Resp{}, Resp{}}}
	_, err = parseInMemoryConfig(resp)
	c.Assert(err, NotNil)
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
