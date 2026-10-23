# 发布流程

[English README](../README.md) · [中文介绍](../README.zh-CN.md) · [开发与验证](development.md) · [Windows 安装包](packaging.md)

本文说明版本更新、Git tag、推送及 GitHub Release 的完整流程。发布说明同时提供中文和英文。

## 版本号规则

版本号使用 `主版本.次版本.补丁版本` 三段数字。**没有特殊说明时，只将最后一位加 1，前两位保持不变。**例如 `0.3.5 → 0.3.6`、`0.3.9 → 0.3.10`。只有明确指定版本号或要求升级主版本、次版本时，才采用其他版本；不要因新增功能自行升级次版本。

以当前仓库版本为基础，并核对远端 tag 和已有 Release，避免重复使用版本号。tag 沿用三段数字，不加 `v` 前缀，例如 `0.3.6`；Release 标题为 `PasteWin 0.3.6`。

同步更新以下位置，确保版本完全一致：

| 文件 | 字段 |
| --- | --- |
| [build/windows/info.json](../build/windows/info.json) | `fixed.file_version`、`info.0000.ProductVersion` |
| [build/config.yml](../build/config.yml) | `info.version` |
| [frontend/package.json](../frontend/package.json) | `version` |

前端“关于”和启动日志读取 `build/windows/info.json`，不要另外添加版本常量。安装包名称由构建脚本生成，不手动改名伪装新版本。仅修改项目版本不需要变更依赖版本或依赖锁文件。

## 1. 检查发布范围

从仓库根目录执行：

```powershell
git status --short
git fetch origin --tags
git log --oneline --left-right HEAD...origin/main
git tag --sort=-version:refname
```

确认工作分支、待发布提交和已有 Release。保留无关的未提交修改，逐项暂存本次需要的文件，不把真实数据库、日志、凭据或构建产物提交到 Git。远端存在新提交时先核对并整合，不能用强制推送覆盖。

## 2. 更新版本并验证

按上述规则修改三个版本文件。按变更范围完成 [开发验证](development.md)，准确记录通过的检查和未验证的边界。典型检查包括：

```powershell
go test ./...
go vet ./...
pnpm --dir frontend test
```

Go 嵌入 `frontend/dist`，全新检出时先构建前端或完成生产构建，再执行 Go 测试。前端测试使用独立的 9246 端口和合成数据；不为发布验证替换用户的真实剪贴板或修改真实历史。

生成 Windows x64 生产安装包：

```powershell
powershell -NoProfile -File build/windows/package.ps1 -Arch amd64
```

脚本在 `.work/package-amd64` 构建，不覆盖正在运行的 `bin/pastewin.exe`。NSIS 路径和安装器回归见 [安装包文档](packaging.md)。修改安装逻辑时执行对应安装器测试。任何检查失败都先修复，不创建成功发布记录。

确认产物为 `bin/pastewin-<版本>-amd64-setup.exe`，且本次确实重新构建。生成校验文件：

```powershell
$releaseVersion = (Get-Content build/windows/info.json -Raw | ConvertFrom-Json).fixed.file_version
$installerName = "pastewin-$releaseVersion-amd64-setup.exe"
$installerPath = Join-Path 'bin' $installerName
$installerHash = (Get-FileHash -LiteralPath $installerPath -Algorithm SHA256).Hash.ToLowerInvariant()
New-Item -ItemType Directory -Force .work | Out-Null
[IO.File]::WriteAllText((Join-Path $PWD '.work/SHA256SUMS.txt'), "$installerHash  $installerName`n")
```

## 3. 提交、打 tag、推送

将功能改动和版本改动组织成清晰的提交。版本提交示例：

```powershell
git add build/windows/info.json build/config.yml frontend/package.json
git diff --cached --check
git diff --cached
git commit -m "chore: bump version to $releaseVersion"
```

确认所有发布源码已提交，构建内容与目标提交一致后，创建注解 tag 并推送。以下命令以当前发布分支为 `main` 为前提：

```powershell
git tag -a $releaseVersion -m "PasteWin $releaseVersion"
git show --no-patch $releaseVersion
git push --atomic origin main "refs/tags/$releaseVersion"
```

逐条检查命令是否成功。tag 已存在时核对它指向的提交，不覆盖或移动已发布 tag；普通推送被拒绝时先处理分支差异，不强制推送。推送失败时不要继续发布 Release。

## 4. 创建 GitHub Release

在 [项目 Releases](https://github.com/bobbyz3g/pastewin/releases) 中创建草稿，选择刚推送的 tag，标题为 `PasteWin <版本>`。也可以使用已配置的 GitHub CLI 或 API 完成相同步骤；凭据不得写入仓库、发布说明或日志。

1. 先写中文更新日志，再写英文更新日志，内容和已知限制保持一致。两段之间用 Markdown 分割线 `---` 分隔，不添加“中文”“英文”或“English”等语言标题。
2. 说明本次相对上一版本的用户可见变化、升级注意事项、实际验证结果，以及未验证的边界。不要把计划功能写成已完成。
3. 添加从上一 tag 到本次 tag 的完整变更比较链接。
4. 上传 `pastewin-<版本>-amd64-setup.exe` 和 `SHA256SUMS.txt`。
5. 核对 tag 对应的提交、附件名称和大小。将上传资产的 SHA-256 摘要与本地比对；无法取得远端摘要时下载附件计算校验值。
6. 确认附件完整后发布正式 Release，并设为最新稳定版本；只有明确要求时才标记为预发布。

发布说明可按以下结构组织，填入实际内容，不保留占位符：

```markdown
### 更新内容
- 本次用户可见变化。

### 下载与说明
适用平台、安装包名称、校验方式、升级注意事项。
实际通过的检查和未验证的边界。

---

### What's changed
- User-visible changes in this release.

### Download and notes
Supported platform, installer name, checksum verification, and upgrade notes.
Checks completed and limitations not verified.

**Full changelog:** https://github.com/bobbyz3g/pastewin/compare/<previous-tag>...<new-tag>
```

目前安装包未配置代码签名，不应声称已签名。上传中断时检查现有草稿和附件后继续，不重复创建 Release；发现同 tag 已正式发布时先核对，不静默覆盖已有发布。

## 5. 发布后确认

检查 Release 已公开、版本和 tag 正确、中英文说明完整、两个附件均可下载且校验一致。交付时提供版本号、版本提交哈希、Release 链接，以及推送和附件验证结果。临时说明、校验文件和构建产物可以保留在 `.work/`、`bin/`，不纳入源码提交。
