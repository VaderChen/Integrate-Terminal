#!/usr/bin/env node

import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";

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
const resolvedOutputPath = resolve(outputPath);

mkdirSync(dirname(resolvedOutputPath), { recursive: true });
writeFileSync(resolvedOutputPath, `${JSON.stringify(metadata, null, 2)}\n`);
