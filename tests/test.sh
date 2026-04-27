#!/usr/bin/env bash

./tests/test.dracula.sh
./tests/test.default.sh
python -m http.server -d tests/ 8182
