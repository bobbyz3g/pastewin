# PasteWin

Windows 本地文字与图片剪贴板工具，使用 **Wails v3.0.0-beta.25 + Go + React / TypeScript + SQLite**。

## 直接体验

双击 `bin\pastewin.exe`。

1. 在记事本复制文本，或在截图工具、浏览器、画图等应用复制图片。
2. 保持光标在需要粘贴的位置，按 **Ctrl + Shift + V**。
3. 搜索或用 **↑ / ↓** 选择历史，按 **Enter** 粘贴回原窗口。
4. **Ctrl + Enter** 仅复制，**Esc** 先清空搜索，再次按下隐藏面板。
5. **Delete** 删除选中记录（文字和图片均支持）；搜索框有文字并处于输入焦点时只编辑搜索文字，弹窗内不删除记录，长按不会连续删除。

关闭面板会保留托盘和后台监听。在设置或托盘菜单中选择“退出”才会结束进程。初次直接启动时没有确定的粘贴目标，请使用“复制”，或者从目标应用按快捷键重新打开面板。

从其他应用重新唤起时，清空搜索、切回“全部”、关闭弹窗并选中最新记录，列表回到顶部，焦点进入搜索框。刚复制的图片若稍后才处理完成，在尚未开始操作时会自动选中；开始选择、输入、滚动或操作按钮后保留当前选择。删除选中项优先选择下一条，没有下一条时选择上一条。面板已在前台时重复按快捷键，以及粘贴失败后重新显示，都保留当前操作状态。

在设置中打开“开机自启”，登录 Windows 后会自动在托盘后台运行，不弹出面板；默认关闭，可随时关闭。该选项仅作用于当前用户，无需管理员权限。启动项使用当前 EXE 的绝对路径，手动移动便携版后需重新开启。若在 Windows“启动应用”中禁用了 PasteWin，或组织策略禁止自启，还需在系统中恢复允许；应用内开关表示启动项登记状态。

图片记录显示缩略图、尺寸和大小，点击“查看内容”查看原图。图片也支持收藏、删除、重启保留、Ctrl+Enter 复制和 Enter/双击粘贴。同一次复制包含文字和图片时保存为一条记录，默认复制/粘贴文字；预览中可单独选择“复制图片”或“粘贴图片”。搜索只匹配文字，不识别图片中的文字。目标应用必须支持图片粘贴；PasteWin 只发送 Ctrl+V，绝不额外发送 Enter 或提交消息。

## 安装包

双击 `bin\pastewin-0.2.0-amd64-setup.exe` 运行中文安装向导：

- 选择安装目录，默认 `%LOCALAPPDATA%\Programs\PasteWin`，仅为当前用户安装。
- 分别选择是否创建桌面、开始菜单快捷方式。
- 再次安装自动读取旧安装位置及快捷方式选择。同目录升级替换旧 EXE；更换目录后删除已登记旧目录中的 EXE 和卸载程序。
- 如果旧程序仍在运行，会提示从托盘退出后重试。不会强行结束进程。
- 可从 Windows“已安装的应用”或安装目录中的 `uninstall-pastewin.exe` 卸载。升级和卸载均保留 `%APPDATA%\PasteWin\history.db`，不递归删除安装目录内的其他文件。
- 同目录升级保留开机自启；更换安装目录时更新指向旧安装的启动项，卸载时清理指向该安装的启动项，不改动其他便携版副本的登记。
- 缺少 WebView2 时会运行微软引导安装程序，需要网络连接。

此前直接拷贝到其他目录的便携版 EXE 没有安装登记，安装程序不会扫描磁盘删除它；选择其所在目录安装可以替换该 EXE。

