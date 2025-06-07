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
Authored by Vavaldi and Cyclone with the original idea provided by Flagg in 2021.
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
	Target             string   `arg:"" help:"Path to target data file (must fit in memory)"`
	Wordlists          []string `optional:"" arg:"" help:"Path to wordlist files or directory"`
	MinTarget          int      `optional:"" short:"m" help:"Minimum target occurrences" default:"1"`
	MaxTarget          int      `optional:"" short:"x" help:"Maximum target occurrences" default:"3"`
	TargetRules        string   `optional:"" short:"t" help:"Apply rules file to Target" default:""`
	WordlistRules      string   `optional:"" short:"r" help:"Apply rules file to Wordlist, warning: forces wordlist memory" default:""`
	Separator          string   `optional:"" short:"s" help:"Word Separator" default:""`
	OutputFile         string   `optional:"" short:"o" help:"Output File" default:""`
	Keyspace           bool     `optional:"" help:"Show keyspace for attack (used for HTP)" default:"false"`
	Skip               uint64   `optional:"" help:"Skip initial N generated candidates (used for HTP)" default:"0"`
	Limit              uint64   `optional:"" help:"Stop attack early after N generated candidates (used for HTP)" default:"0"`
	SelfCombination    bool     `optional:"" help:"Combine without using a wordlist (default True)" default:"true"`
	PartialDeduplicate bool     `optional:"" help:"Help reduce the amount of duplicates" default:"false"`
	Debug              bool     `optional:"" help:"Show Debug Messages" default:"false"`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("Targinator"),
		kong.Description("A self-combinator using a targeted and generic wordlist - v0.0.1-2025-06-07-dev"),
		kong.UsageOnError(),
	)

	// Get the target list and exit if invalid
	if cli.Debug {
		log.Println("Loading Target File:", cli.Target)
	}
	if cli.PartialDeduplicate && cli.Keyspace {
		log.Fatal("PartialDeduplicate and Keyspace are mutually exclusive as deduplication affects the keyspace")
		return
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
		fmt.Printf("%d\n", calculateKeyspace(targetFile, cli))
		return
	}

	processAllWordlists(targetFile, cli)
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
			if len(ro.RuleLine) == 1 && ro.RuleLine[0].Function == ":" {
				continue
			}

			// apply rule in parallel on all cores
			newWords := applyRuleCPU(ro.RuleLine, targetFile)
			if cli.PartialDeduplicate {
				newWords = removeMatchingWords(newWords, targetFile)
			}
			if len(newWords) > 0 {
				processAllWordlists(newWords, cli)
			}
		}
	}

	if cli.Debug {
		log.Println("Done")
	}
}
