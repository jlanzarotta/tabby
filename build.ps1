$ErrorActionPreference = 'Stop'

Remove-Item tabby.exe -ErrorAction SilentlyContinue

$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'

$buildVersion = '1.0.0'
$buildDateTime = Get-Date -Format s

$ldflags = "-s -w -X 'tabby/internal/cli.BuildVersion=$buildVersion' -X 'tabby/internal/cli.BuildDateTime=$buildDateTime'"

go build -ldflags $ldflags -o tabby.exe ./cmd/tabby
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
