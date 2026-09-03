#!/usr/bin/env node

import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  existsSync,
  readFileSync,
  readdirSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { basename, join } from "node:path";

const projectRoot = ".";
const frontendRoot = "./frontend";
const noticePath = "./THIRD-PARTY-NOTICES.md";
const licensePath = "./THIRD-PARTY-LICENSES.txt";
const targetPlatforms = ["darwin/arm64", "windows/amd64", "linux/amd64"];
const licenseNamePattern = /^(?:licen[cs]e|copying|notice)(?:$|[._-])/i;

function run(command, args, options = {}) {
  const executable = process.platform === "win32" && command === "npm" ? "npm.cmd" : command;
  try {
    return execFileSync(executable, args, {
      cwd: projectRoot,
      encoding: "utf8",
      maxBuffer: 64 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"],
      ...options,
    }).trim();
  } catch (error) {
    const details = error.stderr?.toString().trim() || error.message;
    throw new Error(`${command} 執行失敗：${details}`);
  }
}

function normaliseText(value) {
  return value.replace(/\r\n?/g, "\n").trimEnd() + "\n";
}

function compareText(left, right) {
  if (left < right) {
    return -1;
  }
  if (left > right) {
    return 1;
  }
  return 0;
}

function findLicenseDocuments(packageDirectory) {
  if (!existsSync(packageDirectory)) {
    return [];
  }

  return readdirSync(packageDirectory)
    .filter((name) => licenseNamePattern.test(name))
    .map((name) => ({ name, path: join(packageDirectory, name) }))
    .filter(({ path }) => statSync(path).isFile())
    .sort((left, right) => compareText(left.name, right.name));
}

function collectGoPackages() {
  const packages = new Map();
  const template = "{{if .Module}}{{if not .Module.Main}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}{{end}}";

  for (const target of targetPlatforms) {
    const [goos, goarch] = target.split("/");
    const output = run("go", ["list", "-deps", "-f", template, "."], {
      env: {
        ...process.env,
        GOOS: goos,
        GOARCH: goarch,
        CGO_ENABLED: "1",
      },
    });

    for (const line of output.split("\n").filter(Boolean)) {
      const [name, version, directory] = line.split("|");
      if (!name || !directory) {
        throw new Error(`無法解析 Go 相依套件資料：${line}`);
      }

      const key = `${name}@${version}`;
      const existing = packages.get(key);
      if (existing) {
        existing.targets.add(target);
        continue;
      }

      packages.set(key, {
        ecosystem: "Go",
        name,
        version: version || "local replacement",
        declaredLicense: "See bundled license text",
        directory,
        targets: new Set([target]),
      });
    }
  }

  return [...packages.values()];
}

function formatDeclaredLicense(value) {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  if (value !== undefined && value !== null) {
    return JSON.stringify(value);
  }
  return "Not declared in package.json";
}

function collectNpmPackages() {
  const output = run("npm", ["--prefix", frontendRoot, "ls", "--omit=dev", "--all", "--parseable"]);
  const packages = new Map();

  for (const directory of output.split("\n").filter(Boolean)) {
    const packageJsonPath = join(directory, "package.json");
    if (basename(directory) === "frontend" || !existsSync(packageJsonPath)) {
      continue;
    }

    const packageJson = JSON.parse(readFileSync(packageJsonPath, "utf8"));
    if (!packageJson.name || !packageJson.version) {
      throw new Error(`npm 套件資料不完整：${packageJsonPath}`);
    }

    const key = `${packageJson.name}@${packageJson.version}`;
    if (!packages.has(key)) {
      packages.set(key, {
        ecosystem: "npm",
        name: packageJson.name,
        version: packageJson.version,
        declaredLicense: formatDeclaredLicense(packageJson.license ?? packageJson.licenses),
        directory,
        targets: new Set(targetPlatforms),
      });
    }
  }

  return [...packages.values()];
}

