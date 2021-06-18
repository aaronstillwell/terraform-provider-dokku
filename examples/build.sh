#!/bin/bash
cd ../
make install
cd examples
rm .terraform.lock.hcl
terraform init