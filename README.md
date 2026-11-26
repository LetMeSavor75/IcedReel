# IcedReel

视频库整理工具（Go）。当前实现 **逻辑目录命名**、**单文件技术信息探测**、**NFO 生成** 和 **单条展示**；`rank` 后续加。

## 分层

- **目录名 = 逻辑层**：描述“这是哪部作品”，由 TMDB / Wikidata 和 IMDb/TMDB/豆瓣 ID 确定。
- **文件名 = 技术层**：实际时长、内置音轨、内置字幕、编码、片源等，由后续 `scrape` 探测，不进入目录名。
- 一个逻辑目录可以放多个版本文件，目录名不绑定某一个文件。

示例：

```
目录：肖申克的救赎_The Shawshank Redemption_(1994)_imdb-tt0111161_tmdb-278_douban-1292052
文件：www.example.com@The.Shawshank.Redemption.1994.UHD.BluRay.REMUX.2160p.HEVC.DTS-HD.MA.5.1.mkv
```

## 用法

```powershell
icedreel name [选项] [tt...]
```

| 选项              | 说明                                         |
| --------------- | ------------------------------------------ |
| `--imdb <id>`   | IMDb id 或 URL                              |
| `--douban <id>` | 豆瓣 id；Wikidata 缺少映射时手动补                    |
| `-y, --yes`     | 不交互，只输出目录名                                 |
| `--fallback`    | 非交互模式下 TMDB 失败时用 Wikidata 兜底；交互模式会自动进入手动补充 |
| `-h, --help`    | 帮助                                         |

示例：

```powershell
icedreel name tt0111161 --douban 1292052 -y
icedreel name tt0111161
icedreel name --douban 1292052 -y
```

交互模式（默认，TTY 下）：

```text
先填 IMDb id（可跳过，改用豆瓣 id）。
  IMDb id (tt...):
  豆瓣 id:                     # 仅当 IMDb 为空时询问

逐项确认（回车接受反查值，输入新值覆盖，输入 - 清空）：
  中文名 [肖申克的救赎]:
  English [The Shawshank Redemption]:
  年份 [1994]:
  imdb-id [tt0111161]:
  tmdb-id [278]:
  douban-id [1292052]:

确认生成？[Y/n]:
目录名: 肖申克的救赎_The Shawshank Redemption_(1994)_imdb-tt0111161_tmdb-278_douban-1292052
```

- 有 IMDb 就立刻查 TMDB，不会先拦着问豆瓣。
- 只有没有 IMDb 时，才问豆瓣 id。
- TMDB 失败时，才提示补豆瓣 id 走 Wikidata；不补也能进入手动字段。
- 已通过 `--imdb` / `--douban` 提供的 ID 不会重复问。
- `-y`：不交互，只输出目录名，适合脚本。
- 非 TTY（管道/CI）自动进入非交互模式。

输出格式：

```
中文_English_(年份)_imdb-tt..._tmdb-..._douban-...
```

缺失的段整段省略。ID 顺序固定为 IMDb、TMDB、豆瓣。

## scrape

```powershell
icedreel scrape [选项] <视频文件>
```

探测单个视频的技术信息，按 IMDb / 豆瓣 id 查 TMDB，写同名 NFO；**不修改视频文件名**。电影和剧集一视同仁，统一写 movie NFO；不生成 `tvshow` / `season` / `episodedetails` NFO。

| 选项                 | 说明                                     |
| ------------------ | -------------------------------------- |
| `--imdb <id>`      | IMDb id 或 URL                          |
| `--douban <id>`    | 豆瓣电影 id；Wikidata 缺少映射时手动补 `--imdb`      |
| `--ffprobe <path>` | ffprobe.exe 路径                         |
| `-h, --help`       | 帮助                                     |

示例：

```powershell
icedreel scrape "D:\Movies\...\www.example.com@The.Shawshank.Redemption.1994.UHD.BluRay.REMUX.2160p.HEVC.DTS-HD.MA.5.1.mkv" --imdb tt0111161 --douban 1292052
```

输出：

```text
文件:   www.example.com@The.Shawshank.Redemption.1994.UHD.BluRay.REMUX.2160p.HEVC.DTS-HD.MA.5.1.nfo
逻辑名: 肖申克的救赎_The Shawshank Redemption_(1994)_imdb-tt0111161_tmdb-278_douban-1292052
标题:   肖申克的救赎
原名:   The Shawshank Redemption
年份:   1994
链接:
  imdb https://www.imdb.com/title/tt0111161/
  tmdb https://www.themoviedb.org/movie/278
  douban https://movie.douban.com/subject/1292052/
  wikidata https://www.wikidata.org/wiki/Q172241
评分:   8.7 (27000)
类型:   剧情
制片:   Castle Rock Entertainment
国家:   United States
简介:   ...
视频:   3840x2160 HEVC HDR10 2:22:00 (142 min)
音轨:
  English, 英语 (Audio 1) [Default] (DTS-HD MA, 48.0 kHz, 5.1 chn, 24 bit)
  English, 英语 (Audio 2) (DD, 48.0 kHz, 2.0 chn, 16 bit)
字幕:
  Chinese, 简英双语
  Chinese, 简体中文
```

