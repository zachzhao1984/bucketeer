# Bucketeer 桌面应用 UI 重设计 · 设计 spec

## 项目是什么
Bucketeer——macOS 桌面应用,管理 S3 兼容对象存储(Cloudflare R2、AWS S3、Backblaze B2、Wasabi、MinIO、阿里 OSS、腾讯 COS)。当前版本为 macOS 原生风格(浅/深色、蓝色强调、Phosphor 图标),交互模型:左侧边栏(导航 + 底部连接卡片)、顶部工具栏(路径、过滤、排序、视图切换)、文件网格/列表、悬浮传输 dock。

## 受众与场景
独立开发者与创作者,在 Mac 上管理自己的媒体桶:上传图片、拿直链、看用量。键盘熟练,对开发者工具的审美有判断力。

## 核心信息
1. 「我的桶里的东西一目了然」——文件、类型、大小、时间
2. 「传输正在进行,不失控」——上传/下载队列与进度
3. 「这是个趁手的工具,不是个网站」

## 情感基调
趁手、扎实、可信。要有工具的"物件感",不要营销页的花哨。

## 输出格式(三版统一)
- 单文件 HTML,桌面 App 窗口壳内 **1180×820** 的应用主屏:文件浏览(网格模式)+ 悬浮传输 dock
- 每版同一套真实内容,只换设计

## 已知约束
- 可读性底线:正文 ≥14px、标签 ≥12px、对比度 ≥4.5:1
- 文件类型图标需要一眼可辨(图片/视频/音频/压缩包/文档)
- 不改变交互模型(侧栏 + 工具栏 + 网格 + 传输 dock)

## 共用内容(三版一致,真实不占位)
- 侧栏导航:Dashboard / All files / Recent files / Settings;底部连接卡片:「R2 主力 · media · s3.example.com」
- 面包屑:media / photos / 2026-10
- 工具栏:过滤框、类型过滤(All)、排序(Date)、Grid/List 切换
- 文件(12 个,图片为主,混真实类型):IMG_2417.jpg 3.4 MB、sunrise-timelapse.mp4 482 MB、album-cover.png 1.1 MB、field-notes.pdf 82 KB、backup-2026-10.zip 1.9 GB、podcast-ep-42.m4a 36 MB、IMG_2418.jpg 2.9 MB、logo-dark.svg 14 KB、contract-v2.docx 210 KB、IMG_2419.jpg 3.1 MB、waveform.png 640 KB、readme.md 4 KB
- 汇总:12 items · 2.4 GB · 上次同步 2 分钟前
- 传输 dock:2 项——IMG_2420.jpg ↑ 78%、sunset-timelapse.mp4 ↑ 31%
- 缩略图:真实图片(picsum 固定 seed),程序化渐变仅用于非图片类型
