#!/bin/bash

set -e

PENDING_DIR=${GITHUB_WORKSPACE}/log/leaves/pending

if [ ! -d ${PENDING_DIR} ]; then
    echo "${PENDING_DIR} not found, exiting."
    exit 0
fi

FILES=$(ls -A ${PENDING_DIR})

if [ -z "${FILES}" ]; then
    echo "Error: ${PENDING_DIR} empty, this should not happen."
    exit 1
fi

for f in ${FILES}; do
    echo "Verifying ${PENDING_DIR}/${f}..."
    /tmp/verify_release --manifest ${PENDING_DIR}/${f}
    [ $? != 0 ] && exit 1 || echo Ok
done
