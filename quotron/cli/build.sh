#!/bin/bash
# Build and install the Quotron CLI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building Quotron CLI..."
go build -o quotron ./cmd/main

echo "Installing to ../quotron binary..."
cp quotron ..

# Update top-level README only if we're in a CI environment or if UPDATE_README=1
# This ensures the badge is only updated during CI builds or when explicitly requested
# To manually update the README: UPDATE_README=1 ./build.sh
if [ -n "$CI" ] || [ "$UPDATE_README" = "1" ]; then
  # Get current git commit hash
  COMMIT_HASH=$(git rev-parse --short HEAD)

  # Generate CLI help output
  CLI_HELP=$(./quotron help)

  # Check if top-level README exists
  TOP_README="../../README.md"
  if [ -f "$TOP_README" ]; then
    echo "Updating top-level README.md..."
    
    # Update the CLI badge with current commit hash - both the badge text and the label parameter
    sed -i "s/^\[\!\[CLI:[^]]*|/[![CLI:${COMMIT_HASH}|/" "$TOP_README"
    sed -i "s/label=CLI%3A[^&]*/label=CLI%3A${COMMIT_HASH}/" "$TOP_README"
    
    # Add CLI help section if it doesn't exist or update existing section
    if grep -q "<!-- CLI_HELP_START -->" "$TOP_README"; then
      # CLI help section exists, update it
      awk -v help="$CLI_HELP" 'BEGIN{in_section=0} 
        /<!-- CLI_HELP_START -->/ {print; print "```"; print help; print "```"; in_section=1; next} 
        /<!-- CLI_HELP_END -->/ {in_section=0; print; next} 
        !in_section {print}' "$TOP_README" > "$TOP_README.tmp" && mv "$TOP_README.tmp" "$TOP_README"
    else
      # CLI help section doesn't exist, add it before the Development section
      awk -v help="$CLI_HELP" '/^## Development/ { 
        print "## CLI Reference"; 
        print ""; 
        print "<!-- CLI_HELP_START -->"; 
        print "```"; 
        print help; 
        print "```"; 
        print "<!-- CLI_HELP_END -->"; 
        print ""; 
        print $0; 
        next 
      } { print }' "$TOP_README" > "$TOP_README.tmp" && mv "$TOP_README.tmp" "$TOP_README"
    fi
    
    echo "Top-level README.md updated with current CLI help and commit hash ${COMMIT_HASH}"
  else
    echo "Warning: Top-level README.md not found at $TOP_README"
  fi
fi

echo "Build completed successfully!"
echo "You can run the CLI with: ./quotron [command] [options]"