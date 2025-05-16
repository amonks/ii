#!/bin/sh
set -e

echo "Running TV Data Recovery"
cd /usr/home/ajm/monks.co

# Build the tool
go build -o tvdatarecovery ./apps/movies/cmd/tvdatarecovery

# Run it
./tvdatarecovery

echo "Recovery completed. Run tvmetadatafetcher and tvcopier to re-download missing files."