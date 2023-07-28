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
	Databases []int
	DBList    map[int]*DBInfo
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

// ReplicaInfo contains info about connected replica
type ReplicaInfo struct {
	IP     string
	Port   int
	State  string
	Offset int64
	Lag    int64
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

// Flatten flatten info data
func (i *Info) Flatten() [][2]string {
	var result [][2]string

	for _, section := range i.Sections {
		for _, field := range section.Fields {
			result = append(result, [2]string{field, section.Values[field]})
		}
	}

	for db, dbInfo := range i.Keyspace.DBList {
		dbName := "db" + strconv.Itoa(db)
		result = append(result, [2]string{dbName + "_keys", fmt.Sprintf("%d", dbInfo.Keys)})
		result = append(result, [2]string{dbName + "_expires", fmt.Sprintf("%d", dbInfo.Expires)})
	}

	result = append(result, [2]string{"keys_total", fmt.Sprintf("%d", i.Keyspace.Keys())})
	result = append(result, [2]string{"expires_total", fmt.Sprintf("%d", i.Keyspace.Expires())})

	return result
}

// Get returns field value as string
func (i *Info) Get(section string, fields ...string) string {
	if i == nil || section == "" || len(fields) == 0 {
		return ""
	}

	sec, ok := i.Sections[strings.ToLower(section)]

	if !ok {
		return ""
	}

	for _, field := range fields {
		val, ok := sec.Values[strings.ToLower(field)]

		if ok {
			return val
		}
	}

	return ""
}

// GetB returns field value as boolean
func (i *Info) GetB(section string, fields ...string) bool {
	rs := i.Get(section, fields...)

	switch strings.ToLower(rs) {
	case "1", "ok":
		return true
	}

	return false
}

// GetI returns field value as int
func (i *Info) GetI(section string, fields ...string) int {
	rs := i.Get(section, fields...)

	if rs == "" {
		return 0
	}

	ri, _ := strconv.Atoi(rs)

	return ri
}

// GetF returns field value as float64
func (i *Info) GetF(section string, fields ...string) float64 {
	rs := i.Get(section, fields...)

	if rs == "" {
		return 0.0
	}

	rf, _ := strconv.ParseFloat(rs, 64)

	return rf
}

// GetU returns field value as uint64
func (i *Info) GetU(section string, fields ...string) uint64 {
	rs := i.Get(section, fields...)

	if rs == "" {
		return 0
	}

	ru, _ := strconv.ParseUint(rs, 10, 64)

	return ru
}

// Is checks if field value is equals to given value
func (i *Info) Is(section string, field string, value any) bool {
	switch t := value.(type) {
	case bool:
		return i.GetB(section, field) == t
	case int:
		return i.GetI(section, field) == t
	case float64:
		return i.GetF(section, field) == t
	case uint64:
		return i.GetU(section, field) == t
	}

	return i.Get(section, field) == fmt.Sprintf("%s", value)
}

// GetReplicaInfo parses and returns info about connected replica with given index
func (i *Info) GetReplicaInfo(index int) *ReplicaInfo {
	rawInfo := i.Get("Replication",
		"slave"+strconv.Itoa(index),
		"replica"+strconv.Itoa(index),
	)

	if rawInfo == "" {
		return nil
	}

	ip := readField(rawInfo, 1, false, "=", ",")
	port := readField(rawInfo, 3, false, "=", ",")
	state := readField(rawInfo, 5, false, "=", ",")
	offset := readField(rawInfo, 7, false, "=", ",")
	lag := readField(rawInfo, 9, false, "=", ",")

	portInt, _ := strconv.Atoi(port)
	offsetInt, _ := strconv.ParseInt(offset, 10, 64)
	lagInt, _ := strconv.ParseInt(lag, 10, 64)

	return &ReplicaInfo{
		IP:     ip,
		Port:   int(portInt),
		State:  state,
		Offset: offsetInt,
		Lag:    lagInt,
	}
}

// ////////////////////////////////////////////////////////////////////////////////// //

// Keys calculates number of keys
func (k *KeyspaceInfo) Keys() uint64 {
	var result uint64

	for _, i := range k.DBList {
		result += i.Keys
	}

	return result
}

// Expires calculates number of expires
func (k *KeyspaceInfo) Expires() uint64 {
	var result uint64

	for _, i := range k.DBList {
		result += i.Expires
	}

	return result
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
			Databases: make([]int, 0),
			DBList:    make(map[int]*DBInfo),
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
				dbName, _ := strconv.Atoi(strings.TrimLeft(k, "db"))
				dbInfo := parseDBInfo(v)

				info.Keyspace.Databases = append(info.Keyspace.Databases, dbName)
				info.Keyspace.DBList[dbName] = dbInfo
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
