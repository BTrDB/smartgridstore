package configparser

import (
	"fmt"
	"strings"
	)

func ParseConfig(str string) (toReturn map[string]interface{}, err bool) {
	var lines []string = strings.Split(str, "\n")
	for i := 0; i < len(lines); i++ {
		lines[i] = strings.TrimSpace(lines[i])
	}
	toReturn, _, err = parseLevel(lines, 0, 0)
	return
}

func parseLevel(lines []string, index int, currLevel int) (map[string]interface{}, int, bool) {
	kvpairs := make(map[string]interface{})
	numlines := len(lines)
	var line string
	var i int
	var secname string
	var kv []string
	var err bool = false
	var tempErr bool
	for index < numlines {
		line = lines[index]
		if len(line) == 0 || line[0] == '#' {
			index++
			continue
		}
		for i = 0; line[i] == '[' && i < len(line); i++ {}
		if i > 0 && line[len(line) - i] == ']' {
			if i <= currLevel {
				return kvpairs, index, err
			} else if i == currLevel + 1 {
				secname = string(line[i:len(line) - i])
				kvpairs[secname], index, tempErr = parseLevel(lines, index + 1, currLevel + 1)
				if tempErr {
					err = tempErr
				}
				continue
			} else {
				err = true
				fmt.Printf("line %v: skips from level %v to level %v\n", index + 1, currLevel, i)
			}
		} else {
			kv = strings.SplitN(line, "=", 2)
			if len(kv) == 2 {
				kvpairs[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			} else {
				err = true
				fmt.Printf("line %v: statement declares neither a key nor a section\n", index + 1)
			}
		}
		index++
	}
	return kvpairs, index, err
}
