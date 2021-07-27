package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func main() {
	var err error
	//input := []string{"*/status","/status","/status*", "*/status*", "blabla/*/status", "*blabla/**/status*", "", "*"}
	input := []string{"/servers/*/status"}
	testData := []string{"/servers/123/status", "/servers/123/status", "/status", "/status/fdg", ""}
	rules := getRegex(input)
	for _, test := range testData {

		b := false
		for _, rule := range rules {
			b, err = regexp.MatchString(rule, test)
			if err != nil {
				fmt.Println(err)
			}

			fmt.Printf("%s %s\n", test, b)
		}
	}
}
