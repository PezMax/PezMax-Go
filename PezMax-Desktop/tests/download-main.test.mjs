import assert from 'node:assert/strict'
import fs from 'node:fs'
import http from 'node:http'
import os from 'node:os'
import path from 'node:path'
import { randomUUID } from 'node:crypto'
import test from 'node:test'
import { readFileSync } from 'node:fs'
import { downloadPartPaths, downloadURL, nextAvailableDownloadPath, safeDownloadName, strongETag } from '../src/main/main-utils/download-path.mjs'

const source = readFileSync(new URL('../src/main/index.js', import.meta.url), 'utf8')
const begin = source.indexOf("  ipcMain.handle('download-file-directly'")
const end = source.indexOf('  // =========================================================', begin)
assert.ok(begin >= 0 && end > begin)

test('main-process downloads preserve names and validate resumed content', async (t) => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'pezmax-download-test-'))
  const requests = []
  const server = http.createServer((request, response) => {
    requests.push({ url: request.url, range: request.headers.range, ifRange: request.headers['if-range'] })
    const current = request.url === '/resume' || request.url === '/wild' ? 'AAABBB' : 'fresh'
    const etag = request.url === '/resume' || request.url === '/wild' ? '"current"' : '"fresh"'
    const resumable = request.url === '/resume' && request.headers.range === 'bytes=3-' && request.headers['if-range'] === etag
    const invalidRange = request.url === '/wild' && !!request.headers.range
    const body = resumable || invalidRange ? 'BBB' : current
    const headers = {
      'Content-Length': Buffer.byteLength(body),
      ETag: etag
    }
    if (resumable) headers['Content-Range'] = 'bytes 3-5/6'
    if (invalidRange) headers['Content-Range'] = 'bytes 3-5/*'
    response.writeHead(resumable || invalidRange ? 206 : 200, headers)
    response.end(body)
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  t.after(async () => {
    await new Promise(resolve => server.close(resolve))
    const root = fs.realpathSync(os.tmpdir()) + path.sep
    assert.ok(fs.realpathSync(directory).startsWith(root))
    fs.rmSync(directory, { recursive: true, force: true })
  })

  let handler
  const install = new Function(
    'ipcMain', 'fs', 'loadSettings', 'app', 'join', 'dirname', 'dialog', 'net', 'console',
    'mainWindow', 'randomUUID', 'activeDownloads', 'downloadURL', 'safeDownloadName',
    'nextAvailableDownloadPath', 'downloadPartPaths', 'strongETag', source.slice(begin, end)
  )
  install(
    { handle(_name, callback) { handler = callback } }, fs,
    () => ({ downloadPath: directory, silentDownload: true }),
    { getPath() { return directory } }, path.join, path.dirname, {},
    { request(url) { return http.request(url) } }, { log() {}, warn() {}, error() {} },
    {}, randomUUID, new Set(), downloadURL, safeDownloadName,
    nextAvailableDownloadPath, downloadPartPaths, strongETag
  )
  const run = (route, fileName) => handler(
    { sender: { send() {} } },
    { url: `http://127.0.0.1:${server.address().port}${route}`, fileName, silent: true }
  )

  const first = await run('/file', '../report.pdf')
  assert.equal(first.filePath, path.join(directory, 'report.pdf'))
  assert.equal(first.savedPath, undefined)
  assert.equal(fs.readFileSync(first.filePath, 'utf8'), 'fresh')
  const second = await run('/file', 'report.pdf')
  assert.equal(second.filePath, path.join(directory, 'report (1).pdf'))
  assert.equal(fs.readFileSync(first.filePath, 'utf8'), 'fresh')

  const resumeURL = `http://127.0.0.1:${server.address().port}/resume`
  const resumeDestination = path.join(directory, 'resume.pdf')
  const { partPath, metaPath } = downloadPartPaths(resumeDestination, resumeURL)
  fs.writeFileSync(partPath, 'OLD')
  fs.writeFileSync(metaPath, JSON.stringify({ etag: '"stale"' }))
  const restarted = await run('/resume', 'resume.pdf')
  assert.equal(fs.readFileSync(restarted.filePath, 'utf8'), 'AAABBB')
  assert.ok(requests.some(request => request.range === 'bytes=3-' && request.ifRange === '"stale"'))

  fs.writeFileSync(partPath, 'AAA')
  fs.writeFileSync(metaPath, JSON.stringify({ etag: '"current"' }))
  fs.rmSync(resumeDestination)
  const continued = await run('/resume', 'resume.pdf')
  assert.equal(fs.readFileSync(continued.filePath, 'utf8'), 'AAABBB')
  assert.ok(requests.some(request => request.range === 'bytes=3-' && request.ifRange === '"current"'))
  const wildURL = `http://127.0.0.1:${server.address().port}/wild`
  const wildDestination = path.join(directory, 'wild.pdf')
  const wildPart = downloadPartPaths(wildDestination, wildURL)
  fs.writeFileSync(wildPart.partPath, 'AAA')
  fs.writeFileSync(wildPart.metaPath, JSON.stringify({ etag: '"current"' }))
  const wild = await run('/wild', 'wild.pdf')
  assert.equal(fs.readFileSync(wild.filePath, 'utf8'), 'AAABBB')
  assert.equal(requests.filter(request => request.url === '/wild').length, 2)

  const collision = path.join(directory, 'collision.pdf')
  const collisionResult = await handler(
    { sender: { send(channel) {
      if (channel === 'download-progress' && !fs.existsSync(collision)) fs.writeFileSync(collision, 'keep')
    } } },
    { url: `http://127.0.0.1:${server.address().port}/file`, fileName: 'collision.pdf', silent: true }
  )
  assert.equal(fs.readFileSync(collision, 'utf8'), 'keep')
  assert.equal(collisionResult.filePath, path.join(directory, 'collision (1).pdf'))
  await assert.rejects(() => handler({ sender: { send() {} } }, { url: 'file:///etc/passwd', fileName: 'x' }))
})
