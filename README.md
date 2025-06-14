# Targinator
Targinator combines a wordlist with your hints in every possible way. Ideal for highly targeted attacks. 
This tool has been semi-public for a while, being shared around in the community after being requested by Flagg.
For the last few months I've slowly been working on this project in my free time to make it open source and public with some modifications and improvements.

This tool functions as a semi-infinite combinator with wordlists and some extra options.

# Quick Start
First, **Install GoLang**. Then run the following commands:
or on linux:
```
git clone https://github.com/0xVavaldi/Targinator && cd Targinator
go build
```

```
targinator target.txt hashmob.net_2025-04-13.micro.found -x4 -t rule.txt
```

## Basic Usage
```
.\targinator.exe --help
Usage: Targinator <target> [<wordlists> ...] [flags]

A self-combinator using a targeted and generic wordlist - v0.0.1-2025-06-07-dev

Arguments:
  <target>             Path to target data file (must fit in memory)
  [<wordlists> ...]    Path to wordlist files or directory

Flags:
  -h, --help                   Show context-sensitive help.
  -m, --min-target=1           Minimum target occurrences
  -x, --max-target=3           Maximum target occurrences
  -t, --target-rules=""        Apply rules file to Target
  -r, --wordlist-rules=""      Apply rules file to Wordlist, warning: forces
                               wordlist memory
  -s, --separator=""           Word Separator
  -o, --output-file=""         Output File
      --keyspace               Show keyspace for attack (used for HTP)
      --skip=0                 Skip initial N generated candidates (used for
                               HTP)
      --limit=0                Stop attack early after N generated candidates
                               (used for HTP)
      --self-combination       Combine without using a wordlist [default: True]
      --partial-deduplicate    Help reduce the amount of duplicates
      --debug                  Show Debug Messages
```


### In Memoriam
A few years ago Flagg came with an idea. He wanted to take a set of hints or 'targets' and combine them infinitely together.
This inspired a tool originally called PermutationFlagg. It would take words like "James" "Bond" "2006" and make combinations
such as: 
- JamesBond
- BondJames
- James2006BondJames
- Bond2006JamesBond
- 2006JamesBondJamesBond
And so on until the end of time. This tool was named Valdicomb by Flagg though retained the original name PermutationFlagg in the folder structure so the included 7z is that same folder for the original project.

![image](https://github.com/user-attachments/assets/886bf778-a150-423b-8211-4729b8066bce)

Now I release this tool in his memory hoping that others will see his idea and share it.
Rest well Flagg, we miss you.
