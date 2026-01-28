#!/bin/bash -ex

sudo apt-get update
sudo apt-get install -y build-essential xvfb

echo "export PATH=\"$PATH:$HOME/go/bin\"" >> $HOME/.bashrc
echo "export PATH=\"$PATH:$HOME/go/bin\"" >> $HOME/.zshrc

curl -fsSL https://claude.ai/install.sh | bash
curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash

echo "eval \"\$(ssh-agent -s)\" >& /dev/null" >> $HOME/.bashrc
echo "eval \"\$(ssh-agent -s)\" >& /dev/null" >> $HOME/.zshrc
echo "ssh-add ~/.ssh/id_xana_2024_12_10 >& /dev/null" >> $HOME/.bashrc
echo "ssh-add ~/.ssh/id_xana_2024_12_10 >& /dev/null" >> $HOME/.zshrc
