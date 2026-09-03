#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";

const [outputPath, version, commit, tag, buildState, sourceUrl] = process.argv.slice(2);

if (![outputPath, version, commit, tag, buildState, sourceUrl].every((value) => typeof value === "string" && value.trim())) {
  console.error("用法：node scripts/write-build-metadata.mjs <output> <version> <commit> <tag> <buildState> <sourceUrl>");
  process.exit(1);
}

const metadata = {
  product: "IntegTERM",
  version,
  commit,
  tag,
  buildState,
  sourceUrl,
};
mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(outputPath, `${JSON.stringify(metadata, null, 2)}\n`);
