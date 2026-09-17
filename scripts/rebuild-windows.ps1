[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$projectDir = Split-Path -Parent $PSScriptRoot
$packageDir = Join-Path $projectDir "dist\owl-windows-amd64"
$owlExe = Join-Path $packageDir "owl.exe"
$newExe = Join-Path $packageDir "owl.exe.new"
$backupExe = Join-Path $packageDir "owl.exe.bak"
$mediaExe = Join-Path $packageDir "MediaServer\MediaServer.exe"
$ffmpegExe = Join-Path $packageDir "ffmpeg.exe"
$mediaFFmpegExe = Join-Path $packageDir "MediaServer\ffmpeg.exe"
$runDir = Join-Path $packageDir "run"

Write-Host "[1/4] Building owl.exe..."
$env:GOSUMDB = "sum.golang.org"
$env:GOTOOLCHAIN = "auto"
Push-Location $projectDir
try {
    & go build -trimpath -o $newExe ./main.go
    if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
}
finally { Pop-Location }

Write-Host "[2/4] Stopping the packaged services..."
$targets = @($owlExe, $mediaExe)
Get-CimInstance Win32_Process | Where-Object {
    $_.ExecutablePath -and ($targets -contains $_.ExecutablePath)
} | ForEach-Object {
    Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Milliseconds 500

Write-Host "[3/4] Replacing owl.exe..."
if (Test-Path -LiteralPath $backupExe) { Remove-Item -LiteralPath $backupExe -Force }
if (Test-Path -LiteralPath $owlExe) { Move-Item -LiteralPath $owlExe -Destination $backupExe }
try {
    Move-Item -LiteralPath $newExe -Destination $owlExe
}
catch {
    if (Test-Path -LiteralPath $backupExe) { Move-Item -LiteralPath $backupExe -Destination $owlExe }
    throw
}

Write-Host "[4/4] Starting owl and MediaServer..."
if (Test-Path -LiteralPath $ffmpegExe) {
    Copy-Item -LiteralPath $ffmpegExe -Destination $mediaFFmpegExe -Force
}
New-Item -ItemType Directory -Path $runDir -Force | Out-Null
$env:OWL_ONE_CLICK = "1"
$process = Start-Process -FilePath $owlExe `
    -WorkingDirectory $packageDir `
    -WindowStyle Hidden `
    -RedirectStandardOutput (Join-Path $runDir "owl.out") `
    -RedirectStandardError (Join-Path $runDir "owl.err") `
    -PassThru

Write-Host "Done. owl PID: $($process.Id)"
Write-Host "Backup: $backupExe"
