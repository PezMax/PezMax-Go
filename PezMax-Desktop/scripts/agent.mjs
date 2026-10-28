import { spawnSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { resolve, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../PezMax-Agent')
const python = join(
    root,
    '.venv',
    process.platform === 'win32' ? 'Scripts/python.exe' : 'bin/python'
)
function run(file, args) {
    const result = spawnSync(file, args, {
        cwd: root,
        stdio: 'inherit',
        windowsHide: true,
        shell: false
    })
    if (result.error)
        throw new Error(`Cannot start ${file}; install Python 3.11+ or run npm run agent:setup.`)
    if (result.status !== 0) process.exit(result.status || 1)
}
const action = process.argv[2]
// Some Windows installs ship without the stdlib `venv` module; fail loudly
// instead of silently exiting 1, and fall back to the virtualenv package.
function createEnvironment(bootstrap) {
    const target = join(root, '.venv')
    const created = spawnSync(bootstrap, ['-m', 'venv', target], {
        cwd: root,
        stdio: 'inherit',
        windowsHide: true,
        shell: false
    })
    if (created.error)
        throw new Error(
            `Cannot start ${bootstrap}; install Python 3.11+ or run npm run agent:setup.`
        )
    if (created.status === 0) return
    const fallback = spawnSync(bootstrap, ['-m', 'virtualenv', '--version'], {
        cwd: root,
        stdio: 'ignore',
        windowsHide: true,
        shell: false
    })
    if (fallback.status !== 0)
        throw new Error(
            `${bootstrap} 缺少 venv 模块，且未安装 virtualenv。请安装完整 Python 3.11+，或执行 ${bootstrap} -m pip install virtualenv 后重试。`
        )
    run(bootstrap, ['-m', 'virtualenv', target])
}
if (action === 'setup') {
    if (!existsSync(python))
        createEnvironment(
            process.env.PEZMAX_AGENT_PYTHON || (process.platform === 'win32' ? 'python' : 'python3')
        )
    run(python, ['-m', 'pip', 'install', '-r', 'requirements.lock', '-e', '.[build]'])
} else {
    if (!existsSync(python))
        throw new Error('Agent environment missing; run npm run agent:setup first.')
    if (action === 'test') run(python, ['-m', 'unittest', 'discover', '-s', 'tests', '-v'])
    else if (action === 'check') run(python, ['-m', 'pezmax_agent', '--check'])
    else if (action === 'build') run(python, ['scripts/build.py'])
    else if (action === 'eval')
        run(python, ['-m', 'pezmax_agent.evaluation', ...process.argv.slice(3)])
    else throw new Error('Expected setup, test, check, build, or eval')
}
