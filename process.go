package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func applyRuleCPU(rules []Rule, input []string) []string {
	out := make([]string, len(input))
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i, w := range input {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int, w string) {
			defer wg.Done()
			defer func() { <-sem }()
			for _, rule := range rules {
				w = rule.Process(w)
			}
			if len(rules) != 1 || rules[0].Function != ":" {
				out[i] = w
			}
		}(i, w)
	}
	wg.Wait()
	return out
}

// generateCombinations generates all possible combinations of words from the dictionary for length k
func generateCombinations(dict []string, targetLength int) [][]string {
	var res [][]string
	used := make([]bool, len(dict)) // Track used elements
	var backtrack func(current []string)
	backtrack = func(current []string) {
		if len(current) == targetLength {
			// Save a copy of the current permutation
			combination := make([]string, targetLength)
			copy(combination, current)
			res = append(res, combination)
			return
		}
		for i := 0; i < len(dict); i++ {
			if !used[i] {
				// Mark element as used and add to current
				used[i] = true
				current = append(current, dict[i])

				backtrack(current)

				// Backtrack: remove element and mark as unused
				current = current[:len(current)-1]
				used[i] = false
			}
		}
	}
	backtrack([]string{})
	return res
}

// generateRuledCombinations generates all possible combinations of words from the dictionary for length k with 1 ruled word in each position, this is AI generated based on generateCombinations.
// start AI
// start AI
// start AI
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func removeStringsPresentIn(slice, toRemove []string) []string {
	removeSet := make(map[string]bool)
	for _, s := range toRemove {
		removeSet[s] = true
	}
	result := []string{}
	for _, s := range slice {
		if !removeSet[s] {
			result = append(result, s)
		}
	}
	return result
}

type positionSet struct {
	ruledPositions   []int
	unruledPositions []int
}

func generatePositionCombinations(totalLength, k int) []positionSet {
	var combs []positionSet
	if k < 0 || k > totalLength {
		return combs
	}

	indices := make([]int, k)
	for i := range indices {
		indices[i] = i
	}

	for {
		unruled := make([]int, 0, totalLength-k)
		posMap := make(map[int]bool, k)
		for _, idx := range indices {
			posMap[idx] = true
		}
		for i := 0; i < totalLength; i++ {
			if !posMap[i] {
				unruled = append(unruled, i)
			}
		}
		ruled := make([]int, k)
		copy(ruled, indices)
		combs = append(combs, positionSet{
			ruledPositions:   ruled,
			unruledPositions: unruled,
		})

		i := k - 1
		for i >= 0 && indices[i] == totalLength-k+i {
			i--
		}
		if i < 0 {
			break
		}
		indices[i]++
		for j := i + 1; j < k; j++ {
			indices[j] = indices[j-1] + 1
		}
	}

	return combs
}

func generatePermutations(arr []string, length int) [][]string {
	if length == 0 {
		return [][]string{{}}
	}
	n := len(arr)
	if n < length {
		return nil
	}
	var res [][]string
	used := make([]bool, n)
	var backtrack func(current []string)
	backtrack = func(current []string) {
		if len(current) == length {
			temp := make([]string, length)
			copy(temp, current)
			res = append(res, temp)
			return
		}
		for i := 0; i < n; i++ {
			if !used[i] {
				used[i] = true
				current = append(current, arr[i])
				backtrack(current)
				current = current[:len(current)-1]
				used[i] = false
			}
		}
	}
	backtrack([]string{})
	return res
}

func generateRuledCombinations(dict []string, ruledDict []string, targetLength int) [][]string {
	dict = removeDuplicates(dict)
	ruledDict = removeDuplicates(ruledDict)
	ruledDict = removeStringsPresentIn(ruledDict, dict)

	A := dict
	B := ruledDict
	nA := len(A)
	nB := len(B)

	var res [][]string

	// Start from k=1 to ensure at least one ruledDict word is included
	for k := 1; k <= targetLength && k <= nB; k++ {
		if targetLength-k > nA {
			continue
		}

		positionCombs := generatePositionCombinations(targetLength, k)
		permsB := generatePermutations(B, k)
		permsA := generatePermutations(A, targetLength-k)

		for _, posSet := range positionCombs {
			for _, permB := range permsB {
				for _, permA := range permsA {
					comb := make([]string, targetLength)
					for idx, pos := range posSet.ruledPositions {
						comb[pos] = permB[idx]
					}
					for idx, pos := range posSet.unruledPositions {
						comb[pos] = permA[idx]
					}
					res = append(res, comb)
				}
			}
		}
	}

	return res
}

// END AI
// END AI
// END AI

// removeMatchingWords removes overlapping words between targetDictWithRule and targetFile
func removeMatchingWords(targetDictWithRule, targetFile []string) []string {
	removeWords := make(map[string]struct{}, len(targetFile))
	for _, word := range targetFile {
		removeWords[word] = struct{}{}
	}
	result := make([]string, 0, len(targetDictWithRule))
	for _, word := range targetDictWithRule {
		if _, exists := removeWords[word]; !exists {
			result = append(result, word)
		}
	}
	return result
}

