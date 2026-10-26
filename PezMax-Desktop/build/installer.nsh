; 自定义 NSIS 卸载逻辑（electron-builder nsis.include）
; 模板默认清理安装目录、快捷方式、注册表与 %APPDATA%\PezMax（deleteAppDataOnUninstall），
; 但不覆盖 electron-updater 在 %LOCALAPPDATA% 下缓存的安装包与差分数据。

!include LogicLib.nsh
!include FileFunc.nsh

; 更新安装会以 "--updated" 参数复用卸载器（见模板 include/installUtil.nsh），
; 此时必须保留用户数据与更新缓存，只有真正卸载才清理。
!macro customUnInstall
  ClearErrors
  ${GetParameters} $R9
  ${GetOptions} $R9 "--updated" $R8
  ${If} ${Errors}
    ; 卸载器以 all-users 上下文运行，$LOCALAPPDATA 此时解析为 C:\ProgramData；
    ; 切换到当前用户后再删除 electron-updater 缓存。
    SetShellVarContext current
    RMDir /r "$LOCALAPPDATA\pezmax-updater"
    SetShellVarContext all
  ${EndIf}
!macroend

!ifdef BUILD_UNINSTALLER
Function un.onUninstSuccess
  ClearErrors
  ${GetParameters} $R9
  ${GetOptions} $R9 "--updated" $R8
  ${If} ${Errors}
    SetOutPath $TEMP
    ; 卸载器（含 elevate.exe 父进程）退出前会占用 $INSTDIR 作为工作目录，
    ; 进程内 RMDir 只能清空内容、无法删除目录本身。交给隐藏 cmd 在进程退出后轮询删除。
    ExecShell "" '"$SYSDIR\cmd.exe"' '/c cd /d "%TEMP%" & for /l %i in (1,1,15) do @(ping -n 2 127.0.0.1 >nul & rmdir /s /q "$INSTDIR" 2>nul & if not exist "$INSTDIR" exit /b)' SW_HIDE
  ${EndIf}
FunctionEnd
!endif
