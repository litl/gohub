#!/bin/bash

>&2 echo "stderr test"
echo "repo is $1"
echo "project is $2"
echo "branch is $3"
sleep 5
echo "type is $4"
echo "ref is $5"
echo "env is $6"
exit 1
