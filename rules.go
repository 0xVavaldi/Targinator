package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/cespare/xxhash/v2"
)

/*
Modifications by cyclone:
v0.0.1-2025-05-10-dev
	removed all C, CUDA (GPU) logic and converted to Pure Go using CPU
	refactored ParameterCountRule() switch case
*/

// Rule contains the core data structure
// Process is a function that applies the rule to a string
type Rule struct {
	Function          string
	Parameter1        string
	NumericParameter1 int
	Parameter2        string
	NumericParameter2 int
	Parameter3        string
	NumericParameter3 int
	Process           func(string) string // A lambda function for improved processing
}

// Helper function to reverse a string.
func reverseString(s string) string {
	runes := []rune(s)
	sort.SliceStable(runes, func(i, j int) bool { return i > j })
	return string(runes)
}

// CreateTestWords generates an array of words used to validate and verify if Rules are unique with UniqueID
//
//	testWords := rule.CreateTestWords()
//	Rules, _ := rule.ConvertFromHashcat(1, "$1 ]")
//	id := rule.UniqueID(testWords, Rules)
//	Rules2, _ := rule.ConvertFromHashcat(1, ": $2 ]")
//	id2 := rule.UniqueID(testWords, Rules2)
func CreateTestWords() []string {
	var testWords []string

	// Create 256 strings, each containing 37 characters of the same value (0x0 to 0xff)
	for i := 0x0; i <= 0xff; i++ {
		testWords = append(testWords, strings.Repeat(string(rune(i)), 37))
	}

	// Create a string with all possible hex values repeated 37 times with an appended 'a'
	var allChars strings.Builder
	for i := 0x0; i <= 0xff; i++ {
		for j := 0; j < 37; j++ {
			allChars.WriteRune(rune(i))
			allChars.WriteRune('a')
		}
	}
	testWords = append(testWords, allChars.String())

	// Reverse the string created above
	allCharsReversed := reverseString(allChars.String())
	testWords = append(testWords, allCharsReversed)

	// Create alphanumeric strings of different lengths
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for i := 0; i < 37; i++ {
		// Reverse every other alphabet
		currentAlphabet := alphabet
		if i%2 == 0 {
			currentAlphabet = reverseString(currentAlphabet)
		}
		// Erase characters from the start to keep only the last (i + 1) characters
		if len(currentAlphabet) > i+1 {
			currentAlphabet = currentAlphabet[len(currentAlphabet)-i-1:]
		}
		testWords = append(testWords, currentAlphabet)
	}
	return testWords
}

// UniqueID Generates an ID unique to the set of rules allowing you to compare two lines to each-other
func UniqueID(testWords *[]string, rules []Rule) uint64 {
	result := xxhash.New()
	for _, w := range *testWords {
		for _, rule := range rules {
			w = rule.Process(w)
		}
		_, err := result.WriteString(w)
		if err != nil {
			return 0
		}
	}
	return result.Sum64()
}

// ParameterCountRule displays the amount of required parameters
// refactored ParameterCountRule() switch case
func ParameterCountRule(rule string) (int, error) {
	if len(rule) == 0 {
		return -1, errors.New("Empty String")
	}
	switch rule[0] {
	// zero-parameter
	case ':', 'l', 'u', 'c', 'C', 't', 'r', 'd', 'f', '{', '}', '[', ']', 'k', 'K', 'q', 'E':
		return 0, nil
	// one-parameter
	case '@', 'T', 'p', 'D', 'Z', 'z', '$', '^', '<', '>', '_', '\'', '!', '/', 'y', 'Y', '-', '+', 'e', '.', ',', 'L', 'R':
		return 1, nil
	// two-parameter
	case 's', 'x', 'O', 'o', 'i', '3', '*':
		return 2, nil
	// three-parameter
	case 'S':
		return 3, nil
	}
	return -1, errors.New("Unknown Function")
}

// ParseTSVRules Takes TSV input and converts it to an array of Rules
func ParseTSVRules(lineCounter uint64, line string) ([]Rule, error) {
	rulesRaw := strings.Split(line, "\t")

	var parsedRules []Rule
	for _, rule := range rulesRaw {
		parsedRule, err := ParseSingleRule(rule)
		if err != nil {
			if err.Error() != "Empty String" {
				return nil, fmt.Errorf("Invalid rule '%s' on line %d: %v", rule, lineCounter, err)
			}
			continue
		}
		parsedRules = append(parsedRules, parsedRule)
	}
	return parsedRules, nil
}

