package throttler

import (
	"bytes"
	"strings"
)

func getRegex(strs []string) []string {
	result := make([]string, len(strs))

	for i, str := range strs {
		if len(str) == 0 {
			continue
		}

		var strsSplited []string
		var regex bytes.Buffer

		strsSplited = append(strsSplited, strings.Split(str, "*")...)
		regex.WriteString("^")

		for i, expr := range strsSplited {
			regex.WriteString(expr)

			if strsSplited[i] == "" || i+1 < len(strsSplited) && strsSplited[i+1] != "" {
				regex.WriteString(".*")
			}
		}

		regex.WriteString("$")
		result[i] = regex.String()
	}

	return result
}
