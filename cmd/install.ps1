Write-Host "Installing grroxy components..."

# Store the original directory
$ORIGINAL_DIR = Get-Location

# Array of directories to process
$DIRS = @("cmd\grroxy", "cmd\grroxy-app", "cmd\grroxy-tool")

# Loop through each directory
foreach ($dir in $DIRS) {
    Write-Host "Installing in $dir..."
    $FULL_PATH = Join-Path $PSScriptRoot $dir
    
    if (-not (Test-Path $FULL_PATH)) {
        Write-Host "Directory $dir not found at $FULL_PATH"
        continue
    }
    
    Set-Location $FULL_PATH
    go install
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to install in $dir"
    }
    Set-Location $ORIGINAL_DIR
}

Write-Host "Installation complete!" 