// ParseSingleRule Parses a single rule such as : or s31 or $b
func ParseSingleRule(originalRule string) (Rule, error) {
	parameterCount, err := ParameterCountRule(originalRule)
	if parameterCount == -1 {
		return Rule{}, err
	}
	if len([]rune(originalRule))-1 < parameterCount {
		return Rule{}, errors.New("Missing parameters")
	}

	myRule := Rule{}
	if parameterCount >= 0 {
		myRule.Function = originalRule[0:1]
	}
	if parameterCount >= 1 {
		myRule.Parameter1 = originalRule[1:2]
	}
	if parameterCount >= 2 {
		myRule.Parameter2 = originalRule[2:3]
	}
	if parameterCount >= 3 {
		myRule.Parameter3 = originalRule[3:4]
	}

	alphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if strings.Contains(alphabet, myRule.Parameter1) && len(myRule.Parameter1) > 0 {
		myRule.NumericParameter1 = strings.IndexRune(alphabet, rune(myRule.Parameter1[0]))
	}
	if strings.Contains(alphabet, myRule.Parameter2) && len(myRule.Parameter2) > 0 {
		myRule.NumericParameter2 = strings.IndexRune(alphabet, rune(myRule.Parameter2[0]))
	}
	if strings.Contains(alphabet, myRule.Parameter3) && len(myRule.Parameter3) > 0 {
		myRule.NumericParameter3 = strings.IndexRune(alphabet, rune(myRule.Parameter3[0]))
	}

	switch myRule.Function {
	case ":":
		myRule.Process = func(input string) string {
			return input
		}
	case "l":
		myRule.Process = func(input string) string {
			return strings.ToLower(input)
		}
	case "u":
		myRule.Process = func(input string) string {
			return strings.ToUpper(input)
		}
	case "c":
		myRule.Process = func(input string) string {
			if len(input) <= 1 {
				return strings.ToUpper(input)
			}
			lower := strings.ToLower(input)
			return strings.ToUpper(string(lower[0])) + lower[1:]
		}
	case "C":
		myRule.Process = func(input string) string {
			if len(input) <= 1 {
				return strings.ToLower(input)
			}
			upper := strings.ToUpper(input)
			return strings.ToLower(string(upper[0])) + upper[1:]
		}
	case "t":
		myRule.Process = func(input string) string {
			var result []rune
			for _, r := range input {
				if unicode.IsLower(r) {
					result = append(result, unicode.ToUpper(r))
				} else if unicode.IsUpper(r) {
					result = append(result, unicode.ToLower(r))
				} else {
					result = append(result, r)
				}
			}
			return string(result)
		}
	case "q":
		myRule.Process = func(input string) string {
			var result strings.Builder
			for _, r := range input {
				result.WriteRune(r)
				result.WriteRune(r)
			}
			return result.String()
		}
	case "r":
		myRule.Process = func(input string) string {
			runes := []rune(input)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return string(runes)
		}
	case "k":
		myRule.Process = func(input string) string {
			if len([]rune(input)) < 2 {
				return input
			}
			runes := []rune(input)
			runes[0], runes[1] = runes[1], runes[0]
			return string(runes)
		}
	case "K":
		myRule.Process = func(input string) string {
			if len([]rune(input)) < 2 {
				return input
			}
			runes := []rune(input)
			runes[len(runes)-1], runes[len(runes)-2] = runes[len(runes)-2], runes[len(runes)-1]
			return string(runes)
		}
	case "d":
		myRule.Process = func(input string) string {
			return input + input
		}
	case "f":
		myRule.Process = func(input string) string {
			runes := []rune(input)
			// Reverse the string
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return input + string(runes)
		}
	case "{":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			return input[1:] + string(input[0])
		}
	case "}":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			return string(input[len(input)-1]) + input[:len(input)-1]
		}
	case "[":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			return input[1:]
		}
	case "]":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			return input[:len(input)-1]
		}
	case "E":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			input = strings.ToLower(input)
			runes := []rune(input)
			runes[0] = unicode.ToUpper(runes[0])
			for i := 1; i < len(runes); i++ {
				if runes[i-1] == ' ' {
					runes[i] = unicode.ToUpper(runes[i])
				}
			}
			return string(runes)
		}
	case "T":
		myRule.Process = func(input string) string {
			if len(input) == 0 || myRule.NumericParameter1 >= len([]rune(input)) {
				return input
			}
			runes := []rune(input)
			if unicode.IsLower(runes[myRule.NumericParameter1]) {
				runes[myRule.NumericParameter1] = unicode.ToUpper(runes[myRule.NumericParameter1])
			} else if unicode.IsUpper(runes[myRule.NumericParameter1]) {
				runes[myRule.NumericParameter1] = unicode.ToLower(runes[myRule.NumericParameter1])
			}
			return string(runes)
		}
	case "p":
		myRule.Process = func(input string) string {
			var result strings.Builder
			for i := 0; i <= myRule.NumericParameter1; i++ {
				result.WriteString(input)
			}
			return result.String()
		}
	case "D":
		myRule.Process = func(input string) string {
			if len(input) == 0 || myRule.NumericParameter1 > len(input)-1 {
				return input
			}
			return input[:myRule.NumericParameter1] + input[myRule.NumericParameter1+1:]
		}
	case "z":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			return strings.Repeat(string(input[0]), myRule.NumericParameter1) + input
		}
	case "Z":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			return input + strings.Repeat(string(input[len(input)-1]), myRule.NumericParameter1)
		}
	case "'":
		myRule.Process = func(input string) string {
			if len(input) == 0 || myRule.NumericParameter1 > len(input) {
				return input
			}
			return input[:myRule.NumericParameter1]
		}
	case "s":
		myRule.Process = func(input string) string {
			return strings.ReplaceAll(input, myRule.Parameter1, myRule.Parameter2)
		}
	case "S": // Replace nth occurrence of character. S0ab replace the first 'a' with 'b'. SAs$ replace the 10th s with $
		myRule.Process = func(input string) string {
			if n := myRule.NumericParameter1; n >= 0 && myRule.Parameter2 != "" && myRule.Parameter3 != "" {
				var pos, count int
				for {
					if idx := strings.Index(input[pos:], myRule.Parameter2); idx >= 0 {
						if count == n {
							pos += idx
							return input[:pos] + myRule.Parameter3 + input[pos+len(myRule.Parameter2):]
						}
						count++
						pos += idx + len(myRule.Parameter2)
					} else {
						break
					}
				}
			}
			return input
		}
	case "$":
		myRule.Process = func(input string) string {
			return input + myRule.Parameter1
		}
	case "^":
		myRule.Process = func(input string) string {
			return myRule.Parameter1 + input
		}
	case "y":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 == 0 {
				return input
			}
			if myRule.NumericParameter1 > len(input) {
				return input
			}
			return input[:myRule.NumericParameter1] + input
		}

	case "Y":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 == 0 {
				return input
			}
			if myRule.NumericParameter1 > len(input) {
				return input + input
			}
			return input + input[len(input)-myRule.NumericParameter1:]
		}

	case "L":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len(input) {
				return input
			}
			char := input[myRule.NumericParameter1]
			modifiedChar := char << 1
			return input[:myRule.NumericParameter1] + string(modifiedChar) + input[myRule.NumericParameter1+1:]
		}

	case "R":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len(input) {
				return input
			}
			char := input[myRule.NumericParameter1]
			modifiedChar := char >> 1
			return input[:myRule.NumericParameter1] + string(modifiedChar) + input[myRule.NumericParameter1+1:]
		}

	case "-":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len(input) {
				return input
			}
			char := input[myRule.NumericParameter1]
			modifiedChar := char - 1
			return input[:myRule.NumericParameter1] + string(modifiedChar) + input[myRule.NumericParameter1+1:]
		}

	case "+":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len(input) {
				return input
			}
			char := input[myRule.NumericParameter1]
			modifiedChar := char + 1
			return input[:myRule.NumericParameter1] + string(modifiedChar) + input[myRule.NumericParameter1+1:]
		}

	case "@":
		myRule.Process = func(input string) string {
			return strings.ReplaceAll(input, string(myRule.Parameter1[0]), "")
		}

	case ".":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1+1 >= len(input) {
				return input
			}
			modified := input[:myRule.NumericParameter1] + string(input[myRule.NumericParameter1+1]) + input[myRule.NumericParameter1+1:]
			return modified[:len(modified)-1]
		}

	case ",":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len(input) || myRule.NumericParameter1 <= 0 {
				return input
			}
			modified := input[:myRule.NumericParameter1] + string(input[myRule.NumericParameter1-1]) + input[myRule.NumericParameter1+1:]
			return modified
		}

	case "e":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			lower := strings.ToLower(input)
			result := strings.ToUpper(string(lower[0])) + lower[1:]
			for i := 1; i < len(result); i++ {
				if i+1 < len(result) && result[i] == myRule.Parameter1[0] {
					result = result[:i+1] + strings.ToUpper(string(result[i+1])) + result[i+2:]
				}
			}
			return result
		}

	case "i":
		myRule.Process = func(input string) string {
			if len(input) == 0 || myRule.NumericParameter1 > len(input) {
				return input
			}
			return input[:myRule.NumericParameter1] + myRule.Parameter2 + input[myRule.NumericParameter1:]
		}

	case "O":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len(input) || len(input) == 0 {
				return input
			}
			if myRule.NumericParameter1+myRule.NumericParameter2 < len(input) {
				return input[:myRule.NumericParameter1] + input[myRule.NumericParameter1+myRule.NumericParameter2:]
			}
			return input
		}

	case "o":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 < 0 || len(input) == 0 || myRule.NumericParameter1+len(myRule.Parameter2)-1 >= len(input) {
				return input
			}
			modified := input[:myRule.NumericParameter1] + myRule.Parameter2
			if myRule.NumericParameter1+len(myRule.Parameter2) < len(input) {
				modified += input[myRule.NumericParameter1+len(myRule.Parameter2):]
			}
			return modified
		}
	case "*":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 >= len([]rune(input)) || myRule.NumericParameter2 >= len([]rune(input)) || len(input) == 0 {
				return input
			}
			runes := []rune(input)
			runes[myRule.NumericParameter1], runes[myRule.NumericParameter2] = runes[myRule.NumericParameter2], runes[myRule.NumericParameter1]
			return string(runes)
		}
	case "x":
		myRule.Process = func(input string) string {
			if len(input) == 0 || myRule.NumericParameter1 >= len(input) {
				return input
			}
			if myRule.NumericParameter1+myRule.NumericParameter2 <= len(input) {
				input = input[myRule.NumericParameter1 : myRule.NumericParameter1+myRule.NumericParameter2]
			}
			return input
		}

	case "<":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 < len(input) {
				return input
			}
			return ""
		}

	case ">":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 > len(input) {
				return input
			}
			return ""
		}

	case "_":
		myRule.Process = func(input string) string {
			if myRule.NumericParameter1 == len(input) {
				return ""
			}
			return input
		}

	case "!":
		myRule.Process = func(input string) string {
			if strings.Contains(input, myRule.Parameter1) {
				return ""
			}
			return input
		}

	case "/":
		myRule.Process = func(input string) string {
			if !strings.Contains(input, myRule.Parameter1) {
				return ""
			}
			return input
		}

	case "3":
		myRule.Process = func(input string) string {
			if len(input) == 0 {
				return input
			}
			instance := -1
			runes := []rune(input)
			for i, char := range runes {
				if instance == myRule.NumericParameter1 {
					if unicode.IsLower(char) {
						runes[i] = unicode.ToUpper(char)
					} else if unicode.IsUpper(char) {
						runes[i] = unicode.ToLower(char)
					}
					break
				}
				if char == rune(myRule.Parameter2[0]) {
					instance++
				}
			}
			return string(runes)
		}
	}

	return myRule, nil
}

