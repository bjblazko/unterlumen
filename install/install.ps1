# Installs Unterlumen on Windows, with the helper programs it uses (ffmpeg,
# exiftool, cwebp), and starts it. Running it again updates it. The photo
# folder is chosen in the app afterwards.
#
#   irm https://huepattl.de/unterlumen/install.ps1 | iex
#
# $env:UNTERLUMEN_VERSION = '0.13.0'   install that version instead of the newest
# $env:UNTERLUMEN_SKIP_TOOLS = '1'     leave the helper programs alone
#
# Everything happens in Install-Unterlumen, so a download cut short runs nothing.

function Install-Unterlumen {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'   # the progress bar slows downloads in Windows PowerShell
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    $repo = 'https://github.com/bjblazko/unterlumen'
    $webpVersion = '1.6.0'

    $version = $env:UNTERLUMEN_VERSION
    if (-not $version) {
        # The newest release, read from where GitHub's "latest" link points.
        $req = [Net.WebRequest]::Create("$repo/releases/latest")
        $req.AllowAutoRedirect = $false
        $resp = $req.GetResponse()
        $version = ($resp.Headers['Location'] -split '/v')[-1]
        $resp.Close()
    }

    $work = Join-Path ([IO.Path]::GetTempPath()) ("unterlumen-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $work | Out-Null
    try {
        $name = "unterlumen_${version}_windows_amd64.zip"
        Write-Host "Downloading Unterlumen $version..."
        Invoke-WebRequest "$repo/releases/download/v$version/$name" -OutFile "$work\$name"
        Invoke-WebRequest "$repo/releases/download/v$version/checksums.txt" -OutFile "$work\checksums.txt"
        $line = Select-String -Path "$work\checksums.txt" -Pattern " $([regex]::Escape($name))$" | Select-Object -First 1
        $want = if ($line) { ($line.Line -split ' ')[0] } else { '' }
        $got = (Get-FileHash "$work\$name" -Algorithm SHA256).Hash.ToLower()
        if (-not $want -or $want -ne $got) {
            throw "the download is not the file the release lists. Nothing was installed."
        }
        Expand-Archive "$work\$name" -DestinationPath $work -Force

        if ($env:UNTERLUMEN_SKIP_TOOLS -ne '1') { Install-UnterlumenTools $work $webpVersion }

        & "$work\unterlumen.exe" -desktop-install
        if ($LASTEXITCODE -ne 0) { throw "the installer stopped with an error." }

        if (-not $env:CI) {
            Start-Process "$env:LOCALAPPDATA\Unterlumen\launch.bat" -WindowStyle Hidden
        }
        Write-Host "Unterlumen $version is installed. It opens in a window; choose your photo folder there."
    }
    catch {
        Write-Host "Unterlumen could not be installed: $($_.Exception.Message)" -ForegroundColor Red
    }
    finally {
        Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
    }
}

function Install-UnterlumenTools($work, $webpVersion) {
    if (Get-Command winget -ErrorAction SilentlyContinue) {
        foreach ($tool in @(@('ffmpeg', 'Gyan.FFmpeg'), @('exiftool', 'OliverBetz.ExifTool'))) {
            if (-not (Get-Command $tool[0] -ErrorAction SilentlyContinue)) {
                Write-Host "Installing $($tool[0])..."
                winget install --id $tool[1] -e --silent --accept-source-agreements --accept-package-agreements | Out-Null
            }
        }
        # What winget put on the PATH, for the app started below.
        $env:Path = [Environment]::GetEnvironmentVariable('Path', 'Machine') + ';' + [Environment]::GetEnvironmentVariable('Path', 'User')
    }
    else {
        Write-Host 'winget was not found. Install ffmpeg and exiftool yourself; Unterlumen says in Settings which ones it finds.'
    }

    # cwebp has no winget package; it goes where the app looks for its own tools.
    $tools = Join-Path $env:APPDATA 'Unterlumen\tools'
    if (-not (Get-Command cwebp -ErrorAction SilentlyContinue) -and -not (Test-Path "$tools\cwebp.exe")) {
        Write-Host 'Downloading cwebp (WebP export)...'
        New-Item -ItemType Directory -Force -Path $tools | Out-Null
        $zip = "libwebp-$webpVersion-windows-x64.zip"
        Invoke-WebRequest "https://storage.googleapis.com/downloads.webmproject.org/releases/webp/$zip" -OutFile "$work\$zip"
        Expand-Archive "$work\$zip" -DestinationPath $work -Force
        Copy-Item "$work\libwebp-$webpVersion-windows-x64\bin\cwebp.exe" "$tools\cwebp.exe"
    }
}

Install-Unterlumen
