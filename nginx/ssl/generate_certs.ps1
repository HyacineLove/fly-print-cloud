# Create ssl directory if not exists
if (!(Test-Path -Path ".")) {
    New-Item -ItemType Directory -Path "."
}

# Generate a self-signed certificate using OpenSSL
# Requires Git for Windows or OpenSSL installed and in PATH
try {
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 `
      -keyout fly-print.key `
      -out fly-print.crt `
      -subj "/C=CN/ST=State/L=City/O=FlyPrint/CN=fly-print.local"
    
    Write-Host "Certificate generated successfully:"
    Write-Host "  - fly-print.crt"
    Write-Host "  - fly-print.key"
} catch {
    Write-Error "Failed to run openssl. Please ensure Git for Windows is installed and openssl is in your PATH."
    exit 1
}

