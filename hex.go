package main

import (
	"encoding/hex"
	"log"
	"strings"
)

// de-$HEX[] lines
func checkForHex(line string) string {
	if strings.HasSuffix(line, "\r") {
		line = strings.TrimSuffix(line, "\r")
	}

	if !strings.HasPrefix(line, "$HEX[") {
		return line
	}
	// check for trailing bracket ]
	if !strings.HasSuffix(line, "]") {
		line += "]"
	}
	// extract text inside brackets
	start := strings.Index(line, "[")
	end := strings.LastIndex(line, "]")
	hexContent := line[start+1 : end]

	// try to decode
	lineDecode, err := hex.DecodeString(hexContent)
	if err != nil {
		// strip invalid chars (here we go again trying defoobar crappy wordlists)
		cleaned := strings.Map(func(r rune) rune {
			if strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return r
			}
			return -1
		}, hexContent)
		// pad to even length if needed (2x foobar award)
		if len(cleaned)%2 != 0 {
			cleaned = "0" + cleaned
		}
		lineDecode, err = hex.DecodeString(cleaned)
		if err != nil {
			log.Printf("hex decode failed: %v", err)
			return line
		}
	}
	return string(lineDecode)
}
