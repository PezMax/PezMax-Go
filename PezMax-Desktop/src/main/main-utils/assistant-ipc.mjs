import { createHash } from 'node:crypto'
import { join } from 'node:path'
import fs from 'node:fs'
import { createAssistantService } from './assistant-service.mjs'
import { createAgentProcess, resolveAgentRuntime } from './assistant-process.mjs'

export function isAssistantDocument(actual, expected) {
  try {
    const current = new URL(actual)
    const allowed = new URL(expected)
    return current.protocol === allowed.protocol && current.host === allowed.host && current.pathname === allowed.pathname
  } catch { return false }
}

export function assistantScope(context) {
  if (!context || typeof context.server !== 'string' || !context.server || context.server.length > 2048 ||
      typeof context.userId !== 'string' || !context.userId || context.userId.length > 160) {
    throw new Error('请先登录再使用找资料助手')
  }
  return createHash('sha256').update(JSON.stringify([context.server, context.userId])).digest('hex')
}

// Only the application's top frame may use this bridge, never embedded previews.
export function installAssistantHandlers({ ipcMain, getWindow, service, rendererUrl, catalogBaseUrl }) {
  const scopes = new Map()
  function release(owner) {
    service.cancelOwner(owner)
    const scope = scopes.get(owner)
    if (scope) service.forget(scope)
    scopes.delete(owner)
  }
  function sender(event) {
    const win = getWindow()
    if (!win || win.isDestroyed() || event.sender !== win.webContents || event.senderFrame !== win.webContents.mainFrame ||
        !isAssistantDocument(event.senderFrame.url, rendererUrl)) {
      throw new Error('助手仅可在应用主窗口中使用')
    }
    return event.sender.id
  }
  const actions = {
    getConfig: (scope) => service.publicConfig(scope),
    save: (scope, input) => service.save(scope, input),
    clear: (scope) => service.clear(scope),
    run: (scope, input, owner, progress) => service.run(scope, input, owner, progress),
    cancel: (_scope, input, owner) => service.cancel(input?.requestId, owner)
  }
  for (const [action, run] of Object.entries(actions)) {
    ipcMain.handle(`assistant:${action}`, async (event, context, input) => {
      try {
        const owner = sender(event)
        const scope = assistantScope(catalogBaseUrl ? { ...context, server: catalogBaseUrl } : context)
        if (scopes.has(owner) && scopes.get(owner) !== scope) release(owner)
        scopes.set(owner, scope)
        return { ok: true, data: await run(scope, input, owner, (data) => {
          if (scopes.get(owner) !== scope) return
          try {
            if (sender(event) === owner) event.sender.send('assistant:progress', { ...data, requestId: input.requestId })
          } catch { service.cancelOwner(owner) }
        }) }
      } catch (error) {
        // Never let Electron serialize arbitrary filesystem/provider exceptions.
        const message = error?.message || ''
        return { ok: false, error: /^(请|接口|模型|API Key|当前系统|对话|工具|请求|已停止|无法连接|助手|搜索|资源|年份|起始|平台)/.test(message)
          ? message : '助手操作失败，请检查设置或本机存储后重试',
          ...(error?.code === 'catalog_auth_expired' ? { code: error.code } : {}) }
      }
    })
  }
  ipcMain.handle('assistant:release', (event) => {
    try { release(sender(event)); return { ok: true } } catch { return { ok: false, error: '助手仅可在应用主窗口中使用' } }
  })
  return {
    dispose() { for (const owner of [...scopes.keys()]) release(owner) },
    attachWindow(win) {
      const owner = win.webContents.id
      win.webContents.on('did-start-navigation', (_event, _url, inPlace, isMainFrame) => {
        if (isMainFrame && !inPlace) release(owner)
      })
      win.webContents.once('destroyed', () => release(owner))
      win.webContents.on('render-process-gone', () => release(owner))
    }
  }
}

export function registerAssistantIpc({ ipcMain, getWindow, app, safeStorage, rendererUrl, catalogBaseUrl }) {
  const configPath = join(app.getPath('userData'), 'ptmj-assistant.json')
  const encryption = {
    isAvailable: () => safeStorage.isEncryptionAvailable() &&
      (process.platform !== 'linux' || safeStorage.getSelectedStorageBackend() !== 'basic_text'),
    encrypt: (key) => safeStorage.encryptString(key).toString('base64'),
    decrypt: (key) => safeStorage.decryptString(Buffer.from(key, 'base64'))
  }
  const service = createAssistantService({
    encryption,
    readConfig() {
      try {
        if (fs.statSync(configPath).size > 1024 * 1024) return {}
        const parsed = JSON.parse(fs.readFileSync(configPath, 'utf8'))
        return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
      } catch { return {} }
    },
    writeConfig(config) {
      const temporary = `${configPath}.tmp`
      fs.writeFileSync(temporary, JSON.stringify(config), { encoding: 'utf8', mode: 0o600 })
      fs.renameSync(temporary, configPath)
    },
    catalogBaseUrl,
    runAgent: createAgentProcess({ resolveRuntime: () => resolveAgentRuntime({
      isPackaged: app.isPackaged, resourcesPath: process.resourcesPath, appPath: app.getAppPath()
    }) })
  })
  return installAssistantHandlers({ ipcMain, getWindow, service, rendererUrl, catalogBaseUrl })
}
