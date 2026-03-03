#!/usr/bin/env node

const fs = require("node:fs");
const path = require("node:path");
const { spawn } = require("node:child_process");

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

  const os = platformMap[process.platform];
  const arch = archMap[process.arch];
  if (!os || !arch) {
    throw new Error(
      `Unsupported platform/arch: ${process.platform}/${process.arch}`
    );
  }
  return { os, arch };
}

function resolveBinaryPath() {
  const target = resolveTarget();
  const exe = process.platform === "win32" ? "hs.exe" : "hs";
  return path.join(__dirname, "..", "dist", `${target.os}-${target.arch}`, exe);
}

function main() {
  const binaryPath = resolveBinaryPath();
  if (!fs.existsSync(binaryPath)) {
    console.error(
      "hs binary is not installed. Reinstall package: npm i -g @operatorkit/hs"
    );
    process.exit(1);
  }

  const child = spawn(binaryPath, process.argv.slice(2), {
    stdio: "inherit"
  });

  child.on("error", (err) => {
    console.error(`Failed to start hs: ${err.message}`);
    process.exit(1);
  });

  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 1);
  });
}

main();
