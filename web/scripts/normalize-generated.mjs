import { readFile, writeFile } from "node:fs/promises";

const path = new URL("../src/api/generated/client.ts", import.meta.url);
const source = await readFile(path, "utf8");
await writeFile(path, `${source.trimEnd()}\n`);
