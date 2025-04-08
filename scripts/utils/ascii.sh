#!/bin/bash

# ASCII art and console utilities

# Print a centered header with a title
function print_header() {
    local title="$1"
    local width=70
    local padding=$(( (width - ${#title}) / 2 ))
    
    echo ""
    printf "%${padding}s" ""
    echo "$title"
    printf "%${width}s\n" "" | tr " " "-"
    echo ""
}

# Print a success message
function print_success() {
    echo -e "${bold_green}✓ $1${nc}"
}

# Print an error message
function print_error() {
    echo -e "${bold_red}✗ $1${nc}"
}

# Print a warning message
function print_warning() {
    echo -e "${bold_yellow}! $1${nc}"
}

# Print an info message
function print_info() {
    echo -e "${bold_blue}i $1${nc}"
}