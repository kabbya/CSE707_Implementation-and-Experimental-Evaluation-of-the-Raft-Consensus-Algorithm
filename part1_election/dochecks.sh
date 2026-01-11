# This script runs static analysis checks on all Go code
# It fails immediately if any issue is found


#!/bin/bash
# Run this file using the bash shell.

set -ex

# Run Go's built-in static analyzer
go vet ./...

# Run Staticcheck for deeper analysis
staticcheck ./...
