#!/usr/bin/env node

const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const https = require("node:https");
const { pipeline } = require("node:stream/promises");

const AdmZip = require("adm-zip");
const tar = require("tar");

const REPO_OWNER = "operator-kit";
const REPO_NAME = "hs-cli";

function resolveTarget() {
  const platformMap = {
    linux: "linux",
    darwin: "darwin",
    win32: "windows"
  };
  const archMap = {
    x64: "amd64",
    arm64: "arm64"
  };

  const osName = platformMap[process.platform];
  const archName = archMap[process.arch];
  if (!osName || !archName) {
    throw new Error(`Unsupported platform/arch: ${process.platform}/${process.arch}`);
  }

  return {
    os: osName,
    arch: archName,
    archiveExt: osName === "windows" ? ".zip" : ".tar.gz",
    binaryName: osName === "windows" ? "hs.exe" : "hs"
  };
}

function packageVersion() {
  const pkgPath = path.join(__dirname, "..", "package.json");
  const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
  return String(pkg.version || "").trim();
}

function isDevVersion(version) {
  return (
    !version ||
    version === "0.0.0-development" ||
    version === "0.0.0" ||
    version.includes("dev")
  );
}

function releaseAssetURL(version, target) {
  const tag = `v${version}`;
  const asset = `hs_${version}_${target.os}_${target.arch}${target.archiveExt}`;
  return {
    tag,
    asset,
    url: `https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${tag}/${asset}`
  };
}

function downloadFile(url, destination, redirects = 5) {
  return new Promise((resolve, reject) => {
    https.get(url, (response) => {
      if (
        response.statusCode &&
        response.statusCode >= 300 &&
        response.statusCode < 400 &&
        response.headers.location
      ) {
        if (redirects <= 0) {
          reject(new Error(`Too many redirects while downloading ${url}`));
          return;
        }
        response.resume();
        resolve(downloadFile(response.headers.location, destination, redirects - 1));
        return;
      }

      if (response.statusCode !== 200) {
        const code = response.statusCode || "unknown";
        response.resume();
        reject(new Error(`Download failed (${code}): ${url}`));
        return;
      }

      const file = fs.createWriteStream(destination);
      pipeline(response, file)
        .then(() => resolve())
        .catch(reject);
    }).on("error", reject);
  });
}

async function extractArchive(archivePath, target, distDir) {
  if (target.archiveExt === ".zip") {
    const zip = new AdmZip(archivePath);
    zip.extractAllTo(distDir, true);
    return;
  }
  await tar.x({
    file: archivePath,
    cwd: distDir
  });
}

function findFile(rootDir, fileName) {
  const entries = fs.readdirSync(rootDir, { withFileTypes: true });
  for (const entry of entries) {
    const entryPath = path.join(rootDir, entry.name);
    if (entry.isFile() && entry.name === fileName) {
      return entryPath;
    }
    if (entry.isDirectory()) {
      const nested = findFile(entryPath, fileName);
      if (nested) {
        return nested;
      }
    }
  }
  return "";
}

async function install() {
  const version = packageVersion();
  if (isDevVersion(version)) {
    console.log("Skipping hs binary download for development package version.");
    return;
  }

  const target = resolveTarget();
  const release = releaseAssetURL(version, target);
  const distDir = path.join(__dirname, "..", "dist", `${target.os}-${target.arch}`);
  await fs.promises.mkdir(distDir, { recursive: true });

  const tempArchivePath = path.join(
    os.tmpdir(),
    `hs-${target.os}-${target.arch}-${Date.now()}${target.archiveExt}`
  );

  console.log(`Downloading hs ${release.tag} (${target.os}/${target.arch})...`);
  await downloadFile(release.url, tempArchivePath);
  await extractArchive(tempArchivePath, target, distDir);
  fs.unlinkSync(tempArchivePath);

  const binaryPath = path.join(distDir, target.binaryName);
  if (!fs.existsSync(binaryPath)) {
    const discovered = findFile(distDir, target.binaryName);
    if (!discovered) {
      throw new Error(`Binary ${target.binaryName} not found in downloaded archive.`);
    }
    fs.copyFileSync(discovered, binaryPath);
  }

  if (target.binaryName === "hs") {
    await fs.promises.chmod(binaryPath, 0o755);
  }

  console.log(`Installed hs binary to ${binaryPath}`);
}

install().catch((err) => {
  console.error(`Failed to install hs binary: ${err.message}`);
  process.exit(1);
});
