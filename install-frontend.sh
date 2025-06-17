#!/bin/bash

# Find all directories named "frontend"
find . -type d -name "frontend" | while read -r frontend_dir; do
    echo "Installing dependencies in $frontend_dir"
    
    # Change to the frontend directory and run npm install
    cd "$frontend_dir"
    rm -r dist
    npm install
    npm link cybernetic-ui
    npm run build
    cd - > /dev/null  # Return to the original directory
done 