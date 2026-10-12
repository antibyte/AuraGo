function Get-DecisionsRuntimeArchitecture {
    param([System.Runtime.InteropServices.Architecture]$Architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)

    switch ($Architecture) {
        ([System.Runtime.InteropServices.Architecture]::X64) { 'amd64'; break }
        ([System.Runtime.InteropServices.Architecture]::Arm64) { 'arm64'; break }
        ([System.Runtime.InteropServices.Architecture]::X86) { '386'; break }
        ([System.Runtime.InteropServices.Architecture]::Arm) { 'arm'; break }
        default { throw "Unsupported native runtime architecture: $Architecture" }
    }
}