function escapeTableCell(value) {
  return String(value).replaceAll("|", "\\|").replaceAll("\n", " ");
}

const packages = [...collectGoPackages(), ...collectNpmPackages()].sort((left, right) =>
  compareText(
    [left.ecosystem, left.name, left.version].join("\0"),
    [right.ecosystem, right.name, right.version].join("\0"),
  ),
);
const documentsByHash = new Map();

for (const packageInfo of packages) {
  const documents = findLicenseDocuments(packageInfo.directory);
  if (documents.length === 0) {
    throw new Error(`${packageInfo.ecosystem} 套件 ${packageInfo.name}@${packageInfo.version} 找不到 LICENSE、COPYING 或 NOTICE 文件。`);
  }

  packageInfo.documentHashes = [];
  for (const document of documents) {
    const text = normaliseText(readFileSync(document.path, "utf8"));
    const hash = createHash("sha256").update(text).digest("hex");
    packageInfo.documentHashes.push(hash);

    const existing = documentsByHash.get(hash);
    if (existing) {
      existing.usedBy.add(`${packageInfo.ecosystem}: ${packageInfo.name}@${packageInfo.version}`);
      existing.filenames.add(document.name);
    } else {
      documentsByHash.set(hash, {
        hash,
        text,
        usedBy: new Set([`${packageInfo.ecosystem}: ${packageInfo.name}@${packageInfo.version}`]),
        filenames: new Set([document.name]),
      });
    }
  }
}

const documents = [...documentsByHash.values()].sort((left, right) => compareText(left.hash, right.hash));
const documentIds = new Map(documents.map((document, index) => [document.hash, `L${String(index + 1).padStart(3, "0")}`]));

const noticeLines = [
  "# Third-Party Notices",
  "",
  "This project includes third-party software. Each dependency remains subject to its own license terms.",
  "The complete, deduplicated license and notice texts are generated at build time and bundled in release artifacts as `THIRD-PARTY-LICENSES.txt`.",
  "",
  `Covered build targets: ${targetPlatforms.map((target) => `\`${target}\``).join(", ")}.`,
  "",
];

for (const ecosystem of ["Go", "npm"]) {
  noticeLines.push(`## ${ecosystem} Dependencies`, "", "| Package | Version | Declared license | Documents | Targets |", "| --- | --- | --- | --- | --- |");
  for (const packageInfo of packages.filter((entry) => entry.ecosystem === ecosystem)) {
    const ids = [...new Set(packageInfo.documentHashes.map((hash) => documentIds.get(hash)))].sort();
    noticeLines.push(
      `| ${escapeTableCell(packageInfo.name)} | ${escapeTableCell(packageInfo.version)} | ${escapeTableCell(packageInfo.declaredLicense)} | ${ids.join(", ")} | ${[...packageInfo.targets].sort().map((target) => `\`${target}\``).join(", ")} |`,
    );
  }
  noticeLines.push("");
}

const licenseLines = [
  "THIRD-PARTY LICENSE AND NOTICE TEXTS",
  "",
  "Each document below is stored once. The package list identifies every dependency that uses it.",
  "",
];

for (const document of documents) {
  const documentId = documentIds.get(document.hash);
  licenseLines.push(
    "================================================================================",
    `Document ${documentId}`,
    `SHA-256: ${document.hash}`,
    `Source filenames: ${[...document.filenames].sort().join(", ")}`,
    "Used by:",
    ...[...document.usedBy].sort(compareText).map((entry) => `- ${entry}`),
    "--------------------------------------------------------------------------------",
    document.text.trimEnd(),
    "",
  );
}

writeFileSync(noticePath, `${noticeLines.join("\n").trimEnd()}\n`);
writeFileSync(licensePath, `${licenseLines.join("\n").trimEnd()}\n`);
console.log(`已產生 ${packages.length} 個第三方套件、${documents.length} 份授權文件清冊。`);
