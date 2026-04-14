# zhihu-tui

一个基于 Go + Bubble Tea 的知乎终端阅读器（TUI）。

项目支持两种运行模式：
- **真实数据模式**：通过本地 daemon + Chrome 扩展复用知乎登录态抓取数据
- **Mock 模式**：不连接浏览器，使用内置假数据调试界面

## 功能说明

- **热榜**：首页展示知乎热榜条目，支持列表内过滤（`/`，`Esc` 清除）、分页（`n` / `p`）、刷新（`r`）。按 `R` 进入推荐页，按 `f` 进入搜索页。
- **推荐**：个性化推荐流，可刷新（`r`，会清缓存后重拉）。选中问题、回答或文章后进入对应详情或预览。
- **搜索**：关键词搜索知乎内容，支持输入框与结果列表焦点切换（`Tab`）、结果分页。若粘贴的是问题/回答链接，可直接进入对应页面。
- **问题与回答**：查看问题下回答列表（分页），进入回答后可在「正文 / 评论」间切换（`Tab`），浏览评论分页；正文与评论区域滚动衔接（见内置帮助）。
- **文章**：支持专栏文章详情浏览。
- **阅读模式**：对回答正文等提供专注阅读视图；可复制全文（`yy`）。
- **与外部工具联动**：`o` 在系统默认浏览器打开当前条目上下文；`e` 用 `$EDITOR` 打开正文（各页行为见帮助）；`yy` / 连按两次 `y` 复制到剪贴板（具体复制范围因页面而异，详见 `?` 帮助页）。

---

## 环境要求

- Go `1.25`（见 `go.mod`）
- （真实数据模式）可用的浏览器自动化 daemon（默认命令：`zhihu-tui-daemon`）
- （真实数据模式）已安装并启用 Browser Bridge 扩展，且浏览器中已登录 `zhihu.com`

---

## 快速开始

### 1) 安装依赖

```bash
go mod download
```

### 2) 运行（推荐先用 Mock 验证）

```bash
go run . -mock
```

或：

```bash
ZHIHU_TUI_MOCK=1 go run .
```

---

## 真实数据模式使用方式

### 先构建并启动 `zhihu-tui-daemon`

在仓库根目录执行：

```bash
go build -o zhihu-tui-daemon ./cmd/zhihu-tui-daemon
./zhihu-tui-daemon
```

如果希望全局可执行，也可以：

```bash
go install ./cmd/zhihu-tui-daemon
```

> daemon 默认监听 `127.0.0.1:19860`，可通过 `ZHIHU_TUI_DAEMON_PORT` 修改端口。

### 再启动 TUI

直接运行：

```bash
go run .
```

程序会自动：
1. 检查本地 daemon（默认地址 `http://127.0.0.1:19860`）
2. 若未启动则尝试拉起 `zhihu-tui-daemon`
3. 检查 Browser Bridge 扩展连接状态

如果报错「扩展未连接」或 `HTTP 401/403`，通常是以下原因：
- 浏览器未打开或扩展未启用
- 扩展连接异常
- 当前自动化窗口未登录知乎

---

## 可用环境变量

- `ZHIHU_TUI_MOCK=1`  
  启用 Mock 模式（等价于 `-mock`）

- `ZHIHU_TUI_DAEMON_PORT=19860`  
  自定义 daemon 端口

- `ZHIHU_TUI_DAEMON_BIN=xxx`  
  指定 daemon 可执行文件名/路径（默认 `zhihu-tui-daemon`）

- `ZHIHU_TUI_WORKSPACE=site:zhihu`  
  指定自动化 workspace（默认 `site:zhihu`）

---

## 快捷键（核心）

- `q` / `Ctrl+C`：退出
- `?`：打开/关闭帮助
- `h` / `Esc` / `←`：返回上一页
- `Enter` / `l` / `→`：进入详情页
- `Tab`：回答页切换「正文/评论」焦点
- `n` / `p`：问题回答分页、评论分页
- `r`：热榜页刷新

> 列表和视口滚动键位主要遵循 Bubbles 默认行为（如 `j/k`、`pgup/pgdown` 等）。

---

## 扩展构建（可选）

如果你需要手动构建仓库内的最小扩展实现（目录 `extension/`）：

```bash
cd extension
npm install
npm run build
```

然后在 Chrome 扩展管理页中以开发者模式加载 `extension/` 目录。
