if (-not (Get-Module PSReadLine)) {
	Import-Module PSReadLine -ErrorAction SilentlyContinue
}

if (-not (Get-Module PSReadLine)) {
	Write-Warning 'tabby: PSReadLine is required'
	return
}

$global:_tabby_exe = Get-Command tabby -CommandType Application -ErrorAction SilentlyContinue |
	Select-Object -First 1 -ExpandProperty Source
if (-not $global:_tabby_exe) {
	Write-Warning 'tabby: tabby executable not found in PATH'
	return
}

function global:_tabby_location {
	try {
		return (Get-Location -PSProvider FileSystem).ProviderPath
	} catch {
		return ''
	}
}

function global:_tabby_reset {
	$global:_tabby_matches = @()
	$global:_tabby_index = 0
	$global:_tabby_query = ''
}

function global:_tabby_fallback {
	param(
		[System.ConsoleKeyInfo] $Key,
		[object] $Argument,
		[int] $Direction
	)

	_tabby_reset
	if ($Direction -lt 0) {
		[Microsoft.PowerShell.PSConsoleReadLine]::TabCompletePrevious($Key, $Argument)
	} else {
		[Microsoft.PowerShell.PSConsoleReadLine]::TabCompleteNext($Key, $Argument)
	}
}

function global:_tabby_cycle {
	param(
		[System.ConsoleKeyInfo] $Key,
		[object] $Argument,
		[int] $Direction
	)

	$line = $null
	$cursor = 0
	[Microsoft.PowerShell.PSConsoleReadLine]::GetBufferState([ref] $line, [ref] $cursor)

	if ($global:_tabby_matches.Count -gt 0 -and
		$line -ceq $global:_tabby_matches[$global:_tabby_index] -and
		$cursor -eq $line.Length) {
		$global:_tabby_index = (
			$global:_tabby_index + $Direction + $global:_tabby_matches.Count
		) % $global:_tabby_matches.Count
		$match = $global:_tabby_matches[$global:_tabby_index]
		[Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $line.Length, $match)
		[Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($match.Length)
		return
	}

	_tabby_reset

	if ($cursor -ne $line.Length) {
		_tabby_fallback $Key $Argument $Direction
		return
	}

	$tabbyArgs = @('query', '--from-env')
	$tabbyInput = $line
	if ([string]::IsNullOrWhiteSpace($line)) {
		if ([string]::IsNullOrEmpty($global:_tabby_previous)) {
			_tabby_fallback $Key $Argument $Direction
			return
		}
		$tabbyArgs = @('next', '--from-env')
		$tabbyInput = $global:_tabby_previous
	}

	# PowerShell variable names are case-insensitive, so the saved value must
	# not be named $lastExitCode or it would shadow $global:LASTEXITCODE.
	$savedExitCode = $global:LASTEXITCODE
	$env:TABBY_INPUT = $tabbyInput
	$env:TABBY_DIR = _tabby_location
	try {
		$tabbyMatches = @(& $global:_tabby_exe @tabbyArgs 2>$null)
		$querySucceeded = $global:LASTEXITCODE -eq 0
	} finally {
		Remove-Item Env:TABBY_INPUT -ErrorAction SilentlyContinue
		Remove-Item Env:TABBY_DIR -ErrorAction SilentlyContinue
		$global:LASTEXITCODE = $savedExitCode
	}
	$tabbyMatches = @($tabbyMatches | Where-Object { -not [string]::IsNullOrEmpty($_) })
	if (-not $querySucceeded -or $tabbyMatches.Count -eq 0) {
		_tabby_fallback $Key $Argument $Direction
		return
	}

	$global:_tabby_query = $line
	$global:_tabby_matches = $tabbyMatches
	if ($Direction -lt 0) {
		$global:_tabby_index = $tabbyMatches.Count - 1
	}

	$match = $global:_tabby_matches[$global:_tabby_index]
	[Microsoft.PowerShell.PSConsoleReadLine]::Replace(0, $line.Length, $match)
	[Microsoft.PowerShell.PSConsoleReadLine]::SetCursorPosition($match.Length)
}

function global:_tabby_complete {
	param($Key, $Argument)
	_tabby_cycle $Key $Argument 1
}

function global:_tabby_complete_reverse {
	param($Key, $Argument)
	_tabby_cycle $Key $Argument -1
}

function global:_tabby_record_prompt {
	param([bool] $Succeeded)

	$picked = $null
	if ($global:_tabby_matches.Count -gt 0) {
		$picked = $global:_tabby_matches[$global:_tabby_index]
	}
	$query = $global:_tabby_query
	_tabby_reset

	$entry = Get-History -Count 1 -ErrorAction SilentlyContinue
	if ($null -eq $entry -or $entry.Id -eq $global:_tabby_last_history_id) {
		return
	}

	$global:_tabby_last_history_id = $entry.Id
	if (-not $Succeeded) {
		$global:_tabby_previous = ''
		return
	}

	$executed = $entry.CommandLine
	if (-not (_tabby_should_record $executed)) {
		$global:_tabby_previous = ''
		return
	}

	$startInfo = New-Object System.Diagnostics.ProcessStartInfo
	$startInfo.FileName = $global:_tabby_exe
	$startInfo.Arguments = 'add --from-env'
	$startInfo.UseShellExecute = $false
	$startInfo.CreateNoWindow = $true
	$startInfo.EnvironmentVariables['TABBY_INPUT'] = $executed
	$startInfo.EnvironmentVariables['TABBY_DIR'] = _tabby_location
	if ($null -ne $picked -and $executed -ceq $picked) {
		$startInfo.EnvironmentVariables['TABBY_QUERY'] = $query
	}
	if (-not [string]::IsNullOrEmpty($global:_tabby_previous)) {
		$startInfo.EnvironmentVariables['TABBY_PREVIOUS'] = $global:_tabby_previous
	}
	$global:_tabby_previous = $executed
	try {
		$process = [System.Diagnostics.Process]::Start($startInfo)
		if ($null -ne $process) {
			$process.Dispose()
		}
	} catch {
	}
}

# Skip lines that start with whitespace, and apply the PSReadLine
# AddToHistoryHandler so that lines it keeps out of the history file
# (by default, lines that look like they contain secrets) are not stored.
function global:_tabby_should_record {
	param([string] $Line)

	if ([string]::IsNullOrWhiteSpace($Line) -or [char]::IsWhiteSpace($Line[0])) {
		return $false
	}

	$handler = (Get-PSReadLineOption).AddToHistoryHandler
	if ($null -eq $handler) {
		return $true
	}

	try {
		$result = $handler.Invoke($Line)
	} catch {
		return $false
	}

	if ($result -is [bool]) {
		return $result
	}
	return "$result" -eq 'MemoryAndFile'
}

$env:TABBY_SHELL = 'powershell'

# Lets tabby doctor report the prediction source, because the gray inline
# suggestions of PSReadLine are easy to mistake for Tabby. PSReadLine 2.0
# has no PredictionSource, which leaves the variable empty.
function global:_tabby_record_prediction {
	try {
		$env:TABBY_PREDICTION = "$((Get-PSReadLineOption).PredictionSource)"
	} catch {
		$env:TABBY_PREDICTION = ''
	}
}

_tabby_record_prediction
_tabby_reset
$lastHistory = Get-History -Count 1 -ErrorAction SilentlyContinue
$global:_tabby_last_history_id = if ($null -eq $lastHistory) { -1 } else { $lastHistory.Id }

Set-PSReadLineKeyHandler -Chord Tab -ScriptBlock ${function:_tabby_complete} `
	-BriefDescription 'TabbyComplete' `
	-Description 'Complete using commands learned by tabby'
Set-PSReadLineKeyHandler -Chord Shift+Tab -ScriptBlock ${function:_tabby_complete_reverse} `
	-BriefDescription 'TabbyCompleteReverse' `
	-Description 'Complete backwards using commands learned by tabby'

if (-not $global:_tabby_prompt_installed) {
	$global:_tabby_original_prompt = ${function:prompt}
	function global:prompt {
		$succeeded = $?
		$savedExitCode = $global:LASTEXITCODE
		_tabby_record_prompt $succeeded
		_tabby_record_prediction
		$global:LASTEXITCODE = $savedExitCode
		& $global:_tabby_original_prompt
	}
	$global:_tabby_prompt_installed = $true
}
