#!/usr/bin/env bash

# This test data is fully human-made, no AI generated images or text.

# Make sure we are running the latest code changes
make cli

CLI="bin/ecommerce-cli"

# Create test user
$CLI user create -u "fred" -n "Fred Wellis" -e "fred@wellis.org" -p "secret" -r "admin"

# Populate test user details
$CLI user card add \
	--nickname "Big Card" \
	--username "fred" \
	--cardholder-name "Fred Wellis" \
	--card-number 4754799498192087 \
	--exp-month 4 \
	--exp-year 2032 \
	--set-default
$CLI user address add \
	--label "Home" \
	--username "fred" \
	--full-name "Fred Wellis" \
	--line1 "100 SE Frog Avenue" \
	--city "Frogtown" \
	--postal-code "235783" \
	--state "Florida" \
	--country "US" \
	--set-default

# Create test products
$CLI product create -n "Fancy Toothpaste" -d "It'll clean those teeth right out of your mouth" -p 25.00 -s "TPASTE-001" --stock 100
$CLI product create -n "Zakarya's T-Shirt" -d "Stolen right off his back" -p 50.00  -s "SHIRT-001" --stock 1
$CLI product create -n "Colormatic Logo Pillow" -d "A durable, Colormatic-themed throw pillow for everyday use." -p 15.00 -s "CLPILLO-001" --stock 100

# Add product media
$CLI product media-upload -s "CLPILLO-001" -f "assets/demo/products/Colormatic Logo Pillow/Colormatic Logo Pillow.webp"
$CLI product media-upload -s "CLPILLO-001" -f "assets/demo/products/Colormatic Logo Pillow/Colormatic Logo.webp"

$CLI product media-upload -s "TPASTE-001" -f "assets/demo/products/Fancy Toothpaste/Fancy Toothpaste.webp"


$CLI product publish -s "CLPILLO-001"
$CLI product publish -s "TPASTE-001"