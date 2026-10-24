import { spawnSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { resolve, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../PezMax-Agent')
const python = join(root, '.venv', process.platform === 'win32' ? 'Scripts/python.exe' : 'bin/python')
function run(file, args) {
  const result = spawnSync(file, args, { cwd: root, stdio: 'inherit', windowsHide: true, shell: false })
  if (result.error) throw new Error(`Cannot start ${file}; install Python 3.11+ or run npm run agent:setup.`)
  if (result.status !== 0) process.exit(result.status || 1)
}
const action = process.argv[2]
if (action === 'setup') {
  if (!existsSync(python)) run(process.env.PEZMAX_AGENT_PYTHON || (process.platform === 'win32' ? 'python' : 'python3'), ['-m', 'venv', join(root, '.venv')])
  run(python, ['-m', 'pip', 'install', '-r', 'requirements.lock', '-e', '.[build]'])
} else {
  if (!existsSync(python)) throw new Error('Agent environment missing; run npm run agent:setup first.')
  if (action === 'test') run(python, ['-m', 'unittest', 'discover', '-s', 'tests', '-v'])
  else if (action === 'check') run(python, ['-m', 'pezmax_agent', '--check'])
  else if (action === 'build') run(python, ['scripts/build.py'])
  else throw new Error('Expected setup, test, check, or build')
}
