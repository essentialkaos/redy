package redy

// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// Info contains parsed INFO data
type Info struct {
	SectionNames []string
	Sections     map[string]*InfoSection
	Keyspace     *KeyspaceInfo
}

// KeyspaceInfo contains info about keyspace
type KeyspaceInfo struct {
	Databases []string
	DBList    map[string]*DBInfo
	Total     *DBInfo
}

// DBInfo contains info about single db
type DBInfo struct {
	Keys    uint64
	Expires uint64
	AvgTTL  uint64
}

// InfoSection contains section info
type InfoSection struct {
	Header string
	Fields []string
	Values map[string]string
}

// ////////////////////////////////////////////////////////////////////////////////// //

var defaultFieldsSeparators = []string{":"}

// ////////////////////////////////////////////////////////////////////////////////// //

// ParseInfo parses INFO command output
func ParseInfo(r *Resp) (*Info, error) {
	if !r.HasType(STR_BULK) {
		return nil, errors.New("Can't parse INFO data: wrong resp type")
	}

	rawInfo, err := r.Str()

	if err != nil {
		return nil, fmt.Errorf("Can't parse INFO data: %v", err)
	}

	info, err := parseRedisInfo(rawInfo)

	if err != nil {
		return nil, fmt.Errorf("Can't parse INFO data: %v", err)
	}

	return info, nil
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Get returns field value as string
func (i *Info) Get(section, field string) string {
	if i == nil || section == "" || field == "" {
		return ""
	}

	s, ok := i.Sections[strings.ToLower(section)]

	if !ok {
		return ""
	}

	return s.Values[strings.ToLower(field)]
}

// GetI returns field value as int
func (i *Info) GetI(section, field string) int {
	rs := i.Get(section, field)

	if rs == "" {
		return -1
	}

	ri, _ := strconv.Atoi(rs)

	return ri
}

// GetF returns field value as float64
func (i *Info) GetF(section, field string) float64 {
	rs := i.Get(section, field)

	if rs == "" {
		return -1
	}

	rf, _ := strconv.ParseFloat(rs, 64)

	return rf
}

// GetU returns field value as uint64
func (i *Info) GetU(section, field string) uint64 {
	rs := i.Get(section, field)

	if rs == "" {
		return 0
	}

	ru, _ := strconv.ParseUint(rs, 10, 64)

	return ru
}

// ////////////////////////////////////////////////////////////////////////////////// //

// codebeat:disable[ABC,LOC]

func parseRedisInfo(rawInfo string) (*Info, error) {
	if len(rawInfo) == 0 {
		return nil, errors.New("INFO data is empty")
	}

	var section *InfoSection

	var info = &Info{
		Sections: make(map[string]*InfoSection),
		Keyspace: &KeyspaceInfo{
			Databases: make([]string, 0),
			DBList:    make(map[string]*DBInfo),
			Total:     &DBInfo{},
		},
	}

	for _, line := range strings.Split(rawInfo, "\n") {
		if len(line) <= 1 {
			continue
		}

		sectionName := strings.TrimRight(line, "\r")

		if strings.HasPrefix(sectionName, "#") {
			section = &InfoSection{
				Header: strings.TrimPrefix(sectionName, "# "),
				Fields: make([]string, 0),
				Values: make(map[string]string),
			}

			info.Sections[section.Header] = section
			info.Sections[strings.ToLower(section.Header)] = section
			info.SectionNames = append(info.SectionNames, section.Header)
		} else {
			if section == nil {
				continue
			}

			v := readField(sectionName, 1, false)

			if len(v) == 0 {
				continue
			}

			k := readField(sectionName, 0, false)

			section.Fields = append(section.Fields, k)
			section.Values[k] = v

			if section.Header == "Keyspace" {
				dbName := strings.TrimLeft(k, "db")
				dbInfo := parseDBInfo(v)

				info.Keyspace.Databases = append(info.Keyspace.Databases, dbName)
				info.Keyspace.DBList[dbName] = dbInfo

				info.Keyspace.Total.Keys += dbInfo.Keys
				info.Keyspace.Total.Expires += dbInfo.Expires
			}
		}
	}

	return info, nil
}

// codebeat:enable[ABC,LOC]

func parseDBInfo(info string) *DBInfo {
	kv, _ := strconv.ParseUint(readField(info, 1, false, "=", ","), 10, 64)
	ev, _ := strconv.ParseUint(readField(info, 3, false, "=", ","), 10, 64)
	tv, _ := strconv.ParseUint(readField(info, 5, false, "=", ","), 10, 64)

	return &DBInfo{Keys: kv, Expires: ev, AvgTTL: tv}
}

// codebeat:disable[CYCLO]

func readField(data string, index int, multiSep bool, separators ...string) string {
	if data == "" || index < 0 {
		return ""
	}

	if len(separators) == 0 {
		separators = defaultFieldsSeparators
	}

	curIndex, startPointer := -1, -1

MAINLOOP:
	for i, r := range data {
		for _, s := range separators {
			if r == rune(s[0]) {
				if curIndex == index {
					return data[startPointer:i]
				}

				if !multiSep {
					startPointer = i + 1
					curIndex++
					continue MAINLOOP
				}

				startPointer = -1
				continue MAINLOOP
			}
		}

		if startPointer == -1 {
			startPointer = i
			curIndex++
		}
	}

	if index > curIndex {
		return ""
	}

	return data[startPointer:]
}

// codebeat:enable[CYCLO]
