#!/bin/bash

# Set colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
BOLD='\033[1m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "\n------------------------"
echo -e "   📝 Running tests  "
echo -e "------------------------\n"

# Find all packages, excluding examples
PACKAGES=$(go list ./... | grep -v "/examples/")

# Track overall success
OVERALL_SUCCESS=true

# Array to store packages without tests
PACKAGES_WITHOUT_TESTS=()

# Array to store failing packages/tests
FAILED_PACKAGES=()

# Run tests for each package individually
for pkg in $PACKAGES; do
    echo -e "Testing package: ${BOLD}$pkg${NC}"
    
    # Check if package has test files (either in the same package or in a _test package)
    # This checks both TestGoFiles (same package) and XTestGoFiles (external _test package)
    if go list -f '{{.TestGoFiles}}{{.XTestGoFiles}}' $pkg | grep -q "\[\]\[\]"; then
        echo -e "${BOLD}No test files found${NC}\n"
        
        # Skip mock packages when reporting
        if ! [[ "$pkg" == *"/mocks"* ]]; then
            PACKAGES_WITHOUT_TESTS+=("$pkg")
        fi
        continue
    fi
    
    # Test flags
    TEST_FLAGS="-v -timeout=30s"
    
    # Add the -short flag if GOTEST_SHORT is set
    if [ "$GOTEST_SHORT" = "1" ]; then
        TEST_FLAGS="$TEST_FLAGS -short"
    fi
    
    # Run tests with flags and capture output
    TEST_OUTPUT=$(go test $TEST_FLAGS $pkg 2>&1)
    TEST_EXIT_CODE=$?
    
    if [ $TEST_EXIT_CODE -eq 0 ]; then
        echo -e "${GREEN}✓ Package tests passed${NC}\n"
    else
        echo -e "${RED}✗ Package tests failed${NC}\n"
        OVERALL_SUCCESS=false
        
        # Extract failed test names and add to failed packages
        FAILED_TESTS=$(echo "$TEST_OUTPUT" | grep -o -E "^--- FAIL: Test[a-zA-Z0-9_]+" | sed 's/--- FAIL: //')
        for test in $FAILED_TESTS; do
            FAILED_PACKAGES+=("$pkg:$test")
        done
    fi
done

# Print summary of packages without tests
if [ ${#PACKAGES_WITHOUT_TESTS[@]} -gt 0 ]; then
    echo -e "\n${YELLOW}${BOLD}Summary of Packages Without Tests (excluding mocks and examples):${NC}"
    echo -e "${YELLOW}----------------------------------------${NC}"
    for pkg in "${PACKAGES_WITHOUT_TESTS[@]}"; do
        echo -e "${YELLOW}• $pkg${NC}"
    done
    echo -e "${YELLOW}----------------------------------------${NC}"
    echo -e "${YELLOW}Total: ${#PACKAGES_WITHOUT_TESTS[@]} package(s) without tests${NC}\n"
fi

# Print summary of failing tests
if [ ${#FAILED_PACKAGES[@]} -gt 0 ]; then
    echo -e "\n${RED}${BOLD}Summary of Failing Tests:${NC}"
    echo -e "${RED}----------------------------------------${NC}"
    for failure in "${FAILED_PACKAGES[@]}"; do
        IFS=':' read -r pkg test <<< "$failure"
        echo -e "${RED}• $pkg - $test${NC}"
    done
    echo -e "${RED}----------------------------------------${NC}"
    echo -e "${RED}Total: ${#FAILED_PACKAGES[@]} test(s) failed${NC}\n"
fi

# Check if all tests passed
if [ "$OVERALL_SUCCESS" = true ]; then
    echo -e "${GREEN}${BOLD}[ok]${NC} All tests completed successfully${GREEN} ✔️${NC}"
    exit 0
else
    echo -e "${RED}${BOLD}[error]${NC} Some tests failed${RED} ❌${NC}"
    exit 1
fi
