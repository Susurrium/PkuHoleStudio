@echo off
chcp 65001 >nul
cd /d "%~dp0"
title PkuHoleStudio
echo 正在启动 PkuHoleStudio，请稍候……
echo 浏览器打开后请保持此窗口开启；关闭窗口即可退出程序。
echo.
treehole-windows-amd64.exe web
echo.
echo PkuHoleStudio 已停止。按任意键关闭窗口。
pause >nul
