#!/bin/bash

# Navigate to the examples directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
GREEN_BOLD='\033[1;32m'
YELLOW='\033[0;33m'
YELLOW_BOLD='\033[1;33m'
NO_COLOR='\033[0m'
BOLD='\033[1m'

# Spinner
spinner() {
    local pid=$1
    local message=$2
    local spin='|/-\' 
    local spin_char_width=1
    local i=0
    
    tput civis # Hide cursor
    while kill -0 $pid 2>/dev/null; do
        local i=$(((i + spin_char_width) % ${#spin}))
        printf "\r${spin:$i:$spin_char_width} ${message} ${spin:$i:$spin_char_width}"
        sleep 0.1
    done
    tput cnorm # Show cursor
    printf "\r"
}

# Initialize counters and directory logs
num_tested_dirs=0
num_skipped_dirs=0
num_total_test_cases=0
num_passed_test_cases=0
num_failed_test_cases=0
failed_dirs=()
skipped_dirs=()

# Start overall timer
overall_start=$(date +%s)

# TODO: Test for backwards compatibility by running tests using main branch, then test for forward compatibility by running tests using feature branch

for dir in */ ; do
    dir_name="${dir%/}"
    if [ -d "$dir/tests" ]; then
        ((num_tested_dirs++))
        echo "========================================="
        echo -e "${BOLD}Running terraform tests in $dir_name${NO_COLOR}"
        cd "$dir"
    
        echo "========================================="
        if [ ! -d ".terraform" ]; then
            terraform init > /tmp/tf_init_$$.log 2>&1 &
            init_pid=$!
            spinner $init_pid "Initializing terraform"
            wait $init_pid
            echo -e "${GREEN}✓${NO_COLOR} Initialization complete"
        else
            echo "Already initialized, skipping init"
        fi
        echo "========================================="

        terraform test > /tmp/tf_test_$.log 2>&1 &
        test_pid=$!
        spinner $test_pid "Testing in progress"
        wait $test_pid
        test_exit_code=$?
        
        # Read the output
        test_output=$(cat /tmp/tf_test_$.log)
        echo "$test_output"
        
        # Looking for patterns like "3 passed, 1 failed" or "Success! 4 passed"
        passed=$(echo "$test_output" | grep -Eo '[0-9]+ passed' | grep -Eo '[0-9]+' | head -1)
        failed=$(echo "$test_output" | grep -Eo '[0-9]+ failed' | grep -Eo '[0-9]+' | head -1)
        
        passed=${passed:-0}
        failed=${failed:-0}
        
        ((num_total_test_cases += passed + failed))
        ((num_passed_test_cases += passed))
        ((num_failed_test_cases += failed))
        
        if [ $test_exit_code -ne 0 ]; then
            failed_dirs+=("$dir_name")
        fi
        
        # Clean up temp files
        rm -f /tmp/tf_init_$$.log /tmp/tf_test_$$.log
        
        cd ..
    else
        echo "========================================="
        echo -e "${YELLOW_BOLD}Skipping $dir_name (no test folder found)${NO_COLOR}"
        ((num_skipped_dirs++))
        skipped_dirs+=("$dir_name")
    fi
done

# Calculate overall duration
overall_end=$(date +%s)
overall_duration=$((overall_end - overall_start))

# Format duration as minutes and seconds if over 60 seconds
format_duration() {
    local seconds=$1
    if [ $seconds -ge 60 ]; then
        local minutes=$((seconds / 60))
        local remaining_seconds=$((seconds % 60))
        echo "${minutes}m ${remaining_seconds}s"
    else
        echo "${seconds}s"
    fi
}

echo "========================================="
echo -e "${BOLD}Summary${NO_COLOR}"
echo "========================================="
echo -e "Runtime: $(format_duration $overall_duration)"
echo -e "Total directories: $((num_tested_dirs + num_skipped_dirs))"
echo -e "Skipped directories: $num_skipped_dirs"
echo -e "Tested directories: $num_tested_dirs"
echo -e "Total test cases: $num_total_test_cases"
echo -e "${GREEN}Passed: $num_passed_test_cases${NO_COLOR}"
echo -e "${RED}Failed: $num_failed_test_cases${NO_COLOR}"

echo ""

if [ $num_failed_test_cases -gt 0 ]; then
    echo -e "${RED}Directories with failures:${NO_COLOR}"
    for failed_dir in "${failed_dirs[@]}"; do
        echo "  - $failed_dir"
    done
else
    echo -e "${GREEN}✓ All $num_total_test_cases test cases passed!${NO_COLOR}"
fi

if [ $num_skipped_dirs -gt 0 ]; then
    echo ""
    echo -e "${YELLOW}Skipped directories:${NO_COLOR}"
    for skipped_folder in "${skipped_dirs[@]}"; do
        echo "  - $skipped_folder"
    done
fi

echo ""

if [ $num_failed_test_cases -gt 0 ]; then
    exit 1
else
    exit 0
fi