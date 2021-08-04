#!/usr/bin/env bash
cd docs_markdown
mkdocs build
cd ..
cp -Rf docs_markdown/site/* docs/
rm -rf docs_markdown/site
