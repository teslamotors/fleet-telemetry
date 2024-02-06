#!/bin/bash

c_red="\033[0;31m"
c_green="\033[0;32m"
c_yellow="\033[0;33m"
c_reset="\033[0m"

success() {
        echo -e "${c_green}$1${c_reset}"
}

warning() {
        echo -e "${c_yellow}$1${c_reset}"
}

error() {
        echo -e "${c_red}$1${c_reset}"
}

die() {
    >&2 echo -e "${c_red}$*${c_reset}"
    exit 1
}

CONFIG=$1

if [ ! -f "$CONFIG" ]; then
  die "could not find fleet-telemetry client config json file: $CONFIG"
fi

HOSTNAME=$(jq -r ".hostname" "$CONFIG")
CA=$(jq -r ".ca" "$CONFIG")
PORT=$(jq -r '.port // 443' "$CONFIG")

CA_CERT_FILE=$(mktemp)
SERVER_CERT_FILE=$(mktemp)
TMP_SERVER_CERT_FILE=$(mktemp)
echo "$CA" > "$CA_CERT_FILE"

echo | openssl s_client -connect "$HOSTNAME:$PORT" -servername "$HOSTNAME" -showcerts 2>/dev/null > "$TMP_SERVER_CERT_FILE"
openssl x509 -in "$TMP_SERVER_CERT_FILE" -outform PEM > "$SERVER_CERT_FILE"

if openssl verify -CAfile "$CA_CERT_FILE" "$SERVER_CERT_FILE"; then
    success "The server certificate is valid."
else
  if openssl verify -partial_chain -CAfile "$CA_CERT_FILE" "$SERVER_CERT_FILE"; then
      warning "The server certificate has a valid partial chain, and may work with the root chain."
  else
      error "The server certificate is invalid."
  fi
fi

rm "$CA_CERT_FILE"
rm "$SERVER_CERT_FILE"
rm "$TMP_SERVER_CERT_FILE"
