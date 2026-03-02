#!/usr/bin/env bash
# Generate a Graphviz dot file of dependencies between apps/* and pkg/* packages.
# Usage: ./scripts/depgraph.sh | dot -Tpng -o depgraph.png

set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

MODULE="monks.co"

echo 'digraph deps {'
echo '  rankdir=LR;'
echo '  node [shape=box, style=filled];'
echo '  /* default node color */'
echo '  node [fillcolor="#d4e6f1"];'
echo ''

# Collect app and pkg nodes
for app in apps/*/; do
  app="${app%/}"
  name="${app#apps/}"
  [[ "$name" == "README.md" ]] && continue
  echo "  \"apps/$name\" [fillcolor=\"#aed6f1\"];"
done
echo ''
for pkg in pkg/*/; do
  pkg="${pkg%/}"
  name="${pkg#pkg/}"
  echo "  \"pkg/$name\" [fillcolor=\"#a9dfbf\"];"
done
echo ''

# For each app, find all imports of monks.co/pkg/*
for app in apps/*/; do
  app="${app%/}"
  appname="${app#apps/}"
  [[ "$appname" == "README.md" ]] && continue

  # Get all imports from all .go files under this app
  { grep -rh '"monks.co/pkg/' "$app" --include='*.go' 2>/dev/null || true; } \
    | sed 's/.*"monks.co\/pkg\///' \
    | sed 's/".*//' \
    | cut -d/ -f1 \
    | sort -u \
    | while read -r pkg; do
        echo "  \"apps/$appname\" -> \"pkg/$pkg\";"
      done
done
echo ''

# For each pkg, find all imports of other monks.co/pkg/*
for pkg in pkg/*/; do
  pkg="${pkg%/}"
  pkgname="${pkg#pkg/}"

  { grep -rh '"monks.co/pkg/' "$pkg" --include='*.go' 2>/dev/null || true; } \
    | sed 's/.*"monks.co\/pkg\///' \
    | sed 's/".*//' \
    | cut -d/ -f1 \
    | sort -u \
    | while read -r dep; do
        [[ "$dep" == "$pkgname" ]] && continue
        echo "  \"pkg/$pkgname\" -> \"pkg/$dep\";"
      done
done

echo '}'