// PrintFormat renders a rule back to hashcat syntax
func (r *Rule) PrintFormat() string {
	s1 := strings.ReplaceAll(r.Parameter1, "\t", "\\x09")
	s2 := strings.ReplaceAll(r.Parameter2, "\t", "\\x09")
	s3 := strings.ReplaceAll(r.Parameter3, "\t", "\\x09")
	pc, _ := ParameterCountRule(r.Function)
	switch pc {
	case 0:
		return r.Function
	case 1:
		return r.Function + s1
	case 2:
		if len(r.Parameter1) > 1 || len(r.Parameter2) > 1 {
			return fmt.Sprintf("%s/%s/%s", r.Function, s1, s2)
		}
		return r.Function + s1 + s2
	case 3:
		if len(r.Parameter1) > 1 || len(r.Parameter2) > 1 || len(r.Parameter3) > 1 {
			return fmt.Sprintf("%s/%s/%s/%s", r.Function, s1, s2, s3)
		}
		return r.Function + s1 + s2 + s3
	}
	return ""
}

// FormatAllRules joins multiple rules with delimiter
func FormatAllRules(all []Rule, delim ...string) string {
	d := "\t"
	if len(delim) > 0 {
		d = delim[0]
	}
	var b strings.Builder
	for i, r := range all {
		b.WriteString(r.PrintFormat())
		if i < len(all)-1 {
			b.WriteString(d)
		}
	}
	return b.String()
}

