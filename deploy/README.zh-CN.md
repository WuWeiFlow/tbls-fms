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
    image: ghcr.io/wuweiflow/tbls-fms:latest
    env_file:
      - .env
    working_dir: /work
    volumes:
      - ./.tbls.yml:/work/.tbls.yml:ro
      - ./docs:/work/docs
    command:
      - doc
      - --force
```

`.env`、`.tbls.yml` 和 `docs/` 不需要修改。数据库账号和密码仍然只保存在服务器的
`.env` 中，不会提交到 GitHub。

`.tbls.yml` 的完整注释模板见 [`tbls.example.yml`](./tbls.example.yml)，其中包含
FMS 自动关系、手动指定关系、复合关系和多态关系示例。手动配置的子表字段优先于
自动识别，同一张表中未手动配置的其他字段仍会继续自动识别。

## 四、拉取并运行

首次部署以及以后更新程序时，执行：

```bash
cd /root/db-doc
docker compose pull tbls
docker compose run --rm tbls
```

第一条命令从 GHCR 下载最新的 `tbls-fms` 镜像；第二条命令连接数据库并重新生成
文档。由于命令中包含 `--force`，`docs/` 中同名的 `schema.json`、`schema.svg` 和
Markdown 文件会被新结果覆盖。

如果只想使用服务器当前已经下载的镜像，不检查更新，可以只执行：

```bash
docker compose run --rm tbls
```

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
