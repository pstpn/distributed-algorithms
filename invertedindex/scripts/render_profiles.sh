#!/bin/sh
set -e

input=${1:-data/profiles/cpu.pprof}
output=${2:-data/results/cpu.pdf}

dotfile=${output%.pdf}.dot

go tool pprof -dot -nodecount=50 "$input" > "$dotfile"
dot -Tpdf "$dotfile" > "$output"
