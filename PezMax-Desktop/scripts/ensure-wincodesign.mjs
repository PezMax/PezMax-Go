/**
 * Pre-seeds electron-builder's winCodeSign cache on Windows.
 *
 * Why: app-builder extracts winCodeSign-2.6.0.7z with 7za, which tries to create
 * macOS symlinks (darwin/10.12/lib/*.dylib). Without SeCreateSymbolicLinkPrivilege
 * (admin or Windows Developer Mode) extraction fails, electron-builder aborts and
 * the PE icon/version resources are never written to the app executable.
 *
 * This script extracts the archive with `darwin` excluded (irrelevant on Windows)
 * into the cache layout app-builder expects, so it skips its own extraction.
 *
 * Remove this script once electron-builder no longer uses the legacy winCodeSign bundle.
 */
import { spawnSync } from 'node:child_process'
import { createWriteStream, existsSync, mkdirSync, readFileSync, renameSync, rmSync } from 'node:fs'
import { join } from 'node:path'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import { fileURLToPath } from 'node:url'

const RELEASE = 'winCodeSign-2.6.0'
const ARCHIVE = `${RELEASE}.7z`
const projectRoot = fileURLToPath(new URL('..', import.meta.url))

if (process.platform !== 'win32') {
  process.exit(0)
}

const cacheRoot =
  process.env.ELECTRON_BUILDER_CACHE?.trim() ||
  join(process.env.LOCALAPPDATA || join(process.env.USERPROFILE || '.', 'AppData', 'Local'), 'electron-builder', 'Cache')
const artifactDir = join(cacheRoot, 'winCodeSign')
// app-builder's rcedit command looks for the versioned directory (unlike its
// generic download-artifact command, which uses the bare artifact name).
const targetDir = join(artifactDir, RELEASE)

const requiredFiles = [
  join(targetDir, 'rcedit-x64.exe'),
  join(targetDir, 'windows-10', 'x64', 'signtool.exe')
]

if (requiredFiles.every(existsSync)) {
  console.log(`[winCodeSign] cache ready: ${targetDir}`)
  process.exit(0)
}

function getDownloadBaseUrl() {
  const override = process.env.ELECTRON_BUILDER_BINARIES_DOWNLOAD_OVERRIDE_URL?.trim()
  if (override) {
    return `${override.replace(/\/+$/, '')}/`
  }

  const pkg = JSON.parse(readFileSync(join(projectRoot, 'package.json'), 'utf8'))
  const mirror =
    process.env.NPM_CONFIG_ELECTRON_BUILDER_BINARIES_MIRROR ||
    process.env.npm_config_electron_builder_binaries_mirror ||
    process.env.npm_package_config_electron_builder_binaries_mirror ||
    process.env.ELECTRON_BUILDER_BINARIES_MIRROR ||
    pkg.config?.electron_builder_binaries_mirror ||
    'https://github.com/electron-userland/electron-builder-binaries/releases/download/'

  return `${mirror.replace(/\/+$/, '')}/`
}

async function downloadArchive(archivePath) {
  const url = `${getDownloadBaseUrl()}${RELEASE}/${ARCHIVE}`
  console.log(`[winCodeSign] downloading ${url}`)
  const response = await fetch(url, { redirect: 'follow' })
  if (!response.ok || response.body == null) {
    throw new Error(`[winCodeSign] download failed: HTTP ${response.status} ${response.statusText}`)
  }

  const partialPath = `${archivePath}.part`
  await pipeline(Readable.fromWeb(response.body), createWriteStream(partialPath))
  renameSync(partialPath, archivePath)
}

function extractArchive(archivePath) {
  const sevenZip = join(projectRoot, 'node_modules', '7zip-bin', 'win', 'x64', '7za.exe')
  if (!existsSync(sevenZip)) {
    throw new Error(`[winCodeSign] 7za not found: ${sevenZip} (run npm install first)`)
  }

  rmSync(targetDir, { recursive: true, force: true })
  mkdirSync(targetDir, { recursive: true })

  const result = spawnSync(
    sevenZip,
    ['x', '-bd', '-xr!darwin', archivePath, `-o${targetDir}`],
    { stdio: 'inherit' }
  )
  if (result.status !== 0) {
    throw new Error(`[winCodeSign] extraction failed with exit code ${result.status}`)
  }
}

mkdirSync(artifactDir, { recursive: true })
const archivePath = join(artifactDir, ARCHIVE)
if (!existsSync(archivePath)) {
  await downloadArchive(archivePath)
}

extractArchive(archivePath)

const missing = requiredFiles.filter(file => !existsSync(file))
if (missing.length > 0) {
  throw new Error(`[winCodeSign] cache incomplete, missing:\n${missing.join('\n')}`)
}

console.log(`[winCodeSign] cache ready: ${targetDir}`)
