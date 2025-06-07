package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/alecthomas/kong"
)

/*
aka flaginator

This tool, originally dubbed PermutationFlagg and later renamed ValdiComb and now
renamed to Targinator is a combinator tool combining wordlists and targeted
values and is written of celebration of Flagg's work (R.I.P. 2025-03). He was
a valued member of the HashMob and Hashpwn community and a good friend to us
all. May you rest in peace.

By combining specific aspects of a password as well as a targeted wordlist we
hit passwords that combine both personal aspects and general aspects of a password.
Authored by Vavaldi and Cyclone with the original idea provided by Flagg in
2021.
*/

/*
Modifications by cyclone:
v0.0.1-2025-05-10-dev
	fix mutexes (copy from locked mutex)
	fix waitgroups (concurrency safety)
	removed unused functions (timer, binomial)
	removed all C, CUDA (GPU) logic and converted to Pure Go using CPU
	added support for $HEX[] lines
	began refactoring code
	this code is an order of magnitude+ faster than the original GPU version, but the output is not always 100% 1:1

TODO:
	refactor code
	tweak: file read / write buffers, channels, sync groups, mutexes
*/

type ruleObj struct {
	ID           uint64
	Fitness      uint64
	LastFitness  uint64
	RuleLine     []Rule
	PreProcessed bool
	Hits         map[uint64]struct{}
	HitsMutex    sync.Mutex
}

var kill = sync.Mutex{}

var builderPool = sync.Pool{
	New: func() interface{} { return new(strings.Builder) },
}

type lineObj struct {
	ID   uint64
	line string
}

type CLI struct {
	Target        string   `arg:"" help:"Path to target data file (must fit in memory)"`
	Wordlists     []string `arg:"" help:"Path to wordlist files or directory"`
	MinTarget     int      `optional:"" short:"m" help:"Minimum target occurrences" default:"1"`
	MaxTarget     int      `optional:"" short:"x" help:"Maximum target occurrences" default:"3"`
	TargetRules   string   `optional:"" short:"t" help:"Apply rules file to Target" default:""`
	WordlistRules string   `optional:"" short:"r" help:"Apply rules file to Wordlist, warning: forces wordlist memory" default:""`
	Separator     string   `optional:"" short:"s" help:"Word Separator" default:""`
	OutputFile    string   `optional:"" short:"o" help:"Output File" default:""`
	Keyspace      bool     `optional:"" help:"Show keyspace for attack (used for HTP)" default:"false"`
	Skip          uint64   `optional:"" help:"Skip initial N generated candidates (used for HTP but are not consistent)" default:"0"`
	Limit         uint64   `optional:"" help:"Stop attack early after N generated candidates (used for HTP but are not consistent)" default:"0"`
	Debug         bool     `optional:"" help:"Show Debug Messages" default:"false"`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("Targinator"),
		kong.Description("v0.0.1-2025-05-10-dev - A combinator application using a targeted and generic wordlist"),
		kong.UsageOnError(),
	)

	// Get the target list and exit if invalid
	if cli.Debug {
		log.Println("Loading Target File:", cli.Target)
	}
	targetFile, tarErr := loadTargetFile(cli.Target)
	if tarErr != nil {
		log.Fatal(tarErr)
		return
	}
	if cli.Debug {
		log.Printf("Loaded %d target words.", len(targetFile))
	}

	if cli.Keyspace {
		targetKeyspace, err := lineCounter(cli.Target)
		if err != nil {
			log.Fatal(err)
		}

		numTargetRules := 0
		if cli.TargetRules != "" {
			targetRules, err := loadRulesFast(cli.TargetRules)
			if err != nil {
				log.Fatal(err)
			}
			numTargetRules = len(targetRules)
		}
		targetKeyspace = targetKeyspace * (1 + numTargetRules)

		wordlistKeyspace := 0
		wordlists := filterByValidWordlistTarget(cli.Wordlists, cli)
		if len(wordlists) == 0 {
			log.Fatal("No valid wordlist files specified")
		}

		numWordlistRules := 0
		if cli.WordlistRules != "" {
			wordlistRules, err := loadRulesFast(cli.WordlistRules)
			if err != nil {
				log.Fatal(err)
			}
			numWordlistRules = len(wordlistRules)
		}

		for _, wordlist := range wordlists {
			wordlistLines, err := lineCounter(wordlist)
			if err != nil {
				log.Fatal(err)
			}
			wordlistKeyspace += wordlistLines * (1 + numWordlistRules)
		}
		totalCombinations := 0
		for i := cli.MinTarget; i <= cli.MaxTarget; i++ {
			if cli.Debug {
				log.Printf("Processing length %d", i)
			}
			combinations := generateCombinations(targetFile, i)
			totalCombinations += len(combinations)
		}
		totalKeyspace := totalCombinations * wordlistKeyspace
		fmt.Printf("%d\n", totalKeyspace)
		return
	}

	process_all_wordlists(targetFile, cli)

	// striped out all C / CUDA / GPU code and replaced with Pure Go CPU logic
	// run target rules on CPU
	if cli.TargetRules != "" {
		targetRuleFile, tarErr := loadRulesFast(cli.TargetRules)
		if tarErr != nil {
			log.Fatal(tarErr)
			return
		}
		for _, ro := range targetRuleFile {
			if cli.Debug {
				log.Printf("Running rule: %s", FormatAllRules(ro.RuleLine))
			}
			// apply rule in parallel on all cores
			newWords := applyRuleCPU(ro.RuleLine, targetFile)
			newTargets := removeMatchingWords(newWords, targetFile)
			if len(newTargets) > 0 {
				process_all_wordlists(newTargets, cli)
			}
		}
	}

	if cli.Debug {
		log.Println("Done")
	}
}
