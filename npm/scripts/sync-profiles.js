#!/usr/bin/env node
"use strict";

/**
 * prepack: copy repo-root profiles/*.example.json into npm/profiles/ so the npm
 * tarball ships the example tenant profiles next to bin/ (#92). The Go binary
 * also embeds them; this keeps `<pkg>/profiles/` discoverable on disk.
 * Fails when no example is available, so a publish never silently drops them.
 */

const fs = require("fs");
const path = require("path");

const pkgRoot = path.join(__dirname, "..");
const repoProfiles = path.join(pkgRoot, "..", "profiles");
const dest = path.join(pkgRoot, "profiles");
const SUFFIX = ".example.json";

function listExamples(dir) {
  if (!fs.existsSync(dir)) return [];
  return fs
    .readdirSync(dir)
    .filter((n) => n.endsWith(SUFFIX) && fs.statSync(path.join(dir, n)).isFile())
    .sort();
}

function main() {
  const names = listExamples(repoProfiles);
  if (names.length === 0) {
    const existing = listExamples(dest);
    if (existing.length > 0) {
      process.stderr.write(`[yunxiao-cli] ${repoProfiles} not found; keeping npm/profiles/ (${existing.join(", ")})\n`);
      return;
    }
    throw new Error(`no ${SUFFIX} found under ${repoProfiles} or ${dest}`);
  }
  fs.rmSync(dest, { recursive: true, force: true });
  fs.mkdirSync(dest, { recursive: true });
  for (const n of names) {
    fs.copyFileSync(path.join(repoProfiles, n), path.join(dest, n));
  }
  process.stderr.write(`[yunxiao-cli] synced profiles/: ${names.join(", ")}\n`);
}

if (require.main === module) {
  try {
    main();
  } catch (err) {
    console.error(`sync-profiles failed: ${err && err.message ? err.message : err}`);
    process.exit(1);
  }
}

module.exports = { main, listExamples };