func calculateKeyspace(targetWordlist []string, cli CLI) uint64 {
	n := len(targetWordlist)
	min := cli.MinTarget
	max := cli.MaxTarget
	var total uint64

	permutations := func(n, k int) uint64 {
		if k > n || n < 0 || k < 0 {
			return 0
		}
		result := uint64(1)
		for i := 0; i < k; i++ {
			result *= uint64(n - i)
		}
		return result
	}

	validWordlists := filterByValidWordlistTarget(cli.Wordlists, cli)

	if cli.TargetRules != "" {
		targetRuleFile, err := loadRulesFast(cli.TargetRules)
		if err != nil {
			log.Fatalf("loading target rules: %v", err)
		}

		for _, ro := range targetRuleFile {
			if len(ro.RuleLine) == 1 && ro.RuleLine[0].Function == ":" {
				continue
			}

			newWords := applyRuleCPU(ro.RuleLine, targetWordlist)
			m := len(newWords)
			baseSetSize := n + m

			if cli.SelfCombination {
				for i := min; i <= max; i++ {
					total += permutations(baseSetSize, i)
				}
			}

			if len(validWordlists) > 0 {
				var T uint64
				for i := min; i <= max; i++ {
					perm := permutations(baseSetSize, i)
					T += perm * uint64(i+1)
				}

				for _, wordlist := range validWordlists {
					if cli.WordlistRules != "" {
						rules, err := loadRulesFast(cli.WordlistRules)
						if err != nil {
							log.Fatalf("loading wordlist rules: %v", err)
						}

						words, err := readWordlist(wordlist)
						if err != nil {
							log.Fatalf("reading wordlist %q: %v", wordlist, err)
						}

						var totalWords uint64
						for _, rule := range rules {
							filtered := applyRuleCPU(rule.RuleLine, words)
							totalWords += uint64(len(filtered))
						}
						total += totalWords * T
					} else {
						count, err := countLines(wordlist)
						if err != nil {
							log.Fatalf("counting lines in %q: %v", wordlist, err)
						}
						total += uint64(count) * T
					}
				}
			}
		}
	} else {
		baseSetSize := n

		if cli.SelfCombination {
			for i := min; i <= max; i++ {
				total += permutations(baseSetSize, i)
			}
		}

		if len(validWordlists) > 0 {
			var T uint64
			for i := min; i <= max; i++ {
				perm := permutations(baseSetSize, i)
				T += perm * uint64(i+1)
			}

			for _, wordlist := range validWordlists {
				if cli.WordlistRules != "" {
					rules, err := loadRulesFast(cli.WordlistRules)
					if err != nil {
						log.Fatalf("loading wordlist rules: %v", err)
					}

					words, err := readWordlist(wordlist)
					if err != nil {
						log.Fatalf("reading wordlist %q: %v", wordlist, err)
					}

					var totalWords uint64
					for _, rule := range rules {
						filtered := applyRuleCPU(rule.RuleLine, words)
						totalWords += uint64(len(filtered))
					}
					total += totalWords * T
				} else {
					count, err := countLines(wordlist)
					if err != nil {
						log.Fatalf("counting lines in %q: %v", wordlist, err)
					}
					total += uint64(count) * T
				}
			}
		}
	}

	return total
}

func readWordlist(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var words []string
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			if len(line) > 0 {
				words = append(words, checkForHex(strings.TrimSuffix(line, "\n")))
			}
			break
		}
		if err != nil {
			return nil, err
		}
		words = append(words, checkForHex(strings.TrimSuffix(line, "\n")))
	}
	return words, nil
}

func countLines(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func filterByValidWordlistTarget(wordlists []string, cli CLI) []string {
	var validWordlists []string
	debug := cli.Debug
	for _, wordlist := range wordlists {
		// Skip directories and non-existent stuff
		if isDir, dirErr := isDirectory(wordlist); isDir || dirErr != nil {
			if !isDir {
				if debug {
					log.Printf("%s. It will be skipped", dirErr.Error())
				}
				continue
			}
			loadedFiles := 0
			err := filepath.Walk(wordlist,
				func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						//log.Printf("Loading %s", path)
						validWordlists = append(validWordlists, path)
						loadedFiles += 1
					}
					return nil
				})
			if err != nil {
				log.Println(err)
			}
			if debug {
				log.Printf("Loaded %d files from %s recursively", loadedFiles, wordlist)
			}
		}
		if valid, fileErr := isReadable(wordlist); !valid || fileErr != nil {
			if fileErr != nil {
				if debug {
					log.Printf("%s. It will be skipped", fileErr.Error())
				}
			} else {
				if debug {
					log.Printf("%s is invalid. It will be skipped", wordlist)
				}
			}
			continue
		}
		validWordlists = append(validWordlists, wordlist)
	}
	return validWordlists
}

