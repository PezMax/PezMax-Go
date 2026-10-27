import { spawn } from 'node:child_process'
import fs from 'node:fs'
import { isAbsolute, join, resolve } from 'node:path'

const MAX_INPUT_BYTES = 512 * 1024
const MAX_LINE_BYTES = 8 * 1024 * 1024
const MAX_OUTPUT_BYTES = 32 * 1024 * 1024

export function agentEnvironment(environment = process.env) {
  const result = { ...environment }
  for (const key of Object.keys(result)) {
    if (/^(PYTHON|LANGSMITH|LANGCHAIN)/i.test(key) || /^(OPENAI|ANTHROPIC)_API_KEY$/i.test(key)) delete result[key]
  }
  return { ...result, PYTHONIOENCODING: 'utf-8', PYTHONUTF8: '1', LANGSMITH_TRACING: 'false', LANGCHAIN_TRACING_V2: 'false' }
}

export function resolveAgentRuntime({ isPackaged, resourcesPath, appPath, environment = process.env,
  platform = process.platform, arch = process.arch, exists = fs.existsSync, readFile = fs.readFileSync }) {
  if (isPackaged) {
    const root = join(resourcesPath, 'agent')
    const file = join(root, platform === 'win32' ? 'pezmax-agent.exe' : 'pezmax-agent')
    let manifest
    try { manifest = JSON.parse(readFile(join(root, 'runtime.json'), 'utf8')) } catch { /* require a matching runtime */ }
    if (!exists(file) || manifest?.protocolVersion !== 1 || manifest?.engine !== 'langgraph' || manifest?.platform !== platform || manifest?.arch !== arch) {
      throw new Error('助手模块缺失或版本不匹配，请重新安装完整客户端')
    }
    return { file, args: [], cwd: root }
  }
  const root = resolve(appPath, '../PezMax-Agent')
  const file = environment.PEZMAX_AGENT_PYTHON || join(root, '.venv', platform === 'win32' ? 'Scripts/python.exe' : 'bin/python')
  if (!isAbsolute(file) || !exists(file)) throw new Error('助手模块尚未安装，请先运行 npm run agent:setup')
  return { file, args: ['-m', 'pezmax_agent'], cwd: root }
}

// One worker per request. Secrets travel only in stdin; cancel kills the worker.
export function createAgentProcess({ resolveRuntime, spawnImpl = spawn, environment = process.env }) {
  return function runAgent(payload, { signal, onProgress = () => {} } = {}) {
    if (signal?.aborted) return Promise.reject(new Error('已停止生成'))
    const input = `${JSON.stringify(payload)}\n`
    if (Buffer.byteLength(input, 'utf8') > MAX_INPUT_BYTES) return Promise.reject(new Error('请求内容过长，请开始新对话'))
    let runtime
    try { runtime = resolveRuntime() } catch (error) { return Promise.reject(error) }
    return new Promise((resolveResult, rejectResult) => {
      let child, buffered = Buffer.alloc(0), total = 0, terminal = null, failure = null, settled = false
      const finish = (error, data) => {
        if (settled) return
        settled = true
        signal?.removeEventListener('abort', abort)
        if (error) rejectResult(error)
        else resolveResult(data)
      }
      const stop = (error) => {
        if (settled) return
        failure ||= error
        child.kill()
      }
      const abort = () => stop(new Error('已停止生成'))
      const invalid = () => stop(new Error('助手模块返回格式不正确，请更新客户端后重试'))
      try {
        child = spawnImpl(runtime.file, runtime.args, {
          cwd: runtime.cwd, env: agentEnvironment(environment), windowsHide: true, shell: false,
          stdio: ['pipe', 'pipe', 'pipe']
        })
      } catch { finish(new Error('助手模块无法启动，请检查安装或运行 npm run agent:setup')); return }
      signal?.addEventListener('abort', abort, { once: true })
      child.on('error', () => finish(new Error('助手模块无法启动，请检查安装或运行 npm run agent:setup')))
      child.stderr.on('data', () => {})
      child.stdin.on('error', () => stop(new Error('助手模块通信中断，请重试')))
      child.stdout.on('data', chunk => {
        if (failure || settled) return
        total += chunk.length
        if (total > MAX_OUTPUT_BYTES) { stop(new Error('助手返回内容过大，请缩小搜索范围')); return }
        buffered = Buffer.concat([buffered, chunk])
        let newline
        while ((newline = buffered.indexOf(10)) >= 0) {
          if (newline > MAX_LINE_BYTES) { invalid(); return }
          const line = buffered.subarray(0, newline).toString('utf8')
          buffered = buffered.subarray(newline + 1)
          let event
          try { event = JSON.parse(line) } catch { invalid(); return }
          if (!event || typeof event !== 'object' || Array.isArray(event) || terminal || event.version !== 1 || event.requestId !== payload.requestId) { invalid(); return }
          if (event.type === 'progress' && event.data && typeof event.data === 'object') {
            try { onProgress(event.data) } catch { invalid(); return }
          } else if (event.type === 'result' && event.data && typeof event.data.content === 'string' && Array.isArray(event.data.messages) && Array.isArray(event.data.resources) && Array.isArray(event.data.reports)) {
            terminal = event
          } else if (event.type === 'error' && typeof event.error === 'string' && event.error.length <= 500) {
            terminal = event
          } else { invalid(); return }
        }
        if (buffered.length > MAX_LINE_BYTES) invalid()
      })
      child.on('close', (code) => {
        if (failure) { finish(failure); return }
        if (terminal?.type === 'error') {
          const error = new Error(terminal.error)
          if (terminal.code === 'catalog_auth_expired') error.code = terminal.code
          finish(error)
        } else if (terminal?.type === 'result' && code === 0 && buffered.length === 0) finish(null, terminal.data)
        else finish(new Error('助手模块异常退出，请检查安装后重试'))
      })
      if (signal?.aborted) abort()
      else child.stdin.end(input, 'utf8')
    })
  }
}
