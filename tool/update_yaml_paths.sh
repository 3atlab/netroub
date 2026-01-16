#!/bin/bash
# update_yaml_paths.sh
# Usage: ./update_yaml_paths.sh <logPath> <topoPath> <dataPath> <files...>
# Example: ./update_yaml_paths.sh ./log ./bgp_features/topo.yaml ./bgp_features/bgp.data example/*.json

set -e

if [ $# -lt 4 ]; then
    echo "Usage: $0 <logPath> <topoPath> <dataPath> <files...>"
    echo "Example: $0 ./log ./bgp_features/topo.yaml ./bgp_features/bgp.data example/*.json"
    exit 1
fi

LOG_PATH="$1"
TOPO_PATH="$2"
DATA_PATH="$3"
shift 3

for file in "$@"; do
    if [ -f "$file" ]; then
        echo "Updating: $file"
        # JSON format: "logPath" : "./value"
        sed -i "s|\"logPath\"[[:space:]]*:[[:space:]]*\"[^\"]*\"|\"logPath\" : \"${LOG_PATH}\"|g" "$file"
        sed -i "s|\"topo\"[[:space:]]*:[[:space:]]*\"[^\"]*\"|\"topo\":\"${TOPO_PATH}\"|g" "$file"
        sed -i "s|\"data\"[[:space:]]*:[[:space:]]*\"[^\"]*\"|\"data\": \"${DATA_PATH}\"|g" "$file"
        # YAML format: logPath: "./value"
        sed -i "s|^logPath:[[:space:]]*\"[^\"]*\"|logPath: \"${LOG_PATH}\"|g" "$file"
        sed -i "s|^topo:[[:space:]]*\"[^\"]*\"|topo: \"${TOPO_PATH}\"|g" "$file"
        sed -i "s|^data:[[:space:]]*\"[^\"]*\"|data: \"${DATA_PATH}\"|g" "$file"
    else
        echo "Warning: $file not found, skipping"
    fi
done

echo "Done."
