# Targinator
This tool has been semi-public for a while, being shared around in the community after being created for a personal request.
I've now decided is a good moment to make it open source and public with some modifications and improvements.

This tool functions as a semi-infinite combinator with 2 wordlists and some extra options.

# Quick Start
First, **Install GoLang**. Then run the following code on windows:
```
git clone https://github.com/0xVavaldi/Targinator && cd Targinator
nvcc --shared -o rules.dll rules.cu -Xcompiler "/MD" -link -cudart static
go build
```
or on linux:
```
sudo apt install nvidia-cuda-toolkit
git clone https://github.com/0xVavaldi/Targinator && cd Targinator
nvcc --shared -o librules.so rules.cu -Xcompiler "-fPIC" --cudart static -arch=sm_61  # or higher compute version
go build
```

```
targinator target.txt hashmob.net_2025-04-13.micro.found -x4 -t rule.txt
```

## Basic Usage
```
targinator.exe --help
Usage: Targinator <target> <wordlists> ... [flags]

A combinator application using a targeted and generic wordlist

Arguments:
  <target>           Path to target data file (must fit in memory)
  <wordlists> ...    Path to wordlist files or directory

Flags:
  -h, --help               Show context-sensitive help.
  -m, --min-target=1       Minimum target occurrences
  -x, --max-target=3       Maximum target occurrences
  -t, --target-rules=""    Apply rules file to Target
  -s, --separator=""       Word Separator
  -o, --output-file=""     Output File
```


### In Memoriam
A few years ago Flagg came with an idea, the wish to take a set of hints or 'targets' and combine them infinitely together.
This inspired a tool originally called PermutationFlagg. It would take words like "James" "Bond" "2006" and make combinations
such as: 
- JamesBond
- BondJames
- James2006BondJames
- Bond2006JamesBond
- 2006JamesBondJamesBond
And so on until the end of time. This tool named Valdicomb by Flagg - though I personally wouldn't accept it, and kept it at the original name.
It felt weird to have my name on a tool he inspired, though I ended up changing it a few months later up after seeing a conversation where it was mentioned again.

![image](https://github.com/user-attachments/assets/886bf778-a150-423b-8211-4729b8066bce)

Now I release this tool in his memory hoping that others will see his idea and share in it.
Rest well Flagg, we miss you.
