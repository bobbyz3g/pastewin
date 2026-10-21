# PasteWin Glass Design

Apple 风格的玻璃材质设计系统，为 PasteWin 带来现代、精致的视觉体验。

## 设计理念

**材质驱动的层次感**  
通过不同透明度和模糊强度的玻璃层建立空间关系，而非依赖厚重的边框和阴影。

**克制的视觉语言**  
避免过度装饰，让每个元素都有明确的功能目的。

## 核心设计决策

### 颜色系统

```css
--accent-blue: #007AFF           /* iOS 标准蓝 */
--text-primary: #1d1d1f          /* 深灰文本 */
--text-secondary: #86868b        /* 次要信息 */
--text-tertiary: #c7c7cc         /* 弱化文本 */

--glass-bg: rgba(245,245,250,0.72)      /* 主背景 - 72% 透明 */
--glass-card: rgba(255,255,255,0.65)    /* 卡片层 - 65% 透明 */
--glass-selected: rgba(0,122,255,0.12)  /* 选中态 - 12% 蓝色 */
--glass-toolbar: rgba(249,249,251,0.88) /* 工具栏 - 88% 透明 */
```

### 玻璃效果实现

```css
backdrop-filter: blur(40px) saturate(180%);
-webkit-backdrop-filter: blur(40px) saturate(180%);
```

- **40px 模糊**：足够的模糊半径让背景内容柔和化，但仍可辨识
- **180% 饱和度**：增强背景色彩，让玻璃效果更生动
- **分层透明度**：主窗口 72%，卡片 65%，工具栏 88%

### 圆角系统

```css
--radius-sm: 6px    /* 小元素：按钮、输入框 */
--radius-md: 10px   /* 中等：菜单、卡片 */
--radius-lg: 12px   /* 大元素：dialog、主窗口 */
--radius-xl: 16px   /* 窗口外框 */
```

接近 iOS 的圆角比例，营造柔和、亲和的视觉感受。

### 阴影系统

```css
--shadow-sm: 0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.02)
--shadow-md: 0 4px 12px rgba(0,0,0,0.08), 0 2px 4px rgba(0,0,0,0.04)
--shadow-lg: 0 12px 32px rgba(0,0,0,0.12), 0 4px 8px rgba(0,0,0,0.06)
```

轻量的双层阴影，营造自然的深度感。

### 动画曲线

```css
transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
```

Apple 标准的 ease-out 曲线，响应迅速但不生硬。

## 关键交互状态

### 按钮
- **Hover**: 背景 `rgba(0,0,0,0.04)`
- **Active**: `scale(0.96)` 缩放反馈
- **Disabled**: `opacity: 0.4`

### 卡片
- **默认**: 65% 白色 + 10px 模糊
- **Hover**: 80% 白色 + 上浮 1px
- **Selected**: 12% 蓝色背景 + 2px 蓝色光环

### 搜索框
- **默认**: 50% 白色 + 轻边框
- **Focus**: 90% 白色 + 蓝色边框 + 3px 扩散光晕

## 独特性分析

**避免的 AI 设计模板特征：**
- ❌ 奶油色背景 (#F4F1EA) + 陶土色强调 (#D97757)
- ❌ 纯黑背景 + 荧光绿/红色强调
- ❌ 报纸式布局 + 发际线边框
- ❌ 全大写标签、中点分隔符 (A · B · C)
- ❌ 统一圆角的 SaaS 卡片堆叠

**我们的特点：**
- ✅ 材质透明度驱动的层次
- ✅ iOS 色彩系统（#007AFF）
- ✅ 功能化的视觉结构
- ✅ 克制的动效（只在交互反馈时出现）

## 文件结构

```
frontend/src/
├── style.css           # 当前应用的玻璃设计
├── style-glass.css     # 玻璃设计源文件（备份）
└── style-original.css  # 原始设计（可恢复）
```

## 切换设计

恢复原设计：
```bash
cp frontend/src/style-original.css frontend/src/style.css
```

应用玻璃设计：
```bash
cp frontend/src/style-glass.css frontend/src/style.css
```

## 调整建议

如需微调，重点参数：

- **透明度**: 调整 `--glass-bg`、`--glass-card` 的 alpha 值
- **模糊强度**: 修改 `backdrop-filter: blur(40px)` 的数值
- **强调色**: 替换 `--accent-blue` 和相关使用位置
- **圆角**: 调整 `--radius-*` 变量

## 浏览器兼容性

- ✅ Chromium/Edge (WebView2)
- ✅ Safari
- ⚠️ Firefox 需开启 `layout.css.backdrop-filter.enabled`

PasteWin 使用 WebView2，完全支持 backdrop-filter。
