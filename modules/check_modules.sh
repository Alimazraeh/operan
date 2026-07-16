#!/bin/bash
cd /Users/alimazraeh/ADRI/Operan/modules

for dir in 01-tenant-control-plane 02-identity-access 03-agent-orchestration 04-agent-registry 05 05-department-template-engine 07 07-memory-fabric 08 08-tool-execution 09 09-human-supervision 10 11 11-observability 12 14 15 16 17 18 21-experience-portal; do
  echo "=== $dir ==="
  if [ -f "$dir/go.mod" ]; then
    echo "HAS go.mod"
    echo "source_files=$(find $dir -name '*.go' ! -name '*_test.go' | wc -l)"
    echo "test_files=$(find $dir -name '*_test.go' | wc -l)"
    [ -f "$dir/main.go" ] && echo "HAS main.go" || echo "NO main.go"
    [ -f "$dir/Dockerfile" ] && echo "HAS Dockerfile" || echo "NO Dockerfile"
    [ -d "$dir/chart" ] && echo "HAS Helm chart" || echo "NO Helm chart"
    head -1 "$dir/go.mod"
  else
    echo "NO go.mod (not a Go module)"
  fi
done