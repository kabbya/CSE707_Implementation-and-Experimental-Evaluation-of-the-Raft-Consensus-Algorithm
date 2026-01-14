# dotest.ps1
# -----------------------------
# This script runs Go tests with the race detector,
# saves the output to a log file, and generates an
# HTML visualization using raft-testlog-viz.
#
# Usage:
#   .\dotest.ps1 TestPart1FullFlow
#   .\dotest.ps1 TestElectionBasic
#   .\dotest.ps1
# -----------------------------

param (
    [string]$TestName = ""
)

# Ensure tmp directory exists (used by raft-testlog-viz)
$TmpDir = "C:\tmp\part2_logReplication"
if (!(Test-Path $TmpDir)) {
    New-Item -ItemType Directory -Path $TmpDir | Out-Null
}


# Where to store the test log
$LogFile = "rlog.txt"

Write-Host "Running Go tests..." -ForegroundColor Cyan

if ($TestName -ne "") {
    go test -v -race -run $TestName *> $LogFile
} else {
    go test -v -race *> $LogFile
}

if ($LASTEXITCODE -ne 0) {
    Write-Error "Tests failed. Aborting visualization."
    exit 1
}

Write-Host "Generating HTML visualization..." -ForegroundColor Cyan

Get-Content $LogFile | go run ..\tools\raft-testlog-viz\main.go
Get-ChildItem "C:\tmp\*.html" | Move-Item -Destination $TmpDir -Force

Write-Host "Done. HTML output directory: $TmpDir" -ForegroundColor Green