// ConvertFromHashcat converts a line of hashcat compatible rules to an array of Rule objects.
// Done by first converting to TSV and then using ParseTSVRules to convert to Rule objects.
func ConvertFromHashcat(lineCounter uint64, rawLine string) ([]Rule, error) {
	if len(rawLine) == 0 {
		return nil, fmt.Errorf("empty rule on line [%d]", lineCounter)
	}

	// Sets of each rawLine width
	singleWide := ":lucCtrdf{}[]kKqE"
	doubleWide := "TpDZz$^<>_'!/@-+yYLR.,e"
	tripleWide := "sxOoi*3"
	quadrupleWide := "S"

	var formattedRule strings.Builder
	offset := 0

	for offset < len(rawLine) {
		baseRule := rawLine[offset]

		switch {
		case baseRule == ' ':
			offset++

		case strings.Contains(singleWide, string(baseRule)):
			formattedRule.WriteString(rawLine[offset:offset+1] + "\t")
			offset++

		case strings.Contains(doubleWide, string(baseRule)):
			if offset+3 < len(rawLine) && rawLine[offset+1:offset+3] == "\\x" {
				if offset+5 > len(rawLine) {
					return nil, fmt.Errorf("missing hex parameters on line [%d]: \"%s\"", lineCounter, rawLine)
				}
				formattedRule.WriteString(rawLine[offset:offset+5] + "\t")
				offset += 5
			} else {
				if offset+2 > len(rawLine) {
					return nil, fmt.Errorf("missing rule parameters on line [%d]: \"%s\"", lineCounter, rawLine)
				}
				formattedRule.WriteString(rawLine[offset:offset+2] + "\t")
				offset += 2
			}

		case strings.Contains(tripleWide, string(baseRule)):
			if offset+9 <= len(rawLine) && rawLine[offset+1:offset+3] == "\\x" && rawLine[offset+5:offset+7] == "\\x" {
				formattedRule.WriteString(rawLine[offset:offset+9] + "\t")
				offset += 9
			} else if offset+6 <= len(rawLine) && (rawLine[offset+1:offset+3] == "\\x" || rawLine[offset+2:offset+4] == "\\x") {
				formattedRule.WriteString(rawLine[offset:offset+6] + "\t")
				offset += 6
			} else {
				if offset+3 > len(rawLine) {
					return nil, fmt.Errorf("missing rule parameters on line [%d]: \"%s\"", lineCounter, rawLine)
				}
				formattedRule.WriteString(rawLine[offset:offset+3] + "\t")
				offset += 3
			}

		case strings.Contains(quadrupleWide, string(baseRule)):
			if offset+12 <= len(rawLine) && rawLine[offset+1:offset+3] == "\\x" && rawLine[offset+5:offset+7] == "\\x" && rawLine[offset+9:offset+11] == "\\x" {
				formattedRule.WriteString(rawLine[offset:offset+12] + "\t")
				offset += 12
			} else {
				if offset+4 > len(rawLine) {
					return nil, fmt.Errorf("missing rule parameters on line [%d]: \"%s\"", lineCounter, rawLine)
				}
				formattedRule.WriteString(rawLine[offset:offset+4] + "\t")
				offset += 4
			}

		case baseRule == '#':
			break // Exit loop on comments

		default:
			return nil, fmt.Errorf("unknown rule function \"%c\" on line [%d]", baseRule, lineCounter)
		}
	}
	// Remove last tab if exists
	TSVResult := strings.TrimSuffix(formattedRule.String(), "\t")
	return ParseTSVRules(lineCounter, TSVResult)
}