行为：

- ID 查询规则与 `name` 一致：只用 `--imdb` / `--douban`；TTY 下缺 id 会先问 IMDb、再问豆瓣，没反推到豆瓣 id 时会补问一次；非交互模式必须显式提供。
- 有 IMDb 就直接查 TMDB；只有豆瓣 id 时，先通过 Wikidata 反查 IMDb。
- TMDB `find` 同时返回电影和剧集时，按完整度选条目（票数权重最高，其次热度、海报、简介），不盲目电影优先。`tt8014982` 这类空壳重复条目会被正主压下去。
- Wikidata 缺少豆瓣→IMDb 映射时，直接报错并提示补 `--imdb`，不再写精简 NFO。
- TMDB 查询失败时，TTY 下询问是否重试；非交互直接报错。不会回退 Wikidata 覆盖现有 NFO。
- 文件名不参与查询；只用于确定默认 NFO 路径。
- 只探测容器内置音轨和字幕，不找外挂字幕。
- 音轨行格式：`English, 英语 (Audio 1) [Default] (DTS-HD MA, 48.0 kHz, 5.1 chn, 24 bit)`；标题为空时用语言中文名。`streamdetails/audio` 额外写 `samplerate`、`bitdepth`、`default`；旧 NFO 需重跑 `scrape` 才有采样率/位深。
- 字幕行格式：`Chinese, 简体特字(六区)`；逐条显示，标题为空时用语言中文名，不按语言归并。
- TMDB 不提供豆瓣 id；NFO 的豆瓣 id 来自 `--douban` 或 Wikidata 映射。
- 复用 `show` 的 NFO 展示；TTY 下先显示 poster/fanart，再输出报告。
- NFO 额外写自定义 `<logicalname>`；解析到剧集时根元素仍是 `<movie>`，额外写 `<tmdbtype>tv</tmdbtype>`，`show` 的 TMDB 链接用 `/tv/...`。
- 有 Wikidata ID 时写 `<wikidataid>Q...</wikidataid>`，`show` 的链接多一行 `wikidata https://www.wikidata.org/wiki/Q...`。
- 现有 NFO 与新内容不同：在展示后打印 `NFO diff:`；交互终端询问 `重写 ...？[y/N]`，默认不重写；非交互不写；内容相同报「已一致」。
- 剧集 NFO 不再生成；`show` 仍可读取已有的 `tvshow.nfo`、`season.nfo` 和 `episodedetails` NFO。

## show

```powershell
icedreel show [选项] <视频或 .nfo>
```

显示 NFO 元数据；支持 `movie`、`episodedetails`、`tvshow`、`season` 四种根元素。传视频时读同名 `.nfo`，不查 TMDB、不跑 ffprobe。TTY 下并排显示 poster 和 fanart：支持 Sixel 时输出真实位图，否则用半块字符；远程图可能下载。

| 选项 | 说明 |
|---|---|
| `--no-poster` | 不显示 poster 和 fanart |
| `--poster-width <n>` | 海报宽度（终端字符数），默认 32，范围 4-120；并排或 Sixel 未指定时用 60 |
| `--image <mode>` | 海报模式：`auto`（默认）、`sixel`、`half`、`none` |
| `-h, --help` | 帮助 |

示例：

```powershell
icedreel show "D:\Movies\Movie.nfo"
icedreel show "D:\Movies\Movie.mkv"
icedreel show "D:\Movies\Movie.nfo" --no-poster
```

行为：

- NFO 路径：文本只读本地 XML；输出标题、评分、类型、制片、国家、简介、链接和 `streamdetails`；不再单独显示 `识别:` 行，ID 只放在链接里。NFO 的 `<rating>` 不记录来源，因此原样显示 `评分:`，不猜测网站。视频行时长格式为 `2:22:00 (142 min)`；此时不再单独显示 `<runtime>` 的分钟行，只有没有视频秒数时才用它兜底。`<premiered>`、`<runtime>` 等标准字段仍写入 NFO，只是 `show` 不显示上映日期。字幕按轨道逐条显示 `English, 原标题`；标题为空时用语言中文名。poster/fanart 仍可能按 NFO 里的 URL 下载。
- 视频路径：只找同名 `.nfo`，存在就读；找不到报错并提示先跑 `scrape`。不查 TMDB、不跑 ffprobe；poster/fanart 仍可能按 NFO 的 URL 下载。
- 剧集 NFO 查找会扫描同目录和上一层，支持 `tvshow.nfo` 和 `<逻辑名>.nfo`（只认 `<tvshow>` 根元素）。
- 输出 IMDb / TMDB / 豆瓣 URL；TTY 下用 OSC 8 超链接，Windows Terminal 里 Ctrl+Click 打开；管道输出时是纯文本 URL，不带转义码。
- Windows 双击 `.nfo` 可能打开“系统信息”；用 `show` 查看可以避开扩展名关联。

