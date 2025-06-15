# Targinator
Targinator combines a wordlist with your hints in every possible way. Ideal for highly targeted attacks. 
This tool has been semi-public for a while, being shared around in the community after being requested by Flagg.
For the last few months I've slowly been working on this project in my free time to make it open source and public with some modifications and improvements.

This tool functions as a semi-infinite combinator with wordlists and some extra options.

Authored by Vavaldi with performance optimizations by Cyclone

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

For example you can combine the words:
```
Test
Power
King
Super
Entomb
```
Together multiple times such as with the command: `.\targinator target.txt -x 4`
```
Test
Power
King
Super
Entomb
TestPower
TestKing
TestSuper
TestEntomb
PowerTest
PowerKing
PowerSuper
PowerEntomb
KingTest
KingPower
KingSuper
KingEntomb
SuperTest
SuperPower
SuperKing
SuperEntomb
EntombTest
EntombPower
EntombKing
EntombSuper
TestPowerKing
TestPowerSuper
TestPowerEntomb
TestKingPower
TestKingSuper
TestKingEntomb
TestSuperPower
TestSuperKing
TestSuperEntomb
TestEntombPower
TestEntombKing
TestEntombSuper
PowerTestKing
PowerTestSuper
PowerTestEntomb
PowerKingTest
PowerKingSuper
PowerKingEntomb
PowerSuperTest
PowerSuperKing
PowerSuperEntomb
PowerEntombTest
PowerEntombKing
PowerEntombSuper
KingTestPower
KingTestSuper
KingTestEntomb
KingPowerTest
KingPowerSuper
KingPowerEntomb
KingSuperTest
KingSuperPower
KingSuperEntomb
KingEntombTest
KingEntombPower
KingEntombSuper
SuperTestPower
SuperTestKing
SuperTestEntomb
SuperPowerTest
SuperPowerKing
SuperPowerEntomb
SuperKingTest
SuperKingPower
SuperKingEntomb
SuperEntombTest
SuperEntombPower
SuperEntombKing
EntombTestPower
EntombTestKing
EntombTestSuper
EntombPowerTest
EntombPowerKing
EntombPowerSuper
EntombKingTest
EntombKingPower
EntombKingSuper
EntombSuperTest
EntombSuperPower
EntombSuperKing
TestPowerKingSuper
TestPowerKingEntomb
TestPowerSuperKing
TestPowerSuperEntomb
TestPowerEntombKing
TestPowerEntombSuper
TestKingPowerSuper
TestKingPowerEntomb
TestKingSuperPower
TestKingSuperEntomb
TestKingEntombPower
TestKingEntombSuper
TestSuperPowerKing
TestSuperPowerEntomb
TestSuperKingPower
TestSuperKingEntomb
TestSuperEntombPower
TestSuperEntombKing
TestEntombPowerKing
TestEntombPowerSuper
TestEntombKingPower
TestEntombKingSuper
TestEntombSuperPower
TestEntombSuperKing
PowerTestKingSuper
PowerTestKingEntomb
PowerTestSuperKing
PowerTestSuperEntomb
PowerTestEntombKing
PowerTestEntombSuper
PowerKingTestSuper
PowerKingTestEntomb
PowerKingSuperTest
PowerKingSuperEntomb
PowerKingEntombTest
PowerKingEntombSuper
PowerSuperTestKing
PowerSuperTestEntomb
PowerSuperKingTest
PowerSuperKingEntomb
PowerSuperEntombTest
PowerSuperEntombKing
PowerEntombTestKing
PowerEntombTestSuper
PowerEntombKingTest
PowerEntombKingSuper
PowerEntombSuperTest
PowerEntombSuperKing
KingTestPowerSuper
KingTestPowerEntomb
KingTestSuperPower
KingTestSuperEntomb
KingTestEntombPower
KingTestEntombSuper
KingPowerTestSuper
KingPowerTestEntomb
KingPowerSuperTest
KingPowerSuperEntomb
KingPowerEntombTest
KingPowerEntombSuper
KingSuperTestPower
KingSuperTestEntomb
KingSuperPowerTest
KingSuperPowerEntomb
KingSuperEntombTest
KingSuperEntombPower
KingEntombTestPower
KingEntombTestSuper
KingEntombPowerTest
KingEntombPowerSuper
KingEntombSuperTest
KingEntombSuperPower
SuperTestPowerKing
SuperTestPowerEntomb
SuperTestKingPower
SuperTestKingEntomb
SuperTestEntombPower
SuperTestEntombKing
SuperPowerTestKing
SuperPowerTestEntomb
SuperPowerKingTest
SuperPowerKingEntomb
SuperPowerEntombTest
SuperPowerEntombKing
SuperKingTestPower
SuperKingTestEntomb
SuperKingPowerTest
SuperKingPowerEntomb
SuperKingEntombTest
SuperKingEntombPower
SuperEntombTestPower
SuperEntombTestKing
SuperEntombPowerTest
SuperEntombPowerKing
SuperEntombKingTest
SuperEntombKingPower
EntombTestPowerKing
EntombTestPowerSuper
EntombTestKingPower
EntombTestKingSuper
EntombTestSuperPower
EntombTestSuperKing
EntombPowerTestKing
EntombPowerTestSuper
EntombPowerKingTest
EntombPowerKingSuper
EntombPowerSuperTest
EntombPowerSuperKing
EntombKingTestPower
EntombKingTestSuper
EntombKingPowerTest
EntombKingPowerSuper
EntombKingSuperTest
EntombKingSuperPower
EntombSuperTestPower
EntombSuperTestKing
EntombSuperPowerTest
EntombSuperPowerKing
EntombSuperKingTest
EntombSuperKingPower
```

