#!/usr/bin/env node
// Launcher: runs the prebuilt NestedTerm binary that matches this machine.
"use strict";

const { spawn } = require("child_process");
const fs = require("fs");
const path = require("path");

const ARCH = { x64: "linux-x64", arm64: "linux-arm64" };

function fail(msg) {
  process.stderr.write("nestedterm: " + msg + "\n");
  process.exit(1);
}

if (process.platform !== "linux") {
  fail("only Linux is supported (this is " + process.platform + ").");
}
const dir = ARCH[process.arch];
if (!dir) {
  fail("unsupported CPU architecture: " + process.arch + " (supported: x64, arm64).");
}

const bin = path.join(__dirname, "..", "dist", dir, "nestedterm");
if (!fs.existsSync(bin)) {
  fail("binary not found at " + bin + "; try reinstalling the package.");
}
try {
  fs.accessSync(bin, fs.constants.X_OK);
} catch (_) {
  try {
    fs.chmodSync(bin, 0o755);
  } catch (err) {
    fail("binary is not executable and could not be fixed: " + err.message);
  }
}

// stdio is inherited so the child owns the TTY. Terminal-generated signals
// (Ctrl+C, window resize) already reach the child because it shares our
// process group; only signals sent to this process need forwarding.
const child = spawn(bin, process.argv.slice(2), { stdio: "inherit" });

for (const sig of ["SIGTERM", "SIGHUP"]) {
  process.on(sig, () => child.kill(sig));
}

child.on("error", (err) => fail("failed to start: " + err.message));
child.on("exit", (code, signal) => {
  if (signal) {
    process.removeAllListeners(signal);
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code === null ? 1 : code);
});
