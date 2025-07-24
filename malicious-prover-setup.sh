#!/bin/bash

set -e

echo "Installing Go 1.23.0..."

curl -LO https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
rm go1.23.0.linux-amd64.tar.gz

if ! grep -q '/usr/local/go/bin' ~/.bashrc; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
fi
export PATH=$PATH:/usr/local/go/bin

source ~/.bashrc

echo "Go installed: $(go version)"

echo "Cloning repo..."
git clone --recursive https://github.com/jakovmitrovski/arbitrum-light-client-go.git

cd arbitrum-light-client-go

echo "Running go mod tidy..."
go mod tidy

echo "Done!"