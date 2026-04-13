# zhihu-tui Chrome 扩展

## 构建

```bash
npm install
npm run build
```

## 安装（开发者模式）

1. 打开 Chrome 扩展管理页：`chrome://extensions`
2. 开启「开发者模式」
3. 选择「加载已解压的扩展程序」
4. 选择本目录（`zhihu-tui/extension`）

> 构建后会生成 `dist/background.js`，`manifest.json` 已指向该文件。
