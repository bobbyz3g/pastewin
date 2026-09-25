# PasteWin 开发上下文

本文件适用于整个仓库。先遵循用户当前请求及更具体的目录指令；以下内容记录现有实现和默认约定，并不限制用户明确要求的功能扩展。产品行为、架构或开发命令变化后，同步更新本文件和 README。

## 产品与当前范围

- Windows 本地剪贴板工具，当前是可运行的原型，目标是常驻后台、快捷唤起、快速选择并粘贴历史。
- **目前仅支持纯文本**。图片、文件、HTML/RTF、自定义快捷键、开机启动尚未实现；不要将计划能力描述为已支持。
- 已实现：文本捕获、精确去重、搜索、收藏、删除、清除未收藏记录、暂停/恢复、SQLite 持久化、托盘、单实例，以及支持目录选择、可选快捷方式和升级替换的 NSIS 安装包。
- “粘贴到原应用”指将内容插入唤起面板前的窗口：恢复该窗口焦点并模拟 Ctrl+V；只执行粘贴，绝不额外发送 Enter 或提交消息。原窗口有选区时由目标应用处理替换。

## 技术栈与入口

- Go + Wails **v3.0.0-beta.25**，React 18 + TypeScript + Vite，图标使用 lucide-react。
- SQLite 使用 `modernc.org/sqlite`（纯 Go）。以 `go.mod` 和 `frontend/package-lock.json` 为实际依赖依据。
- 当前构建面向 Windows；`build/` 中保留的其他平台模板不表示已经支持这些平台。

| 路径 | 职责 |
| --- | --- |
| `main.go` | 窗口、托盘、快捷键、单实例、应用生命周期 |
| `clipboard_service.go` | Wails 服务、捕获协调、复制/粘贴、状态与事件 |
| `internal/native/clipboard_windows.go` | Win32 消息窗口、剪贴板监听及读写 |
| `internal/native/paste_windows.go` | 前台窗口识别、焦点恢复、输入注入 |
| `internal/history/` | SQLite 存储和行为测试 |
| `frontend/src/App.tsx`、`style.css` | 界面及样式 |
| `frontend/src/api.ts` | 原生服务适配、事件订阅、预览模式切换 |
| `frontend/src/preview.ts` | 仅开发环境使用的内存示例数据 |
| `frontend/bindings/` | 自动生成的 TypeScript 绑定，不手动编辑 |
| `Taskfile.yml`、`build/` | Wails 开发与构建配置 |

## 已确定的界面与交互

- 用户选择了类似 PasteNow 的窄幅浮动面板：浅灰紫背景、单列圆角卡片、蓝色选中边框。默认 **360 × 680**，最小 **320 × 420**；未经新要求不要改回宽幅双栏工作台。
- 顶部为搜索、设置、隐藏按钮和全部/收藏分类；底部为所选记录操作、记录状态及数量。全文与设置使用弹窗。
- `Ctrl+Shift+V` 唤起，方向键选择，`Enter` 粘贴，`Ctrl+Enter` 仅复制；双击记录也可粘贴。
- 普通列表中 `Esc` 先清空搜索，再隐藏；弹窗中先关闭弹窗。保留输入法组合输入保护，避免选字时触发快捷操作。
- 面板关闭/隐藏后继续在托盘运行；只有明确的“退出”才结束进程。拖动左上角图标可移动窗口。
- 没有有效目标窗口时禁用粘贴并提供提示，仍允许复制。
- 界面改动应检查默认及最小尺寸、长文本、空状态、键盘操作和弹窗；不要加入无实际功能的分类或按钮。

## 必须保留的实现行为

- 使用 `AddClipboardFormatListener` 事件监听，空闲时不轮询；只采集启动后的新复制事件。
- 保留文本中的原始空白和换行；仅完全相同的文本合并，合并时保留收藏状态。
- 当前容量为最多 500 条、每条最多 1 MiB UTF-8 文本；淘汰最旧的未收藏项，收藏不自动清理。全部 500 条都收藏时，新内容不会保留。
- 列表每页 50 条，仅返回最多 180 字符预览，选中后读取全文；避免每次刷新传输所有完整内容。
- 捕获、写入和暂停状态由服务协调；保留剪贴板序列号抑制自身写入，恢复记录时不补采暂停期间的内容。
- 尊重 `ExcludeClipboardContentFromMonitorProcessing` 与 `CanIncludeInClipboardHistory=0` 标记；这不等于能识别所有敏感内容。
- Win32 调用需保持 OS 线程约束、内存长度边界和句柄所有权；输入结构布局针对 Windows 64 位。剪贴板内存通过有边界的 `RtlMoveMemory` 复制，不随意将原生地址转换为 Go 指针。
- 自动粘贴需等待快捷键释放、检查目标与焦点、处理输入注入失败。管理员窗口及特殊编辑器可能不接受普通权限的输入注入，必须保留错误提示与手动复制退路。
- Wails runtime 与生成绑定保持静态导入。曾因同时拆成动态入口产生初始化循环，导致运行时 `Call` 未定义；不要未经验证恢复该加载方式。
- 新增弹窗时保留焦点管理及 Esc 行为；当前提示通过 portal 放进活动弹窗，避免被原生 dialog 顶层遮挡。

