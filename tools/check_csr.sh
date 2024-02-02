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

match_csr() {
    MY_CSR=$1
    MY_PEM=$2

    PUB=$(openssl req -in "$MY_CSR" -noout -pubkey)
    if [ "$MY_PEM" == "$PUB" ]; then
        success "Public keys are matching"
    else
        error "Error: the CSR public key does not match the com.tesla.3p.public-key.pem"
        echo "Public key from CSR:"
        echo "$PUB"
        echo "Public key from com.tesla.3p.public-key.pem"
        echo "$MY_PEM"
    fi
}


CSR="$1"
DN="$(openssl req -in "$1" -subject -noout)"
HOST="$(echo $DN | sed 's/subject=[\/]*CN\s*=\s*//')"

echo "----"
echo "CSR: $1"

if [ -z "$HOST" ]; then
    warning "ERROR: hostname is not valid"
else
    success "Host: $HOST"
    PEM=$(curl -sS "https://$HOST/.well-known/appspecific/com.tesla.3p.public-key.pem")
    if [ -z "$PEM" ]; then
        error "Could not retrieve https://$HOST/.well-known/appspecific/com.tesla.3p.public-key.pem"
    else
        match_csr "$CSR" "$PEM"
    fi
fi
