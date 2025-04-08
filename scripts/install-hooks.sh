#!/bin/bash

# Install Git hooks for the Go SDK

# Source colors and ASCII utilities
source "$(dirname "$0")/utils/colors.sh"
source "$(dirname "$0")/utils/ascii.sh"

print_header "Installing Git Hooks"

PROJECT_ROOT=$(git rev-parse --show-toplevel)
HOOKS_DIR="$PROJECT_ROOT/.githooks"
GIT_HOOKS_DIR="$PROJECT_ROOT/.git/hooks"

# Install each hook
for hook_dir in "$HOOKS_DIR"/*; do
    if [ -d "$hook_dir" ]; then
        hook_name=$(basename "$hook_dir")
        hook_src="$hook_dir/$hook_name"
        hook_dest="$GIT_HOOKS_DIR/$hook_name"
        
        # Check if hook source exists
        if [ -f "$hook_src" ]; then
            # Copy the hook
            cp "$hook_src" "$hook_dest"
            chmod +x "$hook_dest"
            print_success "Installed hook: $hook_name"
        else
            print_warning "Hook source not found: $hook_src"
        fi
    fi
done

print_info "Git hooks installed successfully"