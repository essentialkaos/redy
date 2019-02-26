// +build gofuzz

package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"bytes"
)

// ////////////////////////////////////////////////////////////////////////////////// //

func FuzzInfoParser(data []byte) int {
	b := bytes.NewBuffer(data)
	_, err := parseRedisInfo(b.String())

	if err != nil {
		return 0
	}

	return 1
}

func FuzzConfigParser(data []byte) int {
	r := bytes.NewReader(data)
	br := bufio.NewReader(r)

	_, err := parseConfigData(br)

	if err != nil {
		return 0
	}

	return 1
}

func FuzzRespReader(data []byte) int {
	r := bytes.NewReader(data)
	br := bufio.NewReader(r)

	_, err := bufioReadResp(br)

	if err != nil {
		return 0
	}

	return 1
}
