#!/usr/bin/env bash

make build-cli

bin/ecommerce-cli user create -u "fred" -n "Fred Wellis" -e "fred@wellis.org" -p "secret" -r "admin"

bin/ecommerce-cli product create -n "Fancy Toothpaste" -d "It'll clean those teeth right out of your mouth" -p 25.00 -s "TPASTE-001" --stock 100
bin/ecommerce-cli product create -n "Zakarya's T-Shirt" -d "Stolen right off his back" -p 50.00 -s "SHIRT-001" --stock 1
