#!/usr/bin/env bash
set -euo pipefail

out_dir="${1:-website_files}"
json="$(cat)"
mkdir -p "$out_dir"

jq -r '.Ret0.website_files | keys[]' <<< "$json" | while read -r file; do
  jq -r --arg file "$file" '.Ret0.website_files[$file]' <<< "$json" > "$out_dir/$file"
done
