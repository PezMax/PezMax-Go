"""Build a native runtime on the target OS; no cross-compilation or user Python."""
import importlib.metadata
import json
import platform
import subprocess
import sys
from pathlib import Path

root = Path(__file__).resolve().parents[1]
subprocess.run([
    sys.executable, "-m", "PyInstaller", "--noconfirm", "--clean", "--onedir", "--name", "pezmax-agent",
    "--distpath", str(root / "dist"), "--workpath", str(root / "build" / "pyinstaller"),
    "--specpath", str(root / "build"), "--paths", str(root / "src"),
    "--collect-submodules", "langgraph", "--copy-metadata", "langgraph", "--copy-metadata", "langchain-core",
    str(root / "scripts" / "freeze_entry.py"),
], cwd=root, check=True)
destination = root / "dist" / "pezmax-agent"
runtime = destination / ("pezmax-agent.exe" if sys.platform == "win32" else "pezmax-agent")
check = subprocess.run([str(runtime), "--check"], capture_output=True, text=True, encoding="utf-8", check=True)
assert json.loads(check.stdout)["ready"] is True
architecture = {"amd64": "x64", "x86_64": "x64", "aarch64": "arm64"}.get(platform.machine().lower(), platform.machine().lower())
(destination / "runtime.json").write_text(json.dumps({
    "protocolVersion": 1, "platform": sys.platform, "arch": architecture,
    "engine": "langgraph", "engineVersion": importlib.metadata.version("langgraph"),
    "packageVersion": importlib.metadata.version("pezmax-agent"),
}, indent=2) + "\n", encoding="utf-8")
print(f"Agent runtime ready: {destination}")
