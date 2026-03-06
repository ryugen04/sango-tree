# 設定リファレンス

`sango.yaml` の全スキーマを解説する。

## トップレベル

```yaml
name: my-project        # プロジェクト名（必須）
version: "1.0"          # 設定バージョン

services: {}            # サービス定義
ports: {}               # ポート割り当て設定
profiles: {}            # サービスグループ定義
doctor: {}              # 環境チェック設定
worktree: {}            # ワークツリー管理設定
log: {}                 # ログ管理設定
```

---

## services

サービスの定義。キーがサービス名になる。

```yaml
services:
  api:
    type: process
    port: 3000
    command: npm start
```

### Service フィールド

| フィールド | 型 | 必須 | 説明 |
|-----------|---|------|------|
| `type` | string | Yes | `process` / `docker` / `script` |
| `port` | int | No | 待ち受けポート番号 |
| `shared` | bool | No | `true` の場合ワークツリー間で共有（DB等） |
| `depends_on` | []string | No | 先に起動するサービス名リスト |
| `working_dir` | string | No | 作業ディレクトリ |
| `setup` | []string | No | 初回セットアップコマンド |
| `command` | string | process/script時必須 | 実行コマンド |
| `command_args` | []string | No | コマンド引数 |
| `env` | map | No | 環境変数 |
| `env_file` | string | No | .envファイルパス |
| `env_dynamic` | map | No | 動的環境変数（コマンド実行結果を値にする） |
| `image` | string | docker時必須 | Dockerイメージ |
| `volumes` | []string | No | Dockerボリューム |
| `healthcheck` | object | No | ヘルスチェック設定 |
| `restart` | string | No | 再起動ポリシー（`always` / `on-failure`） |
| `restart_delay` | string | No | 再起動間隔（例: `1s`） |
| `max_restarts` | int | No | 最大再起動回数 |
| `repo` | string | No | クローン元リポジトリURL |
| `repo_path` | string | No | 既存リポジトリのローカルパス |
| `run_on` | []string | No | 実行対象ワークツリー名リスト |
| `troubleshoot` | []object | No | トラブルシュート定義 |
| `runbook` | []object | No | Runbook定義 |

### サービスタイプ

- **process**: ローカルプロセスとして起動。`command` が必須
- **docker**: Dockerコンテナとして起動。`image` が必須
- **script**: 一度だけ実行するスクリプト。`command` が必須

---

## Healthcheck

```yaml
healthcheck:
  url: http://localhost:3000/health   # HTTP URL（URLまたはcommandのどちらか）
  command: pg_isready                 # チェックコマンド
  interval: 5s                        # チェック間隔（デフォルト: 5s）
  timeout: 3s                         # タイムアウト（デフォルト: 3s）
  retries: 3                          # リトライ回数
  start_period: 10s                   # 起動猶予期間（デフォルト: 0s）
```

---

## ports

```yaml
ports:
  strategy: fixed        # fixed（固定） or offset（オフセット加算）
  base_offset: 100       # ワークツリーごとのオフセット増分
  reserved: [5432, 6379] # 予約済みポート
  range: [3000, 9999]    # 使用可能なポート範囲
```

---

## profiles

サービスをグループ化して一括操作できる。

```yaml
profiles:
  backend:
    services: [api, postgres, redis]
  frontend:
    services: [web, bff]
```

```bash
sango up --profile backend
```

---

## doctor

環境チェック項目を定義する。

```yaml
doctor:
  checks:
    - name: Node.js
      command: node --version
      expect: "v"              # 出力に含まれるべき文字列
      fix: "brew install node" # 修復コマンド
```

```bash
sango doctor       # チェック実行
sango doctor --fix # チェック＆自動修復
```

---

## worktree

Git worktreeの管理設定。

```yaml
worktree:
  base_dir: ./worktrees       # ワークツリー配置先
  auto_setup: true             # 作成時にsetupコマンドを自動実行
  default_branch: main         # デフォルトブランチ名

  include:
    common:                    # 全サービス共通のファイル配置
      - source: .env.template
        target: .env
        strategy: copy         # copy / symlink / template
    per_service:               # サービス固有のファイル配置
      api:
        - source: configs/api.env
          target: .env
          strategy: copy

  hooks:
    post_create:               # ワークツリー作成後フック
      - command: npm install
        per_service: true      # 各サービスディレクトリで実行
    pre_remove:                # ワークツリー削除前フック
      - command: docker compose down
```

---

## log

ログ管理の設定。

```yaml
log:
  max_size: 50MB     # ログファイルの最大サイズ
  max_files: 5       # 保持するログファイル数
  max_age: 7d        # ログファイルの最大保持期間
  compress: true     # ローテーション時のgzip圧縮
```

---

## Troubleshoot

サービスごとのトラブルシュート定義。`expect` の評価は `doctor` と同じロジック（部分一致、`<N%` パターン対応）。

```yaml
services:
  api:
    troubleshoot:
      - name: ポート競合チェック
        command: lsof -i :3000
        description: ポート3000が使用中か確認
        expect: ""
        fix: "kill -9 $(lsof -ti :3000)"
```

### CLIコマンド

```bash
sango troubleshoot              # 全サービスのチェックを実行
sango troubleshoot api          # 指定サービスのみ
sango troubleshoot api --fix    # 失敗チェックのfixコマンドを自動実行
```

全チェックがpassなら終了コード0、failがあれば終了コード1。

---

## Runbook

サービスごとのナレッジベース。キーワード検索で症状から対応手順を引き当てる。

```yaml
services:
  api:
    runbook:
      - title: APIが起動しない
        symptoms:
          - "Error: listen EADDRINUSE"
          - ポート3000が使用中
        cause: 前回のプロセスが残っている
        steps:
          - "lsof -i :3000 でプロセスを確認"
          - "kill -9 <PID> で終了"
          - "sango up api で再起動"
        tags: [port, startup]
```

### CLIコマンド

```bash
sango runbook search "connection"      # キーワード検索（title, symptoms, cause, tagsが対象）
sango runbook list                     # 全Runbookを一覧
sango runbook list --service api       # サービスで絞り込み
```

検索はcase-insensitiveの部分一致。マッチしたフィールド（title/symptoms/cause/tags）が結果に表示される。

---

## 変数展開

設定値内で以下の変数が利用可能:

| 変数 | 説明 |
|------|------|
| `${PORT}` | サービスに割り当てられたポート番号 |
| `${WORKTREE}` | アクティブワークツリー名 |
| `${SERVICE}` | サービス名 |
