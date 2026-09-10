package domain

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type CueSheet struct {
	Genre, Date, Performer, Title string
	File                          string
	Tracks                        []CueTrack
}

type CueTrack struct {
	Number           int
	Title, Performer string
	Index00, Index01 string
}

func ParseCue(data []byte) (CueSheet, error) {
	var sheet CueSheet
	var cur *CueTrack
	fileSeen := false

	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "REM":
			parseREM(&sheet, fields)
		case "PERFORMER":
			if cur != nil {
				cur.Performer = unquoteRest(line, "PERFORMER")
			} else {
				sheet.Performer = unquoteRest(line, "PERFORMER")
			}
		case "TITLE":
			if cur != nil {
				cur.Title = unquoteRest(line, "TITLE")
			} else {
				sheet.Title = unquoteRest(line, "TITLE")
			}
		case "FILE":
			if fileSeen {
				return CueSheet{}, ErrMultiFileCue
			}
			fileSeen = true
			sheet.File = parseCueFile(line)
		case "TRACK":
			if len(fields) >= 2 {
				n, err := strconv.Atoi(fields[1])
				if err != nil {
					return CueSheet{}, fmt.Errorf("invalid track number %q", fields[1])
				}
				sheet.Tracks = append(sheet.Tracks, CueTrack{Number: n})
				cur = &sheet.Tracks[len(sheet.Tracks)-1]
			}
		case "INDEX":
			if cur != nil && len(fields) >= 3 {
				idx := fields[1]
				val := fields[2]
				switch idx {
				case "00":
					cur.Index00 = val
				case "01":
					cur.Index01 = val
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return CueSheet{}, err
	}
	return sheet, nil
}

func parseCueFile(line string) string {
	rest := strings.TrimSpace(line[len("FILE"):])
	if rest == "" {
		return ""
	}
	if rest[0] != '"' {
		return strings.Fields(rest)[0]
	}
	var value strings.Builder
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case '"':
			return value.String()
		case '\\':
			if i+1 < len(rest) && (rest[i+1] == '"' || rest[i+1] == '\\') {
				i++
				value.WriteByte(rest[i])
				continue
			}
			value.WriteByte(rest[i])
		default:
			value.WriteByte(rest[i])
		}
	}
	return value.String()
}

func parseREM(sheet *CueSheet, fields []string) {
	if len(fields) < 2 {
		return
	}
	key := strings.ToUpper(fields[1])
	switch key {
	case "GENRE":
		sheet.Genre = unquoteFields(fields, 2)
	case "DATE":
		sheet.Date = unquoteFields(fields, 2)
	}
}

func unquoteRest(line, keyword string) string {
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, strings.ToUpper(keyword))
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[idx+len(keyword):])
	return strings.Trim(rest, `"`)
}

func unquoteFields(fields []string, from int) string {
	if from >= len(fields) {
		return ""
	}
	return strings.Trim(strings.Join(fields[from:], " "), `"`)
}
