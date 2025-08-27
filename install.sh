#!/bin/bash
if ! command -v go &> /dev/null; then
    echo "Error: go is not installed."
    exit 1
fi

if ! command -v git &> /dev/null; then
    echo "Error: git is not installed."
    exit 1
fi

TMP_DIR=$(mktemp -d)
REPO_URL="https://github.com/axodouble/gobonsai.git"

git clone "$REPO_URL" "$TMP_DIR"
cd "$TMP_DIR"

go mod tidy

go build -o gobonsai

BIN_DIR="$HOME/.local/bin"
mkdir -p "$BIN_DIR"
cp gobonsai "$BIN_DIR/"

if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
    echo "export PATH=\"\$PATH:$BIN_DIR\"" >> "$HOME/.bashrc"
    echo "Added $BIN_DIR to PATH in .bashrc. Please restart your shell or run: source ~/.bashrc"
fi
. ~/.bashrc
echo "Installed gobonsai to $BIN_DIR"

