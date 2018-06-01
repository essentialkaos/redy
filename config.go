package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Config is struct with Redis config data
type Config struct {
	Props []string
	Data  map[string][]string
}

// ////////////////////////////////////////////////////////////////////////////////// //

// ReadConfig read and parse Redis config
func ReadConfig(file string) (*Config, error) {
	fd, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer fd.Close()

	reader := bufio.NewReader(fd)
	scanner := bufio.NewScanner(reader)

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

// ////////////////////////////////////////////////////////////////////////////////// //

// Get return config property as string
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

// ////////////////////////////////////////////////////////////////////////////////// //

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
		prop, err := items[i].Str()

		if err != nil {
			return nil, err
		}

		value, err := items[i+1].Str()

		if err != nil {
			return nil, err
		}

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

	return strings.TrimLeft(line[index:], " ")
}