Now inbetween each word you can place a single word from a dictionary. That means for `n` words you get `n+1` possible candidates where a word is inserted in.
This is done with the parameters after the target dictionary which specify wordlists or directories. For example with a Target dictionary of:
```
MyPassword
2006
1997
SuperPassword
Super
Password
```
Running the following command:
```.\targinator.exe -m 1 -x 5 .\target.txt .\hashmob.net_2025-06-15.micro.found <dict2> <folder>```
Which helps combine passwords to a new format of:
```
joseph123SuperPasswordPassword
SuperPasswordjoseph123Password
SuperPasswordPasswordjoseph123
joseph123SuperMyPassword
Superjoseph123MyPassword
SuperMyPasswordjoseph123
joseph123Super2006
Superjoseph1232006
Super2006joseph123
joseph123Super1997
Superjoseph1231997
Super1997joseph123
joseph123SuperSuperPassword
Superjoseph123SuperPassword
SuperSuperPasswordjoseph123
joseph123SuperPassword
Superjoseph123Password
SuperPasswordjoseph123
joseph123PasswordMyPassword
Passwordjoseph123MyPassword
PasswordMyPasswordjoseph123
joseph123Password2006
Passwordjoseph1232006
Password2006joseph123
joseph123Password1997
Passwordjoseph1231997
Password1997joseph123
joseph123PasswordSuperPassword
Passwordjoseph123SuperPassword
PasswordSuperPasswordjoseph123
joseph123PasswordSuper
Passwordjoseph123Super
PasswordSuperjoseph123
David123MyPassword2006
MyPasswordDavid1232006
MyPassword2006David123
David123MyPassword1997
aardvarkMyPassword1997
MyPasswordaardvark1997
MyPassword1997aardvark
aardvarkMyPasswordSuperPassword
MyPasswordaardvarkSuperPassword
MyPasswordSuperPasswordaardvark
MyPassword1997kaylee
kayleeMyPasswordSuperPassword
MyPasswordkayleeSuperPassword
aardvarkMyPasswordSuper
MyPasswordSuperPasswordkaylee
MyPasswordaardvarkSuper
kayleeMyPasswordSuper
MyPasswordkayleeSuper
MyPasswordSuperkaylee
kayleeMyPasswordPassword
MyPasswordkayleePassword
MyPasswordPasswordkaylee
kaylee2006MyPassword
2006kayleeMyPassword
2006MyPasswordkaylee
kaylee20061997
2006kaylee1997
20061997kaylee
MyPasswordSuperaardvark
kaylee2006SuperPassword
aardvarkMyPasswordPassword
2006kayleeSuperPassword
MyPasswordaardvarkPassword
2006SuperPasswordkaylee
MyPasswordPasswordaardvark
aardvark2006MyPassword
2006aardvarkMyPassword
2006MyPasswordaardvark
aardvark20061997
2006aardvark1997
20061997aardvark
aardvark2006SuperPassword
2006aardvarkSuperPassword
2006SuperPasswordaardvark
aardvark2006Super
2006aardvarkSuper
2006Superaardvark
kaylee2006Super
aardvark2006Password
2006aardvarkPassword
2006kayleeSuper
2006Passwordaardvark
2006Superkaylee
aardvark1997MyPassword
kaylee2006Password
1997aardvarkMyPassword
2006kayleePassword
1997MyPasswordaardvark
2006Passwordkaylee
aardvark19972006
MyPasswordDavid1231997
MyPassword1997David123
1997aardvark2006
David123MyPasswordSuperPassword
MyPasswordDavid123SuperPassword
MyPasswordSuperPasswordDavid123
David123MyPasswordSuper
MyPasswordDavid123Super
k12345672006MyPassword
MyPasswordSuperDavid123
2006k1234567MyPassword
19972006aardvark
2006MyPasswordk1234567
aardvark1997SuperPassword
k123456720061997
1997aardvarkSuperPassword
1997SuperPasswordaardvark
2006k12345671997
aardvark1997Super
20061997k1234567
k12345672006SuperPassword
2006k1234567SuperPassword
2006SuperPasswordk1234567
k12345672006Super
2006k1234567Super
2006Superk1234567
k12345672006Password
2006k1234567Password
2006Passwordk1234567
k12345671997MyPassword
1997k1234567MyPassword
1997MyPasswordk1234567
k123456719972006
1997k12345672006
19972006k1234567
David123MyPasswordPassword
1997aardvarkSuper
MyPasswordDavid123Password
1997Superaardvark
MyPasswordPasswordDavid123
David1232006MyPassword
2006David123MyPassword
2006MyPasswordDavid123
David12320061997
2006David1231997
20061997David123
aardvark1997Password
David1232006SuperPassword
1997aardvarkPassword
2006David123SuperPassword
1997Passwordaardvark
2006SuperPasswordDavid123
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
Now I release this tool in his memory hoping that others will see his idea and find good use for it.
Rest well Flagg, we miss you.
