param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Args
)

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$depsDir = Join-Path $baseDir "deps"

if (Test-Path (Join-Path $depsDir "bin")) {
    $env:PATH = (Join-Path $depsDir "bin") + ";" + $env:PATH
}

if (Test-Path (Join-Path $depsDir "lib\\gstreamer-1.0")) {
    $env:GST_PLUGIN_PATH = (Join-Path $depsDir "lib\\gstreamer-1.0")
    $env:GST_PLUGIN_SYSTEM_PATH_1_0 = (Join-Path $depsDir "lib\\gstreamer-1.0")
}

$gstScanner = Join-Path $depsDir "bin\\gst-plugin-scanner.exe"
if (Test-Path $gstScanner) {
    $env:GST_PLUGIN_SCANNER = $gstScanner
}

$gioModules = Join-Path $depsDir "lib\\gio\\modules"
if (Test-Path $gioModules) {
    $env:GIO_EXTRA_MODULES = $gioModules
}

if (Test-Path (Join-Path $depsDir "tessdata")) {
    $env:TESSDATA_PREFIX = (Join-Path $depsDir "tessdata")
}

$exe = Join-Path $baseDir "VideoTools.exe"
& $exe @Args
