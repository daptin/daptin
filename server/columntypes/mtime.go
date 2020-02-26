package fieldtypes

import (
	"errors"
	"sort"
	"strings"
	"time"
	//"fmt"
	//	"fmt"
)

var timeFormat []string
var dateFormat []string
var dateTimeFormat []string

type ByLength []string

func (s ByLength) Len() int {
	return len(s)
}
func (s ByLength) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ByLength) Less(i, j int) bool {
	return len(s[i]) > len(s[j])
}

func init() {
	timeFormat = []string{
		"3:04PM",
		"3:04 PM",
	}
	dateFormat = []string{
		"02 Jan 2006",
		"Jan 02, 2006",
		"02 January 2006",
		"January 02, 2006",
		"January 02",
		"Jan 02",
		"20060102",
		"200601",
		"2006-01",
		"06",
		"2006",
		"2006.0",
		"2006.00",
		"2006.000",
		"2006 01/02",
		"2006 01 02",
		"2006 01",
		"2006/01",
		"01/02",
		"01 02",
		"06 01 02",
		"06 01",
		"2006/01/02",
		"02 Jan 06",
	}
	dateTimeFormat = []string{
		"01021504",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		"_2 Jan 2006 15:04:05 -0700",
		"Mon, _2 Jan 2006 15:04:05 -0700 (MST)",
		"Mon, _2 Jan 2006 15:04:05 -0700 (MST DST)",
		"Mon, _2 Jan 2006 15:04:05 -0700",
		"Mon Jan _2 15:04:05 2006",
		"Mon Jan _2 15:04:05 MST 2006",
		"Mon Jan 02 15:04:05 -0700 2006",
		"02 Jan 06 15:04 MST",
		"02 Jan 06 15:04 -0700",
		"2006-01-02 15:04:05.0",
		"2006-01-02 15:04:05.00",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05.000-0700",
		"2006-01-02 15:04:05.000-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.0",
		"Monday, 02-Jan-06 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		"Jan _2 15:04:05",
		"Jan _2 15:04:05.000",
		"Jan _2 15:04:05.000000",
		"Jan _2 15:04:05.000000000",
	}
	sort.Sort(ByLength(timeFormat))
}

func GetTime(t string) (time.Time, string, error) {
	if strings.Index(t, "0000") > -1 {
		return time.Time{}, "", errors.New("not a date")
	}
	for _, format := range timeFormat {
		//fmt.Printf("Testing %s with %s\n", t, format)
		t, err := time.Parse(format, t)
		if err == nil {
			return t, format, nil
		}
	}
	return time.Now(), "", errors.New("Unrecognised time format - " + t)
}

func GetDate(t1 string) (time.Time, string, error) {
	for _, format := range dateFormat {
		//fmt.Printf("Testing %s with %s\n", t1, format)
		t, err := time.Parse(format, t1)

		if err == nil {
			ret := true
			if format == "2006" || format == "2006.0" || format == "2006.00" || format == "2006.000" {
				if t.Sub(time.Now()).Hours() > 182943 {
					ret = false
				}
			}
			if format == "06" {
				if t.Sub(time.Now()).Hours() > -150179 {
					ret = false
				}
			}

			//log.Printf("Detected %v as date by format %s => %v, Hours: %d", t1, format, t, t.Sub(time.Now()).Hours())
			if ret {
				return t, format, nil
			}
		}
	}
	return time.Now(), "", errors.New("Unrecognised time format - " + t1)
}

func GetDateTime(t string) (time.Time, string, error) {
	for _, format := range dateTimeFormat {
		//fmt.Printf("Testing datetime %s with %s\n", t, format)
		t, err := time.Parse(format, t)
		if err == nil {
			return t, format, nil
		}
	}
	return time.Now(), "", errors.New("Unrecognised time format - " + t)
}

func GetTimeByFormat(t string, f string) (time.Time, error) {
	return time.Parse(f, t)
}
