#!/bin/bash

# Create ssl directory in current directory if not exists
# (This script assumes it's run from the fly-print-cloud/nginx/ssl directory or relative to it)
mkdir -p .

# Generate a self-signed certificate in the current directory
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout ./fly-print.key \
  -out ./fly-print.crt \
  -subj "/C=CN/ST=State/L=City/O=FlyPrint/CN=fly-print.local"

echo "Certificate generated: ./fly-print.crt, ./fly-print.key"
