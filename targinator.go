package main

// aka flaginator

// This tool, originally dubbed PermutationFlagg and later renamed ValdiComb and now
// renamed to Targinator is a combinator tool combining wordlists and targeted
// values and is written of celebration of Flagg's work (R.I.P. 2025-03). He was
// a valued member of the HashMob and Hashpwn community and a good friend to us
// all. May you rest in peace.
//
// By combining specific aspects of a password as well as a targeted wordlist we
// hit passwords that combine both personal aspects and general aspects of a
// password. Authored by Vavaldi with the original idea provided by Flagg in
// 2021.
import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/alecthomas/kong"
	"github.com/schollz/progressbar/v3"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type ruleObj struct {
	ID           uint64
	Fitness      uint64
	LastFitness  uint64
	RuleLine     []Rule
	PreProcessed bool
	Hits         map[uint64]struct{}
	HitsMutex    sync.Mutex
}

type lineObj struct {
	ID   uint64
	line string
}

type CLI struct {
	Target      string   `arg:"" help:"Path to target data file (must fit in memory)"`
	Wordlists   []string `arg:"" help:"Path to wordlist files or directory"`
	MinTarget   int      `optional:"" short:"m" help:"Minimum target occurrences" default:"1"`
	MaxTarget   int      `optional:"" short:"x" help:"Maximum target occurrences" default:"3"`
	TargetRules string   `optional:"" short:"t" help:"Apply rules file to Target" default:""`
	Separator   string   `optional:"" short:"s" help:"Word Separator" default:""`
	OutputFile  string   `optional:"" short:"o" help:"Output File" default:""`
}

func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}

func isReadable(path string) (bool, error) {
	_, err := os.Open(path)
	if err != nil {
		return false, err
	}
	return true, nil
}

func loadTargetFile(path string) ([]string, error) {
	// if file is dir, exit
	if isDir, err := isDirectory(path); isDir || err != nil {
		if isDir {
			return []string{path}, errors.New(fmt.Sprintf("target file %s is a directory", path))
		}
		return []string{}, err
	}
	// open it
	file, err := os.Open(path)
	if err != nil {
		return []string{}, err
	}
	// prepare to close it
	defer func(file *os.File) {
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
	}(file)
	// read it
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

func timer(name string) func() {
	start := time.Now()
	return func() {
		log.Printf("\n%s took %v\n", name, time.Since(start))
	}
}

func lineCounter(inputFile string) (int, error) {
	file, err := os.Open(inputFile)
	if err != nil {
		return 0, err
	}

	defer file.Close()
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := file.Read(buf)
		count += bytes.Count(buf[:c], lineSep)
		switch {
		case err == io.EOF:
			return count, nil
		case err != nil:
			return count, err
		}
	}
}

func loadRulesFast(inputFile string) ([]ruleObj, error) {
	defer timer("loadRules")()
	ruleLines, _ := lineCounter(inputFile)
	ruleQueue := make(chan lineObj, 100)
	ruleOutput := make(chan ruleObj, ruleLines)
	threadCount := runtime.NumCPU()
	wg := sync.WaitGroup{}

	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rawLineObj := range ruleQueue {
				ruleObject, _ := ConvertFromHashcat(rawLineObj.ID, rawLineObj.line)
				hits := make(map[uint64]struct{})
				ruleOutput <- ruleObj{rawLineObj.ID, 0, 0, ruleObject, false, hits, sync.Mutex{}}
			}
		}()
	}

	ruleBar := progressbar.NewOptions(ruleLines,
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowDescriptionAtLineEnd(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionThrottle(500*time.Millisecond),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetWidth(25),
		progressbar.OptionShowIts(),
		progressbar.OptionShowCount(),
	)
	file, err := os.Open(inputFile)
	if err != nil {
		return []ruleObj{}, errors.New(fmt.Sprintf("Error opening file: %s", err))
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	ruleLineCounter := uint64(1)

	for scanner.Scan() {
		lineObject := new(lineObj)
		lineObject.ID = ruleLineCounter
		lineObject.line = scanner.Text()
		if len(lineObject.line) == 0 {
			continue
		}
		ruleQueue <- *lineObject
		ruleLineCounter++
		if ruleLineCounter%10000 == 0 {
			ruleBar.Add(10000)
		}
	}
	ruleBar.Add(int(ruleLineCounter))
	close(ruleQueue)
	go func() {
		wg.Wait()
		close(ruleOutput)
	}()

	// Step 1: Consume the channel into a slice
	var sortedRules []ruleObj
	for obj := range ruleOutput {
		sortedRules = append(sortedRules, obj)
	}

	// Step 2: Sort the slice by ID
	sort.Slice(sortedRules, func(i, j int) bool {
		return sortedRules[i].ID < sortedRules[j].ID
	})
	ruleBar.Finish()
	ruleBar.Close()
	return sortedRules, nil
}

func generateCombinations(arr []string, arr2 []string, k int) [][]string {
	var res [][]string
	var backtrack func(start int, current []string)

	backtrack = func(start int, current []string) {
		if len(current) == k {
			combination := make([]string, k)
			copy(combination, current)
			res = append(res, combination)
			return
		}

		for i := start; i < len(arr); i++ {
			current = append(current, arr[i])
			backtrack(i+1, current)
			current = current[:len(current)-1]
		}
	}

	backtrack(0, []string{})
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

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("Targinator"),
		kong.Description("A combinator application using a targeted and generic wordlist"),
		kong.UsageOnError(),
	)

	// Get the target list and exit if invalid
	log.Println("Loading Target File:", cli.Target)
	targetFile, tarErr := loadTargetFile(cli.Target)
	if tarErr != nil {
		log.Fatal(tarErr)
		return
	}

	process_all_wordlists(targetFile, targetFile, cli, true)
	if cli.TargetRules != "" { // Run with GPU-accelerated target rules
		targetRuleFile, tarErr := loadRulesFast(cli.TargetRules)
		if tarErr != nil {
			log.Fatal(tarErr)
			return
		}

		processBar := progressbar.NewOptions(len(targetFile)*len(targetRuleFile),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionShowDescriptionAtLineEnd(),
			progressbar.OptionSetRenderBlankState(true),
			progressbar.OptionThrottle(1000*time.Millisecond),
			progressbar.OptionShowElapsedTimeOnFinish(),
			progressbar.OptionSetWidth(25),
			progressbar.OptionShowIts(),
			progressbar.OptionShowCount(),
		)

		originalDictGPUArray := make([]byte, len(targetFile)*32)
		originalDictGPUArrayLengths := make([]uint32, len(targetFile))
		for j, word := range targetFile {
			copy(originalDictGPUArray[j*32:], word)
			originalDictGPUArrayLengths[j] = uint32(len(word))
		}

		//sort.Slice(originalHashes, func(i, j int) bool { return originalHashes[i] < originalHashes[j] })
		//sort.Slice(compareDictHashes, func(i, j int) bool { return compareDictHashes[i] < compareDictHashes[j] })
		d_originalDict, d_originalDictLengths := CUDAInitialize(&originalDictGPUArray, &originalDictGPUArrayLengths, len(targetFile))
		for _, targetRule := range targetRuleFile {
			log.Printf("Running rule: %s", FormatAllRules(targetRule.RuleLine))
			d_processedDict, d_processedDictLengths := CUDAInitialize(&originalDictGPUArray, &originalDictGPUArrayLengths, len(targetFile))
			defer CUDADeinitialize(d_processedDict, d_processedDictLengths)
			targetDictWithRule := CUDASingleRule(
				&targetRule.RuleLine,
				d_originalDict, d_originalDictLengths,
				d_processedDict, d_processedDictLengths,
				uint64(len(targetFile)),
			)
			newTargets := removeMatchingWords(targetDictWithRule, targetFile)
			process_all_wordlists(targetFile, newTargets, cli, false)
			processBar.Add(len(targetFile))
		}
	}
	log.Printf("Loaded %d target words.", len(targetFile))
	log.Println("Done")
}

