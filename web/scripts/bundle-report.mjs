import { readFile, stat } from "node:fs/promises"
import { resolve, relative } from "node:path"
import process from "node:process"
import { gzipSync } from "node:zlib"

const distDirectory = resolve(process.cwd(), "dist")
const manifestPath = resolve(distDirectory, ".vite/manifest.json")
const manifest = JSON.parse(await readFile(manifestPath, "utf8"))

function assetPath(file) {
  const path = resolve(distDirectory, file)
  const relativePath = relative(distDirectory, path)
  if (relativePath.startsWith("..")) {
    throw new Error(`manifest asset is outside dist: ${file}`)
  }
  return path
}

async function assetSize(file) {
  const path = assetPath(file)
  const contents = await readFile(path)
  const fileStats = await stat(path)
  return {
    rawBytes: fileStats.size,
    gzipBytes: gzipSync(contents).byteLength,
  }
}

function addSize(total, size) {
  total.rawBytes += size.rawBytes
  total.gzipBytes += size.gzipBytes
}

const jsFiles = new Set()
const cssFiles = new Set()
const chunks = {}

function logicalChunkKey(entry) {
  if (entry.src) return entry.src
  if (entry.name) return `chunk:${entry.name}`
  throw new Error(`manifest JavaScript record has no stable logical owner: ${entry.file}`)
}

const javascriptEntries = Object.values(manifest)
  .filter((entry) => entry.file.endsWith(".js"))
  .map((entry) => ({ entry, logicalKey: logicalChunkKey(entry) }))
  .sort((a, b) => a.logicalKey.localeCompare(b.logicalKey))

for (const { entry, logicalKey } of javascriptEntries) {
  if (chunks[logicalKey]) {
    throw new Error(`duplicate manifest logical owner: ${logicalKey}`)
  }
  const css = await Promise.all((entry.css ?? []).slice().sort().map(assetSize))
  jsFiles.add(entry.file)
  for (const file of entry.css ?? []) cssFiles.add(file)

  chunks[logicalKey] = {
    entryKind: entry.isEntry ? "entry" : entry.isDynamicEntry ? "dynamic-entry" : "chunk",
    js: await assetSize(entry.file),
    css,
  }
}

const jsTotal = { fileCount: jsFiles.size, rawBytes: 0, gzipBytes: 0 }
for (const file of [...jsFiles].sort()) addSize(jsTotal, await assetSize(file))

const cssTotal = { fileCount: cssFiles.size, rawBytes: 0, gzipBytes: 0 }
for (const file of [...cssFiles].sort()) addSize(cssTotal, await assetSize(file))

process.stdout.write(`${JSON.stringify({ chunks, totals: { js: jsTotal, css: cssTotal } }, null, 2)}\n`)