构建安装包需要 [NSIS 3](https://nsis.sourceforge.io/Download)。安装 NSIS 并加入 PATH，或指定编译器路径：

```powershell
wails3 task package
# 自定义 NSIS 路径
powershell -NoProfile -File build/windows/package.ps1 -NsisPath 'C:\Tools\NSIS\makensis.exe'
# ARM64（需在相应设备上验证运行）
wails3 task package ARCH=arm64
# 安装、升级、锁定文件、快捷方式和卸载回归：使用隔离目录及随机注册表标识
powershell -NoProfile -File build/windows/test-installer.ps1 -NsisPath 'C:\Tools\NSIS\makensis.exe'
```

打包会先在 `.work/package-<arch>` 生产构建，再输出 `bin/pastewin-<version>-<arch>-setup.exe`，不覆盖运行中的 `bin/pastewin.exe`。版本号取自 `build/windows/info.json`；发布时同时更新 `build/config.yml` 及该文件中的版本信息。安装逻辑位于 `build/windows/nsis/project.nsi`，不要用模板生成命令覆盖定制内容。

静默安装支持 `/S /DESKTOP=0 /STARTMENU=1 /D=C:\目标目录`；快捷方式开关取值为 `0` 或 `1`。`/D=` 必须是最后一个参数，路径不要加引号，即使包含空格。省略开关时保留上次选择，首次安装默认勾选。静默安装遇到文件占用会返回非零退出码。

## 界面

默认使用 360 × 680 的紧凑浮动面板（最小 320 × 420）。顶部搜索与全部 / 收藏分类，单列卡片展示历史；底部提供查看全文、收藏、删除、复制、粘贴和暂停。拖动左上角图标可移动窗口，双击记录直接粘贴。全文和设置均在弹窗中显示。设置页的“关于 PasteWin”展示应用用途和当前版本号，版本与安装包共用 `build/windows/info.json` 中的配置。

## 已实现

- 通过 `AddClipboardFormatListener` 接收 Windows 剪贴板变化通知，空闲时不轮询。
- Unicode 纯文本捕获，短暂占用时重试；保留原始换行、空格，重复文本合并。
- 静态 PNG 与未压缩 DIB/DIBV5 图片捕获，保留透明通道，按规范化像素去重；回写 PNG 和 sRGB DIBV5。
- SQLite 持久化历史、收藏和暂停状态，列表按 50 条分页，选中后再读取全文。
- 最多 500 条、单条最多 1 MB；容量不足时清理最旧的未收藏记录。若全部 500 条都已收藏，新内容不再保留。
- 图片最多 1600 万像素，原始格式缓冲合计最多 64 MiB，编码后每张 PNG 最多 20 MiB；图片和缩略图有效数据合计最多 256 MiB。超限时优先清理最旧未收藏图片记录，无法腾出空间则拒绝新增并提示。
- 搜索、收藏、删除、清除未收藏记录、暂停与恢复。
- 全局快捷键、托盘、单实例、无边框可调整窗口。
- 设置页开机自启开关，登录 Windows 后在托盘后台运行。
- 记住唤起前的窗口，恢复焦点后用 `SendInput` 发送 Ctrl+V；失败会显示说明。
- 尊重 `ExcludeClipboardContentFromMonitorProcessing` 和 `CanIncludeInClipboardHistory=0` 标记。

## 本地开发

环境：Windows 10/11、Go 1.27.0+、Node.js 22.12+（或兼容的较新 LTS）、WebView2。

```powershell
cd C:\Users\zhang\Developer\projects\pastewin
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.25
wails3 doctor
wails3 dev
```

如果找不到 `wails3`，将 `$(go env GOPATH)\bin` 加入 PATH，或者执行 `& "$(go env GOPATH)\bin\wails3.exe" dev`。当前机器已安装 Wails CLI。

生产构建：

```powershell
wails3 build
go test ./...
```

从全新检出运行测试时，先构建一次前端（完整 `wails3 build` 会自动完成），因为 Go 会嵌入 `frontend/dist`。

仅预览界面：

```powershell
cd frontend
npm ci
npm run dev
```

浏览器访问 `http://127.0.0.1:9245/?preview=1`。此模式使用明确标识的内存示例数据，不读写系统剪贴板；生产构建不包含示例数据。

## 目录

- `main.go`：窗口、托盘、快捷键、单实例。
- `clipboard_service.go`：前后端服务、捕获与粘贴协调。
- `internal/native/`：Windows 消息窗口、剪贴板格式和输入注入。
- `internal/history/`：SQLite 存储、搜索与容量策略，附行为测试。
- `internal/clipimage/`：有界 PNG/DIB 解析、图片去重和缩略图。
- `capture_pipeline.go`：图片后台处理与有界队列，避免占用剪贴板进行编码。
- `image_assets.go`：通过 Wails 应用内资源按记录 ID 读取图片，不开启独立 HTTP 服务。
- `internal/startup/`：当前用户 Run 启动项管理，附隔离注册表行为测试。
- `frontend/src/`：界面、样式和开发预览。
- `frontend/bindings/`：Wails 自动生成的 TypeScript 绑定，请勿手改。

默认数据路径为 `%APPDATA%\PasteWin\history.db`。可设置 `PASTEWIN_DATA_DIR` 改用独立数据目录进行测试。程序只记录启动之后的新复制事件，不主动采集启动前的剪贴板内容。

图片原图和缩略图以 BLOB 保存在同一数据库中，删除、清理和淘汰会同步删除对应图片数据。旧数据库自动迁移，保留文本、收藏及暂停状态。SQLite 会复用已删除空间，数据库和 WAL 文件的磁盘占用可能大于图片有效数据量，不会在每次删除后立刻缩小。

## 原型边界

- 支持纯文本与静态位图；复制图片文件（例如资源管理器 Ctrl+C）、动图、OCR、图片编辑、HTML/RTF 图文还原和自定义快捷键未实现。
- DIB 支持 1/4/8 位调色板及 16/24/32 位 RGB/bitfields；不支持压缩 DIB、链接/嵌入色彩配置等特殊位图，读取失败会尝试其他可用图片格式。PNG 规范化为 8 位 RGBA，不保留源文件元数据或专业色彩配置。
- 图片处理最多同时保留一个处理任务和一个待处理快照；复制过快导致积压时跳过较旧的待处理记录并提示，不保证保存每一次高速复制。暂停/恢复、清除未收藏记录和退出会丢弃尚未完成的旧任务。
- 数据库为本地明文，不上传数据，不自动识别所有密码或敏感内容；可手动暂停。程序尊重退出历史的标准标记，但取决于源应用是否设置。
- 自动粘贴受 Windows 焦点策略和权限级别约束。普通权限不能保证向管理员应用注入输入，特殊编辑器也可能不接受模拟 Ctrl+V。
- 隐藏窗口会保留 WebView，以便快速唤起；这不是最低内存方案。应统计主进程和所有关联 WebView2 进程，而非只看 Go 进程。
- Wails v3 仍为 Beta。本项目固定 Go 模块版本和 npm lockfile；升级框架后需重新验证托盘、热键、焦点和窗口生命周期。
- EXE 与安装包目前均未进行代码签名。

## 验证

`npm --prefix frontend test` 使用本机 Microsoft Edge 无头运行浏览器回归测试，在 360 × 680 和 320 × 420 下覆盖唤起重置、延迟入库与手动选择、旧请求隔离、删除相邻选择、分页刷新、弹窗焦点及复制/粘贴目标。测试启动独立的 9246 端口预览，使用合成数据，不读写系统剪贴板或真实历史；结果保存在 `.work/browser-results`。这不能代替原生热键和目标窗口粘贴验证。

自动测试覆盖：中文与空白保真、重复内容保持收藏、500 条容量、分页、字面搜索、清理保留收藏、超大文本拒绝、数据库重启持久化。

图片测试覆盖透明像素往返、PNG 编码差异去重、DIB 方向/行补齐/调色板/bitfields、损坏数据与尺寸边界、缩略图、旧库迁移、图片容量淘汰和级联删除、暂停与清理后的过期任务、图片资源路径限制。

原生图片集成测试默认跳过；显式运行时会用合成内容替换系统剪贴板，不访问真实历史库。两个包分开运行，避免同时争用剪贴板：

```powershell
$env:PASTEWIN_TEST_CLIPBOARD = '1'
go test . -run TestImageCaptureAndCopyIntegration -v
go test ./internal/native -run TestImageClipboardIntegration -v
Remove-Item Env:PASTEWIN_TEST_CLIPBOARD
go test ./internal/clipimage -run '^$' -fuzz FuzzDecodeDIB -fuzztime 15s
```

跨应用验收应包含截图、浏览器复制图片、透明 PNG、画图及聊天编辑区回贴，并确认没有额外发送 Enter；界面检查默认 360 × 680 和最小 320 × 420 下的缩略图、图文混合预览、长文本与键盘操作。服务集成测试不能代替这些桌面回归。

自启测试覆盖隔离注册表中的开关读写、重复操作、中文及空格路径、移动路径后重新登记、超长命令拒绝与无关值保留；安装回归覆盖启动项迁移、卸载清理和便携副本保护。实际注销/登录仍需在 Windows 会话中验证。

后续手动回归清单：复制中文和多行文本 → 快捷键唤起 → 搜索 → 收藏 → 暂停/恢复 → 粘贴回原应用 → 隐藏后重新唤起 → 退出重启。

2026-09-25 实测通过：生产构建、Go 测试和 go vet 静态检查；普通桌面 EXE 捕获记事本中文文本，Ctrl+Shift+V 唤起，搜索筛选、收藏，再按 Enter 粘贴回记事本。早期桌面验证使用独立测试数据库；最终生产版本另完成多行中文捕获与回贴验证。
