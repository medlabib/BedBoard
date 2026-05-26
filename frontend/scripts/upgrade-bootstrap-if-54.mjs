#!/usr/bin/env node
import { execSync } from 'node:child_process';

function parseVersion(input) {
  const match = String(input).trim().match(/^(\d+)\.(\d+)\.(\d+)/);
  if (!match) return null;
  return { major: Number(match[1]), minor: Number(match[2]), patch: Number(match[3]) };
}

function isBootstrap54Plus(version) {
  if (!version) return false;
  if (version.major !== 5) return false;
  return version.minor >= 4;
}

function main() {
  const latestRaw = execSync('npm view bootstrap version', { encoding: 'utf8' }).trim();
  const latest = parseVersion(latestRaw);
  if (!latest) {
    console.error('Unable to parse latest bootstrap version:', latestRaw);
    process.exit(1);
  }

  if (!isBootstrap54Plus(latest)) {
    console.log(`Bootstrap ${latestRaw} is available; waiting for stable 5.4+.`);
    process.exit(0);
  }

  const target = `^${latest.major}.${latest.minor}.${latest.patch}`;
  console.log(`Bootstrap ${latestRaw} detected (5.4+). Upgrading dependency to ${target}...`);
  execSync(`npm install bootstrap@${target}`, { stdio: 'inherit' });
  console.log('Bootstrap upgraded. Run npm run build to validate SCSS compatibility.');
}

main();
