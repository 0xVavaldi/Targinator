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
			out[i] = w
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

func removeMatchingWords(targetDictWithRule, targetFile []string) []string {
	// Create a map for O(1) lookups
	removeWords := make(map[string]struct{}, len(targetFile))
	for _, word := range targetFile {
		removeWords[word] = struct{}{}
	}

	// Filter the dictionary
	result := make([]string, 0, len(targetDictWithRule))
	for _, word := range targetDictWithRule {
		if _, exists := removeWords[word]; !exists {
			result = append(result, word)
		}
	}

	return result
}

func binomialCoeficient(n, k int) uint64 {
	if k < 0 || k > n {
		return 0
	}
	if k == 0 || k == n {
		return 1
	}
	k = min(k, n-k)
	res := uint64(1)
	for i := 1; i <= k; i++ {
		res = res * uint64(n-i+1) / uint64(i)
	}
	return res
}

func calculateKeyspace(targetWordlist []string, cli CLI) int {
	wordlistLines := 0
	numTargetRules := 0
	numWordlistRules := 0
	wordlistKeyspace := 0
	totalCombinations := 0

	var wordlists []string
	if cli.TargetRules != "" {
		targetRules, err := loadRulesFast(cli.TargetRules)
		if err != nil {
			log.Fatal(err)
		}
		numTargetRules = len(targetRules)
	}
	// Count the number of rules applied to the wordlist
	if cli.WordlistRules != "" {
		wordlistRules, err := loadRulesFast(cli.WordlistRules)
		if err != nil {
			log.Fatal(err)
		}
		numWordlistRules = len(wordlistRules)
	}
	if len(cli.Wordlists) > 0 {
		wordlists = filterByValidWordlistTarget(cli.Wordlists, cli)
		if len(wordlists) == 0 {
			log.Fatal("No valid wordlist files specified")
		}
		for _, wordlist := range wordlists {
			var err error
			wordlistLines, err = lineCounter(wordlist)
			if err != nil {
				log.Fatal(err)
			}
			if numWordlistRules > 0 {
				wordlistKeyspace += wordlistLines * numWordlistRules
			} else {
				wordlistKeyspace += wordlistLines
			}
		}
	}

	for i := cli.MinTarget; i <= cli.MaxTarget; i++ {
		if cli.Debug {
			log.Printf("Processing length %d", i)
		}
		combinations := generateCombinations(targetWordlist, i)
		// i+1 is the possible insertion points
		if cli.SelfCombination {
			totalCombinations += len(combinations)
		}
		totalCombinations += len(combinations) * (wordlistKeyspace) * (i + 1)
	}
	if numTargetRules > 0 {
		totalCombinations *= numTargetRules + 1
	}
	return totalCombinations
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

func processAllWordlists(targetFile []string, cli CLI) {
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
		combinations := generateCombinations(targetFile, i)

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
