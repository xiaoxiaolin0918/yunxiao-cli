#!/usr/bin/env node
"use strict";

/**
 * postinstall / manual installer for yunxiao-cli (Feishu-style).
 * Download order:
 *   1) bundled releases/<archive> (dev/offline)
 *   2) YUNXIAO_CLI_DOWNLOAD_BASE/<archive>
 *   3) GitHub Release: https://github.com/${REPO}/releases/download/v${VERSION}/<archive>
 *   4) China proxy: https://ghproxy.net/https://github.com/...
 * Then verify sha256, extract binary to bin/ and skills/ to package root.
 */

const fs = require("fs");
const path = require("path");
const os = require("os");
const crypto = require("crypto");
const { execFileSync } = require("child_process");

const pkg = require("../package.json");
const VERSION = String(pkg.version).replace(/-.*$/, "");
const BINARY_NAME = "yunxiao";
const ARCHIVE_PREFIX = "yunxiao-cli";
const DEFAULT_GITHUB_REPO = "xiaoxiaolin0918/yunxiao-cli";

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];
const isWindows = process.platform === "win32";

const pkgRoot = path.join(__dirname, "..");
const binDir = path.join(pkgRoot, "bin");
const dest = path.join(binDir, BINARY_NAME + (isWindows ? ".exe" : ""));
const skillsDest = path.join(pkgRoot, "skills");

function log(msg) {
  process.stderr.write(`[yunxiao-cli] ${msg}\n`);
}

function resolveArchiveName(version, platformName, archName) {
  const extension = platformName === "windows" ? ".zip" : ".tar.gz";
  return `${ARCHIVE_PREFIX}-${version}-${platformName}-${archName}${extension}`;
}

function githubRepo() {
  const fromEnv = (process.env.YUNXIAO_CLI_GITHUB_REPO || "").trim();
  if (fromEnv) return fromEnv.replace(/^\/+|\/+$/g, "");
  return DEFAULT_GITHUB_REPO;
}

function githubReleaseUrl(archiveName) {
  const repo = githubRepo();
  return `https://github.com/${repo}/releases/download/v${VERSION}/${archiveName}`;
}

function proxyUrls(primaryUrl) {
  // Optional China-friendly mirrors of the GitHub asset URL.
  const urls = [];
  const ghproxy = `https://ghproxy.net/${primaryUrl}`;
  urls.push(ghproxy);
  // npmmirror github release mirror (best-effort; may 404 if not synced)
  try {
    const u = new URL(primaryUrl);
    // https://github.com/OWNER/REPO/releases/download/vX/file
    // → https://cdn.jsdelivr.net/gh is not for releases; skip jsdelivr
    if (u.hostname === "github.com") {
      urls.push(`https://mirror.ghproxy.com/${primaryUrl}`);
    }
  } catch (_) {}
  return urls;
}

function binaryLooksGood() {
  if (!fs.existsSync(dest)) return false;
  try {
    const out = execFileSync(dest, ["--version"], {
      stdio: ["ignore", "pipe", "ignore"],
      encoding: "utf8",
      timeout: 15000,
    });
    return String(out).includes(VERSION);
  } catch (_) {
    return false;
  }
}

function getExpectedChecksum(archiveName) {
  const checksumsPath = path.join(pkgRoot, "checksums.txt");
  if (!fs.existsSync(checksumsPath)) {
    throw new Error(`[SECURITY] checksums.txt not found at ${checksumsPath}`);
  }
  const content = fs.readFileSync(checksumsPath, "utf8");
  for (const line of content.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const idx = trimmed.indexOf("  ");
    if (idx === -1) continue;
    const hash = trimmed.slice(0, idx).trim();
    const name = trimmed.slice(idx + 2).trim();
    if (name === archiveName) return hash;
  }
  throw new Error(`Checksum entry not found for ${archiveName}`);
}

function verifyChecksum(archivePath, expectedHash) {
  if (typeof expectedHash !== "string" || !/^[0-9a-f]{64}$/i.test(expectedHash)) {
    throw new Error("[SECURITY] Expected checksum must be a 64-char hex SHA-256 digest");
  }
  const hash = crypto.createHash("sha256");
  const fd = fs.openSync(archivePath, "r");
  try {
    const buf = Buffer.alloc(64 * 1024);
    let bytesRead;
    while ((bytesRead = fs.readSync(fd, buf, 0, buf.length, null)) > 0) {
      hash.update(buf.subarray(0, bytesRead));
    }
  } finally {
    fs.closeSync(fd);
  }
  const actual = hash.digest("hex");
  if (actual.toLowerCase() !== expectedHash.toLowerCase()) {
    throw new Error(
      `[SECURITY] Checksum mismatch for ${path.basename(archivePath)}: expected ${expectedHash} but got ${actual}`
    );
  }
}

