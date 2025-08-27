# GoBonsai
This was a simple project that I worked on in my spare time, I just wanted to see if I could recreate cbonsai but in go.

## Installation
If you want to install gobonsai to run it anywhere:
### Install script (easiest)
```bash
curl -fsSL https://raw.githubusercontent.com/Axodouble/gobonsai/refs/heads/master/install.sh | bash
```
### Install manually
0. Make sure you have `git` and `golang` installed.
1. Clone this repository.
   `git clone https://github.com/axodouble/gobonsai`
2. Run the commands to build the binary.
```bash
go mod tidy
go build -o gobonsai
```
3. Run the binary
```bash
chmod +x ./gobonsai
./gobonsai
```
