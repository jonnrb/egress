#!/bin/bash

### run me from the repo root ###

exec docker build -f hack/Dockerfile -t egress .
