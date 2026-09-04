import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const numericVersionPattern = /^[0-9]+(?:\.[0-9]+)*$/;
const buildLabelPattern = /^[0-9]{4}$/;

function environmentValue(name) {
  return (process.env[name] ?? '').trim();
}

function resolveBuildTime() {
  const injectedTimestamp = environmentValue('BUILD_TIMESTAMP');
  if (injectedTimestamp) {
    const timestamp = new Date(injectedTimestamp);
    if (Number.isNaN(timestamp.getTime())) {
      throw new Error(`BUILD_TIMESTAMP 格式錯誤：${injectedTimestamp}`);
    }
    return { timestamp, source: 'BUILD_TIMESTAMP' };
  }

  const sourceDateEpoch = environmentValue('SOURCE_DATE_EPOCH');
  if (sourceDateEpoch) {
    if (!/^[0-9]+$/.test(sourceDateEpoch)) {
      throw new Error(`SOURCE_DATE_EPOCH 必須是 Unix 秒數：${sourceDateEpoch}`);
    }
    const timestamp = new Date(Number(sourceDateEpoch) * 1000);
    if (Number.isNaN(timestamp.getTime())) {
      throw new Error(`SOURCE_DATE_EPOCH 超出可用範圍：${sourceDateEpoch}`);
    }
    return { timestamp, source: 'SOURCE_DATE_EPOCH' };
  }

  return { timestamp: new Date(), source: 'system' };
}

function twoDigits(value) {
  return String(value).padStart(2, '0');
}

function validateVersion(name, value) {
  if (!numericVersionPattern.test(value)) {
    throw new Error(`${name} 格式錯誤：${value}`);
  }
}

function synchronizeProductVersion(marketingVersion) {
  const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
  const projectDirectory = path.resolve(scriptDirectory, '..');
  const configPath = path.join(projectDirectory, 'wails.json');
  const serviceVersionPath = path.join(projectDirectory, 'internal', 'version', 'version.json');
  const config = JSON.parse(fs.readFileSync(configPath, 'utf8'));

  config.info = {
    ...(config.info || {}),
    productVersion: marketingVersion,
  };

  fs.writeFileSync(configPath, `${JSON.stringify(config, null, 2)}\n`);
  fs.writeFileSync(serviceVersionPath, `${JSON.stringify({ productVersion: marketingVersion }, null, 2)}\n`);
}

try {
  const { timestamp, source } = resolveBuildTime();
  const year = twoDigits(timestamp.getFullYear() % 100);
  const month = twoDigits(timestamp.getMonth() + 1);
  const day = twoDigits(timestamp.getDate());
  const hour = twoDigits(timestamp.getHours());
  const minute = twoDigits(timestamp.getMinutes());

  const marketingVersion = environmentValue('APP_MARKETING_VERSION') || `1.${year}.${month}${day}`;
  const buildLabel = environmentValue('APP_BUILD_LABEL') || `${hour}${minute}`;
  const bundleVersion = environmentValue('APP_BUNDLE_VERSION') || `${marketingVersion}${buildLabel}`;

  validateVersion('APP_MARKETING_VERSION', marketingVersion);
  if (!buildLabelPattern.test(buildLabel)) {
    throw new Error(`APP_BUILD_LABEL 必須是 HHmm 四位數字：${buildLabel}`);
  }
  validateVersion('APP_BUNDLE_VERSION', bundleVersion);

  if (process.argv.includes('--sync')) {
    synchronizeProductVersion(marketingVersion);
  }

  process.stdout.write(JSON.stringify({
    marketingVersion,
    buildLabel,
    displayVersion: `${marketingVersion} build ${buildLabel}`,
    bundleVersion,
    timeSource: source,
  }));
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exit(1);
}