func process_all_wordlists(targetFile []string, targetFileRuled []string, cli CLI, debug bool) {
	// Check all wordlists
	var validWordlists []string
	for _, wordlist := range cli.Wordlists {
		// Skip directories and non-existant stuff
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

	if len(validWordlists) == 0 {
		log.Fatalf("No valid wordlists founds.")
		return
	}

	if debug {
		log.Printf("Loaded %d wordlists", len(validWordlists))
	}
	for i := cli.MinTarget; i <= cli.MaxTarget; i++ {
		if debug {
			log.Printf("Processing length %d", i)
		}
		combinations := generateCombinations(targetFile, targetFileRuled, i)
		for _, wordlist := range validWordlists {
			if debug {
				log.Printf("Processing %s", wordlist)
			}
			process_wordlist(&combinations, wordlist, cli)
		}
	}
}

// loop through lines per file
func process_wordlist(combinations *[][]string, wordlist string, cli CLI) {
	// Read target file
	readFile, err := os.Open(wordlist)
	if err != nil {
		log.Fatal(err)
		os.Exit(199)
	}
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	var wg sync.WaitGroup
	var wgOutput sync.WaitGroup
	outputChannel := make(chan string, 1000)

	go func() {
		wgOutput.Add(1)
		defer wgOutput.Done()
		var buffer string
		bufferSize := 0
		// Prep output file handle

		if cli.OutputFile == "" {
			for output := range outputChannel {
				buffer += output + "\n"
				bufferSize += 1
				if bufferSize == 1000 {
					fmt.Print(buffer)
					buffer = ""
					bufferSize = 0
				}
			}
			if bufferSize > 0 {
				fmt.Print(buffer)
				buffer = ""
				bufferSize = 0
			}
		} else {
			f, err := os.OpenFile(cli.OutputFile,
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatal(err)
				os.Exit(-1)
			}
			defer f.Close()
			for output := range outputChannel {
				buffer += output + "\n"
				bufferSize += 1
				if bufferSize == 10000 {
					if _, err := f.WriteString(buffer); err != nil {
						log.Println(err)
						os.Exit(-1)
					}
					buffer = ""
					bufferSize = 0
				}
			}
			if bufferSize > 0 {
				if _, err := f.WriteString(buffer); err != nil {
					log.Println(err)
					os.Exit(-1)
				}
				buffer = ""
				bufferSize = 0
			}
		}
	}()

	sem := make(chan struct{}, runtime.NumCPU())
	for fileScanner.Scan() {
		word := fileScanner.Text()
		sem <- struct{}{}
		wg.Add(1)

		go func(threadCombinations *[][]string, wordx string) {
			defer wg.Done()
			defer func() { <-sem }()
			for _, combo := range *threadCombinations {
				for i := 0; i <= len(combo); i++ {
					newCombo := make([]string, len(combo)+1)
					copy(newCombo, combo[:i])
					newCombo[i] = wordx
					copy(newCombo[i+1:], combo[i:])
					// Print the new combination
					outputChannel <- strings.Join(newCombo, cli.Separator)
				}
			}
		}(combinations, word)
	}
	readFile.Close()
	wg.Wait()
	close(outputChannel)
	wgOutput.Wait()
}
