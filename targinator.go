package main

// aka flaginator

// This tool, originally dubbed FlaggComb, later renamed VavaldiComb and now
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
	"errors"
	"fmt"
	"github.com/alecthomas/kong"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type CLI struct {
	Target     string   `arg:"" help:"Path to target data file (must fit in memory)"`
	Wordlists  []string `arg:"" help:"Path to wordlist files or directory"`
	MinTarget  int      `optional:"" short:"m" help:"Minimum target occurrences" default:"1"`
	MaxTarget  int      `optional:"" short:"x" help:"Maximum target occurrences" default:"3"`
	Separator  string   `optional:"" short:"s" help:"Word Separator" default:""`
	OutputFile string   `optional:"" short:"o" help:"Output File" default:""`
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

func generateCombinations(arr []string, k int) [][]string {
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
	log.Printf("Loaded %d target words.", len(targetFile))

	// Check all wordlists
	var validWordlists []string
	for _, wordlist := range cli.Wordlists {
		// Skip directories and non-existant stuff
		if isDir, dirErr := isDirectory(wordlist); isDir || dirErr != nil {
			if !isDir {
				log.Printf("%s. It will be skipped", dirErr.Error())
				continue
			}
			log.Printf("%s is a directory. Reading files...", wordlist)
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
			log.Printf("Loaded %d files from %s recursively", loadedFiles, wordlist)
		}
		if valid, fileErr := isReadable(wordlist); !valid || fileErr != nil {
			if fileErr != nil {
				log.Printf("%s. It will be skipped", fileErr.Error())
			} else {
				log.Printf("%s is invalid. It will be skipped", wordlist)
			}
			continue
		}
		validWordlists = append(validWordlists, wordlist)
	}

	if len(validWordlists) == 0 {
		log.Fatalf("No valid wordlists founds.")
		return
	}

	log.Printf("Loaded %d wordlists", len(validWordlists))
	for i := cli.MinTarget; i <= cli.MaxTarget; i++ {
		log.Printf("Processing length %d", i)
		combinations := generateCombinations(targetFile, i)
		for _, wordlist := range validWordlists {
			log.Printf("Processing %s", wordlist)
			process_wordlist(&combinations, wordlist, cli)
		}
	}
	log.Println("Done")
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
					if cli.OutputFile == "" {
						fmt.Print(buffer)
					}
					buffer = ""
					bufferSize = 0
				}
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
				if bufferSize == 1000 {
					if _, err := f.WriteString(output + "\n"); err != nil {
						log.Println(err)
						os.Exit(-1)
					}
					buffer = ""
					bufferSize = 0
				}
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
