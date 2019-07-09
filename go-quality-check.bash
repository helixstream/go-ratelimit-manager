#!/usr/bin/env bash

#Run on travis ci before actually tests.
#Causes travis ci build to fail if its not formatted correctly, vetted, linted, etc.

LINTERRORS=false
VETERRORS=false
IMPORTERRORS=false
TEST=""

TEST=$(gofmt -l ./..)
if [[ "$TEST" != "" ]]; then
    echo "GO not formatted correctly in "$TEST""
    exit 1
fi

go vet ./...
if [[ $? != 0 ]]; then
    VETERRORS=true
fi

for FILE in src/host/*.go;
do
    goimports -w "${FILE}"
    if [[ $? != 0 ]]; then
       IMPORTERRORS=true
    fi

    golint "-set_exit_status" "${FILE}"
    if [[ $? == 1 ]]; then
       LINTERRORS=true
    fi

done

if ${LINTERRORS}; then
    echo "golint failed. See above errors"
    exit 1
fi

if ${VETERRORS}; then
    echo "go vet failed. See above errors"
    exit 1
fi

if ${IMPORTERRORS}; then
    echo "go fmt failed. See above errors"
    exit 1
fi