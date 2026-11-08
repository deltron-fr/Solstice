#!/bin/sh
set -eu

expected_program_id=${1:?expected Solana program ID is required}
expected_usdc_mint=${2:?expected USDC mint is required}
expected_domain=${3:?expected public domain is required}

cd /opt/solstice
test -f .env

configured_program_id=$(sed -n 's/^SOLSTICE_SOLANA_PROGRAM_ID=//p' .env | tail -n 1)
configured_usdc_mint=$(sed -n 's/^SOLSTICE_USDC_MINT=//p' .env | tail -n 1)
configured_domain=$(sed -n 's/^SOLSTICE_DOMAIN=//p' .env | tail -n 1)
configured_origin=$(sed -n 's/^SOLSTICE_ALLOWED_ORIGINS=//p' .env | tail -n 1)

if [ "$configured_program_id" != "$expected_program_id" ]; then
	echo "SOLSTICE_SOLANA_PROGRAM_ID in /opt/solstice/.env does not match the frontend image." >&2
	exit 1
fi

if [ "$configured_usdc_mint" != "$expected_usdc_mint" ]; then
	echo "SOLSTICE_USDC_MINT in /opt/solstice/.env does not match the frontend image." >&2
	exit 1
fi

if [ "$configured_domain" != "$expected_domain" ]; then
	echo "SOLSTICE_DOMAIN in /opt/solstice/.env does not match the deployment hostname." >&2
	exit 1
fi

if [ "$configured_origin" != "https://$expected_domain" ]; then
	echo "SOLSTICE_ALLOWED_ORIGINS in /opt/solstice/.env must be https://$expected_domain." >&2
	exit 1
fi

docker compose pull
docker compose up -d --pull always --wait --wait-timeout 120