## 开发与验证

环境：Windows 10/11、Go 1.27.0+、Node.js 22.12+ 或兼容版本、WebView2。CLI 与项目 Wails 版本保持一致：

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.25
wails3 doctor
wails3 dev
```

找不到 CLI 时使用 `& "$(go env GOPATH)\bin\wails3.exe" dev`。运行开发实例或替换 EXE 前，从设置/托盘退出已有实例；单实例机制可能让新进程只唤起旧版本，运行中的 EXE 也可能无法覆盖。

```powershell
# 从仓库根目录执行
wails3 build                 # 完整生产构建，包含绑定生成和前端构建
wails3 task package          # NSIS 安装包，需要 NSIS 3；详见 README
go test ./...
go vet ./...
npm --prefix frontend run format
npm --prefix frontend run build  # 仅检查/构建前端
```

- 产物为 `bin/pastewin.exe`。Go 嵌入 `frontend/dist`；全新检出时先完成构建，再运行 Go 测试。
- 纯界面预览：在 `frontend` 运行 `npm ci`、`npm run dev`，访问 `http://127.0.0.1:9245/?preview=1`。预览不操作系统剪贴板，示例数据只限开发构建；浏览器验证不能代替原生粘贴验证。
- 按改动范围验证：文档改动核对内容即可；前端改动做编译和视觉/交互检查；存储、原生或服务改动做相应测试、静态检查及生产构建。
- 原生回归使用记事本等测试窗口：复制中文/多行 → 热键唤起 → 搜索/收藏 → 粘贴回光标位置；涉及生命周期时检查暂停/恢复、隐藏后唤起和退出重启。
- 为具体行为、边界和回归添加测试，不用仅复述实现的测试。已有存储测试覆盖去重、收藏保留、容量、分页、字面搜索、清理、大小限制及重启持久化。

## Windows 安装程序

- `build/windows/package.ps1` 在 `.work/package-<arch>` 生产构建并生成 `bin/pastewin-<version>-<arch>-setup.exe`。支持 `-NsisPath`、`-Arch`、`-OutputDir`，版本取自 `build/windows/info.json`。
- `build/windows/nsis/project.nsi` 是项目定制脚本，不依赖旧的 `wails_tools.nsh` 模板；不要用 Wails 模板生成命令覆盖。仅当前用户安装，HKCU 卸载登记标识为 `dev.pastewin.app`。
- 默认目录为 `%LOCALAPPDATA%\Programs\PasteWin`；中文向导可修改目录，独立选择桌面/开始菜单快捷方式。升级沿用旧目录和选择，用户仍可更改。
- 替换前检查 EXE 写权限，运行中提示退出后重试；静默安装直接返回非零。复制失败尝试恢复旧 EXE；目录迁移只删除登记位置的程序及卸载文件。禁止递归删除安装目录或用户数据。
- 升级和卸载保留剪贴板数据库及 WebView 配置；未登记的其他便携版副本不会扫描删除。首次安装可选择便携版所在目录来替换其 EXE。
- `build/windows/test-installer.ps1 -NsisPath <makensis.exe>` 使用随机应用标识、独立目录和虚拟 EXE 测试安装/同目录升级/目录迁移/文件占用/快捷方式/卸载，不能操作真实剪贴板库。
- 安装变更须通过上述行为测试、安装包构建，并检查目录选择和快捷方式向导页面。已签名发布流程尚未配置，不声称安装包已签名。

## 数据与工作边界

- 默认数据库为 `%APPDATA%\PasteWin\history.db`，本地明文存储，无上传功能。测试优先设置 `PASTEWIN_DATA_DIR` 指向 `.work/` 下独立目录；该变量只隔离数据，不隔离单实例标识或全局快捷键。
- 不为排查问题输出真实剪贴板全文、不修改或清理用户现有历史；使用自建测试内容。正常交付构建不启用可调用剪贴板服务的 HTTP 调试接口。
- `bin/`、`frontend/dist/`、`frontend/node_modules/`、`.task/`、`.work/`、SQLite 数据文件和 `*.syso` 已忽略，不纳入源码提交。保留依赖锁文件。
- 尊重仓库现有未提交修改。未获得用户明确指令，不提交、推送、创建远程仓库、发布或部署。
- 完成后说明实际改动、验证和未实现的边界；不要把历史检查结果当成本次验证结果。