func processAllWordlists(targetFile []string, ruledFile []string, cli CLI) {
	// Check all wordlists

	validWordlists := filterByValidWordlistTarget(cli.Wordlists, cli)
	if cli.Debug {
		log.Printf("Loaded %d wordlists", len(validWordlists))
	}
	skipCounter := uint64(0)

	for i := cli.MinTarget; i <= cli.MaxTarget; i++ {
		if cli.Debug {
			log.Printf("Processing length %d", i)
		}
		combinations := [][]string{}
		if len(ruledFile) > 0 {
			combinations = generateRuledCombinations(targetFile, ruledFile, i)
		} else {
			combinations = generateCombinations(targetFile, i)
		}
		if len(combinations) == 0 {
			continue
		}

		if cli.SelfCombination {
			processWordlist(&combinations, "", cli, skipCounter)
		}

		if len(validWordlists) > 0 {
			for _, wordlist := range validWordlists {
				if cli.Debug {
					log.Printf("Processing %s", wordlist)
				}
				processWordlist(&combinations, wordlist, cli, skipCounter)
			}
		}
	}
	return
}

// loop through lines per file
func processWordlist(combinations *[][]string, wordlist string, cli CLI, skipCounter uint64) {
	// setup writer with a buffered channel (stdout or file)
	var err error
	var (
		writer io.Writer
		outF   *os.File
	)
	if cli.OutputFile != "" {
		outF, err = os.OpenFile(cli.OutputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating output file %q: %v\n", cli.OutputFile, err)
			os.Exit(1)
		}
		defer outF.Close()
		writer = outF
	} else {
		writer = os.Stdout
	}

	wr := bufio.NewWriterSize(writer, 1<<20) // 1 MiB buffer
	outputChannel := make(chan string, 10_000)
	var wgOutput sync.WaitGroup
	wgOutput.Add(1)
	go func() {
		defer wgOutput.Done()
		for line := range outputChannel {
			wr.WriteString(line)
			wr.WriteByte('\n')
		}
		wr.Flush()
	}()

	// if no wordlist is provided print the combinations without modification
	if wordlist == "" {
		for _, arr := range *combinations {
			outputChannel <- strings.Join(arr, cli.Separator)
		}
		close(outputChannel)
		wgOutput.Wait()
		return
	}

	// preload entire wordlist once with buffered reader
	file, err := os.Open(wordlist)
	if err != nil {
		log.Fatalf("opening wordlist %s: %v", wordlist, err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 1<<20) // 1 MiB buffer
	var fullWL []string
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			if len(line) > 0 {
				fullWL = append(fullWL, checkForHex(strings.TrimSuffix(line, "\n"))) // run through checkForHex()
			}
			break
		}
		if err != nil {
			log.Fatalf("reading wordlist %s: %v", wordlist, err)
		}
		fullWL = append(fullWL, checkForHex(strings.TrimSuffix(line, "\n"))) // run through checkForHex()
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())

	if cli.WordlistRules != "" {
		rules, err := loadRulesFast(cli.WordlistRules)
		if err != nil {
			log.Fatal(err)
		}

		skipRuleCount := 0
		skipRules := (int(cli.Skip) - int(skipCounter)) % (len(fullWL) * len(*combinations))

		for _, ro := range rules {
			if skipRuleCount < skipRules {
				skipRuleCount++
				skipCounter += uint64(len(fullWL)) * uint64(len(*combinations))
				continue
			}
			if cli.Debug {
				log.Printf("Running wordlist rule: %s", FormatAllRules(ro.RuleLine))
			}
			filtered := applyRuleCPU(ro.RuleLine, fullWL)

			skipWordCount := 0
			skipWords := (int(cli.Skip) - int(skipCounter)) % len(*combinations)
			for _, wordLine := range filtered {
				if skipWordCount < skipWords {
					skipWordCount++
					skipCounter += uint64(len(*combinations))
					continue
				}

				sem <- struct{}{}
				wg.Add(1)
				go func(combos *[][]string, word string) {
					defer wg.Done()
					defer func() { <-sem }()
					for _, combo := range *combos {
						for i := 0; i <= len(combo); i++ {
							newCombo := make([]string, len(combo)+1)
							copy(newCombo, combo[:i])
							newCombo[i] = word
							copy(newCombo[i+1:], combo[i:])
							outputChannel <- strings.Join(newCombo, cli.Separator)
						}
					}
				}(combinations, wordLine)
			}
		}

		// no-rules
	} else {
		skipWordCount := 0
		skipWords := (int(cli.Skip) - int(skipCounter)) % len(*combinations)
		for _, wordLine := range fullWL {
			if skipWordCount < skipWords {
				skipWordCount++
				skipCounter += uint64(len(*combinations))
				continue
			}

			sem <- struct{}{}
			wg.Add(1)
			go func(combos *[][]string, word string) {
				defer wg.Done()
				defer func() { <-sem }()
				for _, combo := range *combos {
					for i := 0; i <= len(combo); i++ {
						newCombo := make([]string, len(combo)+1)
						copy(newCombo, combo[:i])
						newCombo[i] = word
						copy(newCombo[i+1:], combo[i:])
						outputChannel <- strings.Join(newCombo, cli.Separator)
					}
				}
			}(combinations, wordLine)
		}
	}

	// wait for all combo goroutines, close output chan, then wait for writer
	wg.Wait()
	close(outputChannel)
	wgOutput.Wait()
}
