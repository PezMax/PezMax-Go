const fs = require('node:fs')
const path = require('node:path')
const { Arch } = require('electron-builder')

module.exports = async (context) => {
  const root = path.resolve(context.packager.projectDir, '../PezMax-Agent/dist/pezmax-agent')
  const platform = context.electronPlatformName
  const arch = Arch[context.arch]
  let manifest
  try { manifest = JSON.parse(fs.readFileSync(path.join(root, 'runtime.json'), 'utf8')) } catch { /* fail with build instructions */ }
  const executable = path.join(root, platform === 'win32' ? 'pezmax-agent.exe' : 'pezmax-agent')
  if (!fs.existsSync(executable) || manifest?.protocolVersion !== 1 || manifest?.engine !== 'langgraph' || manifest?.platform !== platform || manifest?.arch !== arch) {
    throw new Error(`Missing matching LangGraph runtime for ${platform}/${arch}. Run npm run agent:setup and npm run agent:build on the target OS/architecture before packaging.`)
  }
}
