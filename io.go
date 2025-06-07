package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
)

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
			return []string{path}, fmt.Errorf("target file %s is a directory", path)
		}
		return nil, err
	}
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening target file %s: %w", path, err)
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1<<20) // 1 MiB buffer
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			if len(line) > 0 {
				lines = append(lines, checkForHex(strings.TrimSuffix(line, "\n")))
			}
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading target file %s: %w", path, err)
		}
		lines = append(lines, checkForHex(strings.TrimSuffix(line, "\n")))
	}
	return lines, nil
}

func lineCounter(inputFile string) (int, error) {
	file, err := os.Open(inputFile)
	if err != nil {
		return 0, err
	}

	defer file.Close()
	buf := make([]byte, 128*1024)
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

func loadRulesFast(inputFile string) ([]*ruleObj, error) {
	file, err := os.Open(inputFile)
	if err != nil {
		return nil, fmt.Errorf("opening rules file %s: %w", inputFile, err)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, 1<<20) // 1 MiB buffer

	type task struct {
		ID   uint64
		line string
	}
	
	taskCh := make(chan task, runtime.NumCPU())
	var (
		mu      sync.Mutex
		results []*ruleObj
		wg      sync.WaitGroup
	)

	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				if t.line == "" {
					continue
				}
				rules, err := ConvertFromHashcat(t.ID, t.line)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error parsing rule on line %d: %v\n", t.ID, err)
					continue
				}
				mu.Lock()
				results = append(results, &ruleObj{
					ID:       t.ID,
					RuleLine: rules,
					Hits:     make(map[uint64]struct{}),
				})
				mu.Unlock()
			}
		}()
	}

	var lineCount uint64 = 1
	for {
		raw, err := reader.ReadString('\n')
		if err == io.EOF {
			raw = strings.TrimSpace(raw)
			if raw != "" {
				taskCh <- task{ID: lineCount, line: raw}
			}
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading rules file %s: %w", inputFile, err)
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" {
			taskCh <- task{ID: lineCount, line: trimmed}
		}
		lineCount++
	}
	close(taskCh)
	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results, nil
}
