#!/bin/bash

# Agentainer CLI Uninstallation Script

set -e

echo "Uninstalling Agentainer CLI..."

# Remove binary from user bin
if [ -f "$HOME/bin/agentainer" ]; then
    echo "Removing binary from $HOME/bin..."
    rm -f "$HOME/bin/agentainer"
else
    echo "Binary not found in $HOME/bin"
fi

# Remove configuration directory
if [ -d "$HOME/.agentainer" ]; then
    echo "Removing configuration directory..."
    read -p "Remove all configuration and data? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "$HOME/.agentainer"
        echo "Configuration directory removed"
    else
        echo "Configuration directory preserved"
    fi
else
    echo "Configuration directory not found"
fi

# Remove PATH entry from .bashrc (optional)
if grep -q 'export PATH="$HOME/bin:$PATH"' "$HOME/.bashrc" 2>/dev/null; then
    read -p "Remove PATH entry from .bashrc? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Create backup
        cp "$HOME/.bashrc" "$HOME/.bashrc.backup"
        # Remove the PATH entry
        grep -v 'export PATH="$HOME/bin:$PATH"' "$HOME/.bashrc.backup" > "$HOME/.bashrc"
        echo "PATH entry removed from .bashrc (backup created)"
    fi
fi

echo "✅ Agentainer CLI uninstalled successfully!"
echo ""
echo "Note: You may need to restart your terminal or run 'hash -r' to clear the command cache."