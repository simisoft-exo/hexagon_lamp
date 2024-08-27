#!/bin/bash

# Generate a new SSH key
ssh-keygen -t ed25519 -f ~/.ssh/hexagon_key -N ""

# Copy the public key to the remote host
ssh-copy-id -i ~/.ssh/hexagon_key.pub hexagon@hexagon.local

# Add an entry to the SSH config file
cat << EOF >> ~/.ssh/config

Host hexagon
    HostName hexagon.local
    User hexagon
    IdentityFile ~/.ssh/hexagon_key
EOF

echo "SSH key generated and added to hexagon@hexagon.local"
echo "SSH config updated with alias 'hexagon' for hexagon.local"