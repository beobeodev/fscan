const https = require("https");
const http = require("http");
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");
const zlib = require("zlib");

const pkg = require("./package.json");
const version = `v${pkg.version}`;

const REPO = "beobeodev/fscan";
const BINARY = "fscan";

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};

const ARCH_MAP = {
  x64: "amd64",
  arm64: "arm64",
};

function getAssetName() {
  const platform = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];

  if (!platform || !arch) {
    throw new Error(
      `Unsupported platform: ${process.platform}/${process.arch}`
    );
  }

  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `${BINARY}_${version.replace("v", "")}_${platform}_${arch}.${ext}`;
}

function download(url) {
  return new Promise((resolve, reject) => {
    const client = url.startsWith("https") ? https : http;
    client
      .get(url, { headers: { "User-Agent": "fscan-npm" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return download(res.headers.location).then(resolve).catch(reject);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => resolve(Buffer.concat(chunks)));
        res.on("error", reject);
      })
      .on("error", reject);
  });
}

async function extractTarGz(buffer, destDir) {
  const tmpFile = path.join(destDir, "tmp.tar.gz");
  fs.writeFileSync(tmpFile, buffer);
  execSync(`tar xzf "${tmpFile}" -C "${destDir}"`, { stdio: "ignore" });
  fs.unlinkSync(tmpFile);
}

async function extractZip(buffer, destDir) {
  const tmpFile = path.join(destDir, "tmp.zip");
  fs.writeFileSync(tmpFile, buffer);
  // Use PowerShell on Windows, unzip on Unix
  if (process.platform === "win32") {
    execSync(
      `powershell -Command "Expand-Archive -Path '${tmpFile}' -DestinationPath '${destDir}' -Force"`,
      { stdio: "ignore" }
    );
  } else {
    execSync(`unzip -o "${tmpFile}" -d "${destDir}"`, { stdio: "ignore" });
  }
  fs.unlinkSync(tmpFile);
}

async function main() {
  const assetName = getAssetName();
  const url = `https://github.com/${REPO}/releases/download/${version}/${assetName}`;
  const binDir = path.join(__dirname, "bin");

  console.log(`Downloading fscan ${version}...`);

  const buffer = await download(url);

  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  if (assetName.endsWith(".tar.gz")) {
    await extractTarGz(buffer, binDir);
  } else {
    await extractZip(buffer, binDir);
  }

  // Make binary executable on Unix
  const binaryPath = path.join(binDir, BINARY);
  if (process.platform !== "win32" && fs.existsSync(binaryPath)) {
    fs.chmodSync(binaryPath, 0o755);
  }

  console.log(`fscan ${version} installed successfully.`);
}

main().catch((err) => {
  console.error(`Failed to install fscan: ${err.message}`);
  console.error(
    "You can download manually from: https://github.com/beobeodev/fscan/releases"
  );
  process.exit(1);
});
