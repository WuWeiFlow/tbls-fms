# tbls-fms 服务器部署说明

本方案由 GitHub Actions 自动构建 `tbls-fms`，并将完整镜像发布到
GitHub Container Registry（GHCR）。服务器只需要访问 `ghcr.io`，不需要访问
Docker Hub，也不需要保留源代码或安装 Go。

## 一、首次发布镜像

1. 将包含本部署配置的 PR 合并到 `main` 分支。
2. 打开 GitHub 仓库的 **Actions** 页面。
3. 等待 **Publish GHCR image** 工作流执行成功。
4. 打开 GitHub 个人主页，进入 **Packages**，选择 `tbls-fms`。
5. 打开 **Package settings**，在页面底部选择 **Change visibility**，将镜像设置为
   **Public**。

GHCR 首次发布的镜像默认为私有。以上第 4、5 步只需执行一次。设置为公开后，
服务器无需执行 `docker login` 即可拉取镜像。

## 二、服务器目录

服务器上的运行目录保持如下结构：

```text
/root/db-doc/
├── .env
├── .tbls.yml
├── compose.yml
└── docs/
    └── database/
```

以前克隆的 `/root/db-doc/tbls-fms/` 源代码目录不再参与构建和运行，可以暂时保留，
不会产生影响。

## 三、修改服务器 compose.yml

先进入运行目录并备份原文件：

```bash
cd /root/db-doc
cp compose.yml compose.yml.bak
```

将 `/root/db-doc/compose.yml` 的内容全部替换为本目录下
[`compose.yml`](./compose.yml) 的内容：

```yaml
services:
  tbls:
    image: ghcr.io/wuweiflow/tbls-fms:v0.09
    env_file:
      - .env
    working_dir: /work
    volumes:
      - ./.tbls.yml:/work/.tbls.yml:ro
      - ./docs:/work/docs
    command:
      - doc
      - --force
      - --rm-dist
```

`.env`、`.tbls.yml` 和 `docs/` 不需要修改。数据库账号和密码仍然只保存在服务器的
`.env` 中，不会提交到 GitHub。

`.tbls.yml` 的完整注释模板见 [`tbls.example.yml`](./tbls.example.yml)，其中包含
FMS 自动关系、跨模块映射规则、手动指定关系、复合关系和多态关系示例。最终优先级为
手动指定 > 映射规则 > 自动识别；同一张表中未被高优先级关系占用的其他字段仍会
继续按低优先级识别。

模板还预置了以下文档组织配置：自动关系边线文字可配置；按表名前缀自动生成模块
Viewpoint，并通过 `none`、`parents`、`all` 控制跨模块表；同时保留完整 ER 图和
仅显示主键、关联字段及前 N 个普通字段的简化图；单表 Markdown 和单表 ER 图按
`sr/`、`eq/` 等前缀目录存放。`schema.json`、`schema.svg`、
`schema-compact.svg` 和 Viewpoint 文件仍保留在 `docPath` 根目录。

虚拟关系中被跳过的无效匹配会以中文告警输出到控制台，并同时写入 `docPath` 的同级
目录。例如 `docPath: docs/database` 会生成 `docs/virtual-relation-warnings.log`。
日志文件每次执行都会覆盖；本次没有告警时文件内容为空。

## 四、拉取并运行

首次部署时执行：

```bash
cd /root/db-doc
docker compose pull tbls
docker compose run --rm tbls
```

第一条命令从 GHCR 下载配置中指定版本的 `tbls-fms` 镜像；第二条命令连接数据库并重新生成
文档。由于命令中包含 `--rm-dist`，每次会先清理并完整重建 `docPath`（默认示例为
`docs/database`），可避免首次启用前缀目录后旧的根目录单表文件残留。
`docs/virtual-relation-warnings.log` 位于 `docPath` 同级，不会被该参数删除。

生产环境推荐固定版本号。发布新版本后，先将 `compose.yml` 中的版本号修改为
新版本（例如从 `v0.09` 修改为 `v0.10`），再执行以上两条命令。若希望每次拉取都自动跟随最新版，
也可以将镜像标签改回 `latest`。

如果只想使用服务器当前已经下载的镜像，不检查更新，可以只执行：

```bash
docker compose run --rm tbls
```

### 生成集中式 Excel 数据字典

```bash
docker compose run --rm tbls out -t xlsx -o docs/schema.xlsx
```

生成的 `docs/schema.xlsx` 不再为每张数据库表创建独立工作表，而是集中为“模块清单”、
“表清单”、“字段清单”、“关系清单”和“索引与约束”五张清单。模块名称可跳转到表清单，
表名可跳转到字段清单；所有清单均冻结表头并启用筛选。字段清单中的“关系描述”会直接显示
关联方向，例如 `关联sr_order表id_字段`。

## 五、验证使用的是 FMS 镜像

```bash
docker compose images
```

输出中的镜像名称应当是：

```text
ghcr.io/wuweiflow/tbls-fms
```

如果拉取时出现 `denied` 或 `unauthorized`，说明 GHCR 镜像仍是私有状态，请返回
第一部分完成一次公开设置。
