package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Config is struct with Redis configuration data
type Config struct {
	Props []string
	Data  map[string][]string
}

// ////////////////////////////////////////////////////////////////////////////////// //

var ErrWrongConfResponse = errors.New("CONFIG command response must have Array type")

// ////////////////////////////////////////////////////////////////////////////////// //

// ReadConfig reads and parses Redis configuration file
func ReadConfig(file string) (*Config, error) {
	fd, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer fd.Close()

	return parseConfigData(bufio.NewReader(fd))
}

// ParseConfig parse full in-memory config
func ParseConfig(r *Resp) (*Config, error) {
	if !r.HasType(ARRAY) {
		return nil, ErrWrongConfResponse
	}

	return parseInMemoryConfig(r)
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Get returns configuration property value as string
func (c *Config) Get(prop string) string {
	if c == nil || prop == "" {
		return ""
	}

	value, ok := c.Data[prop]

	if !ok || len(value) == 0 {
		return ""
	}

	if len(value) == 1 {
		return value[0]
	}

	return strings.Join(value, " ")
}

// Has checks if given configuration property exists in configuration
func (c *Config) Has(prop string) bool {
	if c == nil || prop == "" {
		return false
	}

	_, ok := c.Data[prop]

	return ok
}

// Diff compares two configurations and returns slice with names of
// differ properties
func (c *Config) Diff(nc *Config) []string {
	if c == nil || nc == nil {
		return []string{}
	}

	var result []string

	for _, prop := range c.Props {
		if c.Get(prop) != nc.Get(prop) {
			result = append(result, prop)
		}
	}

	for _, prop := range nc.Props {
		_, has := c.Data[prop]

		if !has {
			result = append(result, prop)
		}
	}

	return result
}

// ////////////////////////////////////////////////////////////////////////////////// //

func parseConfigData(r *bufio.Reader) (*Config, error) {
	scanner := bufio.NewScanner(r)

	config := &Config{
		Props: make([]string, 0),
		Data:  make(map[string][]string),
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		line = strings.Trim(line, " ")

		if len(line) == 0 || strings.HasPrefix(line, "#") || !strings.Contains(line, " ") {
			continue
		}

		p := readField(line, 0, false, " ")

		if config.Data[p] == nil {
			config.Props = append(config.Props, p)
			config.Data[p] = []string{extractConfValue(line)}
		} else {
			config.Data[p] = append(config.Data[p], extractConfValue(line))
		}
	}

	return config, nil
}

func parseInMemoryConfig(r *Resp) (*Config, error) {
	items, err := r.Array()

	if err != nil {
		return nil, err
	}

	itemsNum := len(items)

	if itemsNum%2 != 0 {
		return nil, errors.New("Wrong number of items in CONFIG response")
	}

	config := &Config{
		Props: make([]string, 0),
		Data:  make(map[string][]string),
	}

	for i := 0; i < itemsNum; i += 2 {
		prop, _ := items[i].Str()
		value, _ := items[i+1].Str()

		config.Props = append(config.Props, prop)
		config.Data[prop] = []string{value}
	}

	return config, nil
}

func extractConfValue(line string) string {
	index := strings.Index(line, " ")

	if index == -1 {
		return line
	}

	return processConfValue(strings.TrimLeft(line[index:], " "))
}

func processConfValue(line string) string {
	for i := 0; i < strings.Count(line, " ")+1; i++ {
		v := readField(line, i, true, " ")

		if isSize(v) {
			s := parseSize(v)
			line = strings.Replace(line, v, strconv.FormatUint(s, 10), -1)
		}
	}

	return line
}

func isSize(v string) bool {
	for _, r := range v {
		switch r {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			't', 'g', 'm', 'k', 'b', 'T', 'G', 'M', 'K', 'B':
			continue

		default:
			return false
		}
	}

	return true
}

func parseSize(size string) uint64 {
	ns := strings.ToLower(strings.Replace(size, " ", "", -1))
	mlt, sfx := extractSizeInfo(ns)

	if sfx == "" {
		num, err := strconv.ParseUint(size, 10, 64)

		if err != nil {
			return 0
		}

		return num
	}

	ns = strings.TrimRight(ns, sfx)
	numFlt, err := strconv.ParseFloat(ns, 64)

	if err != nil {
		return 0
	}

	return uint64(numFlt * float64(mlt))
}

func extractSizeInfo(s string) (uint64, string) {
	var mlt uint64
	var sfx string

	switch {
	case strings.HasSuffix(s, "tb"):
		mlt = 1024 * 1024 * 1024 * 1024
		sfx = "tb"
	case strings.HasSuffix(s, "t"):
		mlt = 1000 * 1000 * 1000 * 1000
		sfx = "t"
	case strings.HasSuffix(s, "gb"):
		mlt = 1024 * 1024 * 1024
		sfx = "gb"
	case strings.HasSuffix(s, "g"):
		mlt = 1000 * 1000 * 1000
		sfx = "g"
	case strings.HasSuffix(s, "mb"):
		mlt = 1024 * 1024
		sfx = "mb"
	case strings.HasSuffix(s, "m"):
		mlt = 1000 * 1000
		sfx = "m"
	case strings.HasSuffix(s, "kb"):
		mlt = 1024
		sfx = "kb"
	case strings.HasSuffix(s, "k"):
		mlt = 1000
		sfx = "k"
	case strings.HasSuffix(s, "b"):
		mlt = 1
		sfx = "b"
	}

	return mlt, sfx
}
