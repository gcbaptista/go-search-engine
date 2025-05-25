#!/bin/bash

# Cleanup script for test data directories
# This script removes all test_data_* directories that might be left behind by tests

echo "Cleaning up test data directories..."

# Find and remove all test_data_* directories
find . -name "test_data_*" -type d -exec rm -rf {} \; 2>/dev/null

# Count how many were removed
count=$(find . -name "test_data_*" -type d 2>/dev/null | wc -l)

if [ $count -eq 0 ]; then
    echo "✅ All test directories cleaned up successfully!"
else
    echo "⚠️  Warning: Some test directories might still exist"
fi

echo "Done." 