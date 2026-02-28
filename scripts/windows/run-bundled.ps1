param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Args
)

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$depsDir = Join-Path $baseDir "deps"

if (Test-Path (Join-Path $depsDir "bin")) {
    $env:PATH = (Join-Path $depsDir "bin") + ";" + $env:PATH
}

if (Test-Path (Join-Path $depsDir "tessdata")) {
    $env:TESSDATA_PREFIX = (Join-Path $depsDir "tessdata")
}

$exe = Join-Path $baseDir "VideoTools.exe"
& $exe @Args