function isCurlVersionSupported(versionOutput) {
  const match = String(versionOutput).match(/^\s*curl\s+(\d+)\.(\d+)\.(\d+)/i);
  if (!match) return false;
  const major = parseInt(match[1], 10);
  const minor = parseInt(match[2], 10);
  return major > 7 || (major === 7 && minor >= 70);
}

let _curlSupportsSslRevokeBestEffort;
function curlSupportsSslRevokeBestEffort() {
  if (_curlSupportsSslRevokeBestEffort !== undefined) {
    return _curlSupportsSslRevokeBestEffort;
  }
  try {
    const output = execFileSync("curl", ["--version"], {
      stdio: ["ignore", "pipe", "ignore"],
      encoding: "utf8",
      timeout: 5000,
    });
    _curlSupportsSslRevokeBestEffort = isCurlVersionSupported(output);
  } catch (_) {
    _curlSupportsSslRevokeBestEffort = false;
  }
  return _curlSupportsSslRevokeBestEffort;
}

function download(url, destPath) {
  const args = [
    "--fail",
    "--location",
    "--silent",
    "--show-error",
    "--connect-timeout",
    "10",
    "--max-time",
    "180",
    "--max-redirs",
    "5",
    "--output",
    destPath,
  ];
  if (isWindows && curlSupportsSslRevokeBestEffort()) {
    args.unshift("--ssl-revoke-best-effort");
  }
  args.push(url);
  execFileSync("curl", args, { stdio: ["ignore", "ignore", "pipe"] });
}

function tryDownload(url, destPath) {
  try {
    log(`Downloading ${url}`);
    download(url, destPath);
    if (!fs.existsSync(destPath) || fs.statSync(destPath).size === 0) {
      throw new Error("empty download");
    }
    return true;
  } catch (err) {
    log(`Download failed: ${err && err.message ? err.message : err}`);
    try {
      fs.unlinkSync(destPath);
    } catch (_) {}
    return false;
  }
}

function extractZipWindows(archivePath, destDir) {
  const psOpts = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-Command"];
  const psStdio = ["ignore", "inherit", "inherit"];
  const psEnv = {
    ...process.env,
    YUNXIAO_CLI_ARCHIVE: archivePath,
    YUNXIAO_CLI_DEST: destDir,
  };
  try {
    const dotnet =
      "$ErrorActionPreference='Stop';" +
      "Add-Type -AssemblyName System.IO.Compression.FileSystem;" +
      "[System.IO.Compression.ZipFile]::ExtractToDirectory($env:YUNXIAO_CLI_ARCHIVE,$env:YUNXIAO_CLI_DEST)";
    execFileSync("powershell.exe", [...psOpts, dotnet], { stdio: psStdio, env: psEnv });
  } catch (primaryErr) {
    try {
      const cmdlet =
        "$ErrorActionPreference='Stop';" +
        "Expand-Archive -LiteralPath $env:YUNXIAO_CLI_ARCHIVE -DestinationPath $env:YUNXIAO_CLI_DEST -Force";
      execFileSync("powershell.exe", [...psOpts, cmdlet], { stdio: psStdio, env: psEnv });
    } catch (secondErr) {
      try {
        execFileSync("tar", ["-xf", archivePath, "-C", destDir], { stdio: psStdio });
      } catch (fallbackErr) {
        throw new Error(
          `Failed to extract ${archivePath}. ` +
            `.NET ZipFile: ${primaryErr.message}. ` +
            `Expand-Archive: ${secondErr.message}. ` +
            `tar: ${fallbackErr.message}`
        );
      }
    }
  }
}

function copyDirRecursive(src, dst) {
  fs.mkdirSync(dst, { recursive: true });
  for (const ent of fs.readdirSync(src, { withFileTypes: true })) {
    const from = path.join(src, ent.name);
    const to = path.join(dst, ent.name);
    if (ent.isDirectory()) {
      copyDirRecursive(from, to);
    } else if (ent.isSymbolicLink()) {
      try {
        fs.symlinkSync(fs.readlinkSync(from), to);
      } catch (_) {
        fs.copyFileSync(from, to);
      }
    } else {
      fs.copyFileSync(from, to);
    }
  }
}

function findExtractedBinary(tmpDir) {
  const binaryName = BINARY_NAME + (isWindows ? ".exe" : "");
  const direct = path.join(tmpDir, binaryName);
  if (fs.existsSync(direct)) return direct;
  for (const ent of fs.readdirSync(tmpDir, { withFileTypes: true })) {
    if (!ent.isDirectory()) continue;
    const nested = path.join(tmpDir, ent.name, binaryName);
    if (fs.existsSync(nested)) return nested;
  }
  throw new Error(`Extracted binary ${binaryName} not found under ${tmpDir}`);
}

function findExtractedSkills(tmpDir, binaryPath) {
  const beside = path.join(path.dirname(binaryPath), "skills");
  if (fs.existsSync(beside) && fs.statSync(beside).isDirectory()) return beside;
  const root = path.join(tmpDir, "skills");
  if (fs.existsSync(root) && fs.statSync(root).isDirectory()) return root;
  return null;
}

