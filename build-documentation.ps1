$dateFormatted = Get-Date -Format 'yyyy-MM-dd'
$timeFormatted = Get-Date -Format 'HH:mm:ss'
$document = Join-Path $PSScriptRoot 'README.adoc'

Write-Host "Build .html documentation..."
asciidoctorj -r asciidoctor-diagram -a docdate=$dateFormatted -a doctime=$timeFormatted $document
