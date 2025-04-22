#!/bin/bash
# SPDX-FileCopyrightText: (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

PATHS_TO_EXCLUDE=$(cat <<EOF
./artifacts/*
./ci/
./.cache/
EOF
)

# Find all JSON files in the current directory recursively and exclude the ones explicitly provided
JSONS_TO_SCAN=$(find . -name "*.json" | grep -vE "$PATHS_TO_EXCLUDE")

# Initialize a flag to track if any errors are found
error_found=0

jq --version

# Check if each JSON file is valid and pretty printed
for json_file in $JSONS_TO_SCAN; do
  if ! jq . "$json_file" | diff -q - "$json_file" > /dev/null; then
    echo "\"$json_file\" is not pretty printed. To pretty-print: jq . \"$json_file\" > tmp && mv tmp \"$json_file\""
    error_found=1
  fi
done

# Return error code 1 if any errors were found
if [ $error_found -eq 1 ]; then
  exit 1
fi