/**
 * Resolve archive: bundled → DOWNLOAD_BASE → GitHub → proxies.
 */
function resolveArchivePath(archiveName, tmpDir) {
  const bundled = path.join(pkgRoot, "releases", archiveName);
  if (fs.existsSync(bundled)) {
    log(`Using bundled archive: releases/${archiveName}`);
    return bundled;
  }

  const archivePath = path.join(tmpDir, archiveName);
  const candidates = [];

  const base = (process.env.YUNXIAO_CLI_DOWNLOAD_BASE || "").trim().replace(/\/+$/, "");
  if (base) {
    if (!/^https:\/\//i.test(base)) {
      throw new Error(`YUNXIAO_CLI_DOWNLOAD_BASE must be an https:// URL (got: ${base})`);
    }
    candidates.push(`${base}/${archiveName}`);
  }

  const ghUrl = githubReleaseUrl(archiveName);
  candidates.push(ghUrl);
  for (const p of proxyUrls(ghUrl)) {
    candidates.push(p);
  }

  const errors = [];
  for (const url of candidates) {
    if (tryDownload(url, archivePath)) {
      return archivePath;
    }
    errors.push(url);
  }

  throw new Error(
    `Could not obtain ${archiveName}.\n` +
      `Tried:\n  - bundled: ${bundled}\n` +
      errors.map((u) => `  - ${u}`).join("\n") +
      `\nSet YUNXIAO_CLI_DOWNLOAD_BASE or YUNXIAO_CLI_GITHUB_REPO, or place the archive under releases/.`
  );
}

function install() {
  if (!platform || !arch) {
    throw new Error(`Unsupported platform: ${process.platform}-${process.arch}`);
  }

  if (binaryLooksGood()) {
    log(`Binary already present and reports v${VERSION}; skipping install`);
    return;
  }

  const archiveName = resolveArchiveName(VERSION, platform, arch);
  fs.mkdirSync(binDir, { recursive: true });

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "yunxiao-cli-"));
  try {
    const archivePath = resolveArchivePath(archiveName, tmpDir);
    const expectedHash = getExpectedChecksum(archiveName);
    log(`Verifying SHA-256 for ${archiveName}`);
    verifyChecksum(archivePath, expectedHash);

    const extractDir = path.join(tmpDir, "extract");
    fs.mkdirSync(extractDir, { recursive: true });
    log(`Extracting ${archiveName}`);
    if (isWindows) {
      extractZipWindows(archivePath, extractDir);
    } else {
      execFileSync("tar", ["-xzf", archivePath, "-C", extractDir], { stdio: "ignore" });
    }

    const extractedBinary = findExtractedBinary(extractDir);
    fs.copyFileSync(extractedBinary, dest);
    if (!isWindows) fs.chmodSync(dest, 0o755);

    const skillsSrc = findExtractedSkills(extractDir, extractedBinary);
    if (skillsSrc) {
      log("Installing skills/ next to package root");
      fs.rmSync(skillsDest, { recursive: true, force: true });
      copyDirRecursive(skillsSrc, skillsDest);
    } else {
      log("Warning: skills/ not found in archive; yunxiao skills install may need a source checkout");
    }

    if (!binaryLooksGood()) {
      throw new Error(`Installed binary at ${dest} does not report version ${VERSION}`);
    }
    log(`${BINARY_NAME} v${VERSION} installed successfully -> ${dest}`);
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  // Skip heavy work during bare `npx` postinstall; run.js / wizard call us with YUNXIAO_CLI_RUN=1.
  const isNpxPostinstall =
    process.env.npm_command === "exec" && !process.env.YUNXIAO_CLI_RUN;
  if (isNpxPostinstall) {
    process.exit(0);
  }
  try {
    install();
  } catch (err) {
    console.error(`Failed to install ${BINARY_NAME}:`, err && err.message ? err.message : err);
    console.error(
      `\nTips:\n` +
        `  • Binaries come from GitHub Releases (default repo: ${DEFAULT_GITHUB_REPO}).\n` +
        `  • Override:\n` +
        `      export YUNXIAO_CLI_GITHUB_REPO=owner/repo\n` +
        `      export YUNXIAO_CLI_DOWNLOAD_BASE=https://example.com/path/to/archives\n` +
        `  • Or place archives under releases/ for offline install.\n` +
        `  • Retry: node "${path.join(__dirname, "install.js")}"\n`
    );
    process.exit(1);
  }
}

module.exports = {
  install,
  resolveArchiveName,
  getExpectedChecksum,
  verifyChecksum,
  binaryLooksGood,
  githubReleaseUrl,
  githubRepo,
  dest,
  VERSION,
  DEFAULT_GITHUB_REPO,
};
