#!/bin/bash
# Alpha Vantage API Setup Script
# This script guides you through setting up the Alpha Vantage API key for Quotron

echo "===== Alpha Vantage API Setup for Quotron ====="
echo 
echo "To use the Alpha Vantage API, you need a free API key."
echo "You can get one at: https://www.alphavantage.co/support/#api-key"
echo
echo "Instructions:"
echo "1. Visit the URL above"
echo "2. Fill out the short form (name, email, etc.)"
echo "3. Click 'GET FREE API KEY'"
echo "4. Copy the API key that is generated"
echo 
echo "Once you have your API key, you can proceed:"

# Check if the API key is already set
if [ -n "$ALPHA_VANTAGE_API_KEY" ]; then
  echo "Current API key: $ALPHA_VANTAGE_API_KEY"
  read -p "Do you want to use a different API key? (y/n): " change_key
  if [ "$change_key" != "y" ]; then
    echo "Keeping existing API key."
    exit 0
  fi
fi

# Ask for the API key
read -p "Enter your Alpha Vantage API key: " api_key

if [ -z "$api_key" ]; then
  echo "Error: API key cannot be empty."
  exit 1
fi

# Set the API key in the environment temporarily for testing
export ALPHA_VANTAGE_API_KEY="$api_key"

# Test the API key with a simple request
echo
echo "Testing your API key with a request for AAPL stock quote..."
cd "$(dirname "$0")/../../api-scraper" && go run cmd/main/main.go --api-key "$api_key" --symbol AAPL

# Check if the test was successful
if [ $? -eq 0 ]; then
  echo
  echo "API key test successful!"
  
  # Ask where to save the API key
  echo 
  echo "You can save your API key to:"
  echo "1. .env file in the quotron directory"
  echo "2. Your shell profile (~/.bashrc or ~/.zshrc)"
  echo "3. Don't save (just use it for this session)"
  read -p "Choose an option (1-3): " save_option
  
  case $save_option in
    1)
      echo "ALPHA_VANTAGE_API_KEY=$api_key" > "$(dirname "$0")/../../.env"
      echo "API key saved to $(dirname "$0")/../../.env"
      ;;
    2)
      if [ -f "$HOME/.bashrc" ]; then
        echo "export ALPHA_VANTAGE_API_KEY=$api_key" >> "$HOME/.bashrc"
        echo "API key added to ~/.bashrc. Run 'source ~/.bashrc' to apply."
      elif [ -f "$HOME/.zshrc" ]; then
        echo "export ALPHA_VANTAGE_API_KEY=$api_key" >> "$HOME/.zshrc"
        echo "API key added to ~/.zshrc. Run 'source ~/.zshrc' to apply."
      else
        echo "Could not find ~/.bashrc or ~/.zshrc. Please add this line to your shell profile:"
        echo "export ALPHA_VANTAGE_API_KEY=$api_key"
      fi
      ;;
    3)
      echo "API key is set for this session only. To use it in the future, run:"
      echo "export ALPHA_VANTAGE_API_KEY=$api_key"
      ;;
    *)
      echo "Invalid option. API key is set for this session only. To use it in the future, run:"
      echo "export ALPHA_VANTAGE_API_KEY=$api_key"
      ;;
  esac
  
  echo
  echo "Setup complete! You can now use Quotron's API scraper."
else
  echo
  echo "API key test failed. Please check your API key and try again."
  exit 1
fi