# Targinator
This tool has been semi-public for a while, being shared around in the community after being created for a personal request.
I've now decided is a good moment to make it open source and public with some modifications and improvements.

This tool functions as a semi-infinite combinator with 2 wordlists and some extra options as opposed to its original implementation.

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
Targinator combines 2 wordlists multiplicatively (creating every combination possible), and combines the 'target' wordlist any number of times with the 'wordlist'.
This means that if you have a few select components you suspect to be in the password you can combine those parts with other popular passwords or words to gain highly specialized
passwords for a specific user.
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

For example, our target wordlist can be:
```
Jim
1991-08-10
1991!
Baseball
```

And our wordlist can be:
```
password
Password
Secret
```

If we run a command such as: ```targinator target.txt wordlist.txt --min-target=1 --max-target=4 --separator"+"```
It will start making combinations such as:
```
Jim+password
Jim+Password
Jim+Secret
1991-08-10+password
1991-08-10+Password
...
Baseball+password
Baseball+Password
...
1991-08-10+Secret1991!
...
password+1991-08-10+Baseball+Jim+Jim
1991-08-10+password+Baseball+Jim+Jim
1991-08-10+Baseball+password+Jim+Jim
1991-08-10+Baseball+Jim+password+Jim
1991-08-10+Baseball+Jim+Jim+password
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

And so on until the end of time. This tool was originally named Valdicomb by Flagg - though I didn't take the name, and kept it as PermutationFlagg which was the project's original name.
It felt weird to have my name on a tool he inspired. But still, I ended up changing it a few months later after seeing a conversation where it was mentioned again.

![image](https://github.com/user-attachments/assets/886bf778-a150-423b-8211-4729b8066bce)

Now I release this tool in his memory hoping that others will see something with his idea.
Rest well Flagg, we miss you.
