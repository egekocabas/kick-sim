import { spawn, spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repositoryRoot = path.resolve(webRoot, "..");
const workspace = path.join(webRoot, ".e2e-workspace");
const binary = path.join(webRoot, "dist", process.platform === "win32" ? "kick-sim-e2e.exe" : "kick-sim-e2e");
const receiverAddress = "127.0.0.1:14322";
const studioAddress = "127.0.0.1:14321";

if (path.dirname(workspace) !== webRoot) throw new Error("refusing to clean an unexpected e2e workspace path");
fs.rmSync(workspace, { recursive: true, force: true });
fs.mkdirSync(path.dirname(binary), { recursive: true });

function run(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, stdio: "inherit" });
  if (result.status !== 0) process.exit(result.status ?? 1);
}

run("npm", ["run", "build"], webRoot);
run("go", ["build", "-trimpath", "-tags", "studio_embed", "-o", binary, "./cmd/kick-sim"], repositoryRoot);
run(binary, ["--workspace", workspace, "init"], repositoryRoot);

const configPath = path.join(workspace, "config.yaml");
const configuration = fs.readFileSync(configPath, "utf8");
const configured = configuration.replace(
  "http://127.0.0.1:3000/webhooks/kick",
  `http://${receiverAddress}/webhooks/kick`,
);
if (configured === configuration) throw new Error("could not configure the e2e receiver URL");
fs.writeFileSync(configPath, configured, { mode: 0o644 });

const children = [];
function start(command, args, cwd) {
  const child = spawn(command, args, { cwd, stdio: "inherit" });
  children.push(child);
  child.once("exit", (code, signal) => {
    if (!stopping) {
      console.error(`${command} exited unexpectedly (${code ?? signal})`);
      stop(1);
    }
  });
  return child;
}

let stopping = false;
function stop(code = 0) {
  if (stopping) return;
  stopping = true;
  for (const child of children) {
    if (!child.killed) child.kill("SIGTERM");
  }
  setTimeout(() => process.exit(code), 250);
}

process.on("SIGINT", () => stop(0));
process.on("SIGTERM", () => stop(0));
process.on("exit", () => {
  for (const child of children) {
    if (!child.killed) child.kill("SIGTERM");
  }
});

start(
  "go",
  [
    "run",
    "./examples/receiver",
    "-listen",
    receiverAddress,
    "-public-key",
    path.join(workspace, "keys", "public-key.pem"),
  ],
  repositoryRoot,
);
start(binary, ["--workspace", workspace, "studio", "--no-open", "--address", studioAddress], repositoryRoot);

await new Promise(() => {});
