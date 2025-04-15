#!/bin/sh

LINE=$(awk '$1=="port:"{print $2}' ./config/glance.yml)
PORT="${LINE:-8080}"

wget --spider -q http://localhost:$PORT/api/healthz