海报 / fanart：

- NFO 同时有 poster 和 fanart 时并排显示：poster 左、fanart 右；只有其中一个就显示一个。未指定 `--poster-width` 时并排宽度用 60。
- poster 优先同目录 `poster.jpg`、`poster.png`、`folder.jpg`、`folder.png`；没有才用 NFO 的 `<thumb>`。
- 下载缓存到 `%LocalAppData%\IcedReel\images\<sha1>.img`，同一 URL 不重复下载。
- 默认 `auto`：终端支持 Sixel 时输出真实位图（Windows Terminal 1.22+），否则退回半块字符；Sixel 用 TMDB `w780`，半块用 `w500`。
- `--image sixel` 强制位图；`--image half` 强制半块字符；`--image none` 等同 `--no-poster`。
- 半块字符用 `▀` 加 24-bit 前景/背景色，每个字符单元格显示上下两个像素。
- 用 `CSI 16 t` 查询实际字符格尺寸；Windows Terminal 固定返回 `10×20` 虚拟格，不能反映自定义字体。程序会改读 Windows Terminal `settings.json` 的字体和字号，用 GDI 实测字符格后再校正 Sixel / 半块字符的宽高比。可用 `ICEDREEL_CELL_SIZE=8x21` 强制指定。
- 跳过海报和 fanart：`--no-poster`、`--image none`、设置了 `NO_COLOR`、stdout 不是 TTY。
- 不支持 Kitty 图形协议；海报失败只提示，不影响文字输出。

## 元数据

- TMDB 优先：IMDb → `find` → movie/tv → 中英标题、年份、TMDB ID。
- 豆瓣 id：优先 `--douban`；否则 Wikidata `P345 ↔ P4529` 反查。
- `name` 在 TMDB 未配置或 `--fallback` 时可用 Wikidata 兜底；`scrape` 不兜底，必须拿到 TMDB 元数据才写 NFO。
  - 中文/英文标签
  - 最早 P577 年份
  - TMDB ID（P4947 / P4983）
  - 中文/英文 Wikipedia 标题兜底
- TMDB 不可达时默认报错，避免静默生成错误目录名；交互模式下会保留已取回的字段并让你手动补充；非交互模式可加 `--fallback` 用 Wikidata 兜底。

## 配置

- `ICEDREEL_TMDB_TOKEN`：TMDB Read Access Token，推荐；走 `Authorization: Bearer`。
- `ICEDREEL_TMDB_APIKEY`：TMDB API Key，可选；走 `?api_key=`。
- `ICEDREEL_CELL_SIZE`：海报宽高比校正用的字符格像素尺寸，格式 `8x21`；不设时自动读 Windows Terminal 字体并实测。

两者都有时优先 Token。密钥不进命令行参数、不写仓库。

设置示例（用户环境变量，新开终端生效）：

```powershell
[Environment]::SetEnvironmentVariable('ICEDREEL_TMDB_TOKEN', '<token>', 'User')
```

TMDB 被墙或超时：

```powershell
$env:HTTPS_PROXY = 'http://127.0.0.1:7890'
icedreel name tt0111161 --douban 1292052 -y
```

Go 的默认 HTTP transport 会读取 `HTTPS_PROXY`。

## 构建

```powershell
cd <仓库目录>
go build -trimpath -ldflags "-s -w" -o IcedReel.exe .
```

## 测试

```powershell
go vet ./...
go test ./...
```

覆盖：

- 命名格式、缺失段、标题去重、全角括号、非法字符
- IMDb URL/纯数字归一化
- Wikidata 中英标签优先级、最早年份、TMDB/豆瓣 ID、Wikipedia 标题兜底、Wikidata ID
- TMDB `find` / search / movie / tv / external_ids 的 httptest 模拟
- IMDb `find` 同时返回电影/剧集时按完整度选择（票数、热度、海报、简介）
- 发布文件名解析、剧集季/集解析、ffprobe JSON 解析、语言/编码/声道/时长格式化
- 音轨显示（语言英文名、标题、序号、Default、编码、采样率、声道、位深）
- 字幕显示（语言英文名 + 每条标题，逐条不归并）
- 电影 NFO 生成、logicalname、streamdetails 映射、unified diff
- 剧集 / 季 / 单集 NFO 的读取与展示
- 半块字符渲染、Sixel 编码与缩放、poster/fanart 并排合成、海报来源优先级、`NO_COLOR` / `--no-poster` 跳过
- Windows Terminal 字体解析、字符格实测、`ICEDREEL_CELL_SIZE` 覆盖与宽高比补偿

## 后续

- `rank`：视频质量排名（ffprobe）。
- `tui`：全屏浏览视频与 NFO，复用 `show` 的数据层。
- 外挂字幕（最后做）。
