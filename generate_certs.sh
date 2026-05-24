#!/bin/bash

# Directory to store certs
CERT_DIR="./certs"
mkdir -p "$CERT_DIR"

# SAN and Domain
DOMAIN="telemetryt.duckdns.org"

# 1. Generate Root CA
openssl genrsa -out "$CERT_DIR/vehicle_device.CA.key" 2048
openssl req -x509 -new -nodes -key "$CERT_DIR/vehicle_device.CA.key" -sha256 -days 1825 \
    -out "$CERT_DIR/vehicle_device.CA.cert" \
    -subj "/C=US/O=Test Co/CN=Tesla Motors Products CA"

# 2. Generate Server Key
openssl genrsa -out "$CERT_DIR/vehicle_device.app.key" 2048

# 3. Create CSR for Server
cat > "$CERT_DIR/server.conf" <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn

[dn]
C = US
O = Test Co
CN = app

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = $DOMAIN
DNS.2 = app
EOF

openssl req -new -key "$CERT_DIR/vehicle_device.app.key" -out "$CERT_DIR/server.csr" -config "$CERT_DIR/server.conf"

# 4. Sign Server Cert with Root CA
cat > "$CERT_DIR/server_ext.conf" <<EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = $DOMAIN
DNS.2 = app
EOF

openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/vehicle_device.CA.cert" -CAkey "$CERT_DIR/vehicle_device.CA.key" \
    -CAcreateserial -out "$CERT_DIR/vehicle_device.app.cert" -days 365 -sha256 \
    -extfile "$CERT_DIR/server_ext.conf"

# Clean up
rm "$CERT_DIR/server.csr" "$CERT_DIR/server.conf" "$CERT_DIR/server_ext.conf" "$CERT_DIR/vehicle_device.CA.srl"

echo "Certificates generated in $CERT_DIR"
ls -l "$CERT_DIR"
