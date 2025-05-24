#!/bin/bash

# Script to search for a term in a specified index

SERVER_URL="http://localhost:8080"
INDEX_NAME="imdb_movies" # Change this if you use a different index

# Check if a search term is provided
if [ -z "$1" ]; then
  echo "Usage: $0 "<search_term>" [page_size]"
  echo "Example: $0 "lord of the rings""
  echo "Example: $0 "batman" 10"
  exit 1
fi

SEARCH_TERM="$1"
PAGE_SIZE=${2:-5} # Default page size to 5 if not provided

echo "Searching for term: '$SEARCH_TERM' in index '$INDEX_NAME' with page size $PAGE_SIZE..."
echo "Ensure your Go search engine application is running at $SERVER_URL"

# Construct the JSON payload
# Note: Using jq for robust JSON creation if available, otherwise manual string construction.
if command -v jq &> /dev/null; then
  JSON_PAYLOAD=$(jq -n --arg query "$SEARCH_TERM" --argjson page_size "$PAGE_SIZE" \
    '{query: $query, page: 1, page_size: $page_size}')
else
  echo "Warning: jq is not installed. Constructing JSON payload manually (less robust for special characters)."
  # Basic escaping for quotes, more robust escaping might be needed for complex terms
  ESCAPED_SEARCH_TERM=$(echo "$SEARCH_TERM" | sed 's/"/\\"/g')
  JSON_PAYLOAD='{"query": "'"$ESCAPED_SEARCH_TERM"'", "page": 1, "page_size": '"$PAGE_SIZE"'}'
fi

echo "Sending request with payload: $JSON_PAYLOAD"

# Perform the search using curl
# Assuming the search endpoint is POST /indexes/{indexName}/_search
# The HTTP method (POST vs GET) for search depends on your API design.
# POST is often used if the query object can be complex (e.g., with filters).
curl -s -X POST -H "Content-Type: application/json" -d "$JSON_PAYLOAD" \
  "$SERVER_URL/indexes/$INDEX_NAME/_search" | jq .

# If you don't have jq or prefer raw output:
# curl -s -X POST -H "Content-Type: application/json" -d "$JSON_PAYLOAD" \
#  "$SERVER_URL/indexes/$INDEX_NAME/_search"; echo ""


echo -e "\nSearch script finished." 