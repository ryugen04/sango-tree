# Worktree ガイド

sango-tree は Git worktree を活用して、ブランチごとに独立した開発環境を構築する。

## 概要

```
.sango/
├── bare/              # bare リポジトリ
│   ├── api.git/
│   └── bff.git/
├── pids/              # PIDファイル
│   ├── main/
│   └── feature___auth/
├── logs/              # ログファイル
│   ├── main/
│   └── feature___auth/
├── locks/             # ロックファイル
└── worktrees.json
```

## ワークフロー

### 1. リポジトリのクローン

```bash
sango clone
```

`sango.yaml` の `repo` フィールドを持つ各サービスをbare cloneし、`main` ワークツリーを作成する。

- `--shallow`: shallow clone で高速化

### 2. ワークツリーの作成

```bash
sango worktree create feature/auth
```

全サービスに対してワークツリーを作成する。

オプション:

| フラグ | 説明 |
|-------|------|
| `--services s1,s2` | 対象サービスを限定 |
| `--from base-branch` | 分岐元ブランチを指定（デフォルト: main） |
| `--no-setup` | セットアップコマンドをスキップ |

作成時の処理:
1. 各サービスのbare repoから新ブランチでworktreeを作成
2. `include` 設定に基づいてファイルを配置
3. `auto_setup: true` なら各サービスの `setup` コマンドを実行
4. `hooks.post_create` フックを実行

### 3. ワークツリーの切替

```bash
sango worktree switch feature/auth
```

アクティブなワークツリーを切り替える。

| フラグ | 説明 |
|-------|------|
| `--stop-current` | 現在のワークツリーのサービスを停止 |
| `--start` | 切替先のワークツリーのサービスを起動 |

### 4. 状態確認

```bash
# ワークツリー一覧
sango worktree list

# 全ワークツリーの詳細状態（実行中プロセス含む）
sango worktree status
```

### 5. ワークツリーの削除

```bash
sango worktree remove feature/auth
```

| フラグ | 説明 |
|-------|------|
| `--force` | 変更がある場合でも強制削除 |

削除時の処理:
1. 実行中のプロセスを停止
2. `hooks.pre_remove` フックを実行
3. 各サービスのworktreeを削除
4. PIDディレクトリをクリーンアップ

## ポートオフセット

Worktreeごとにポートオフセットが自動的に割り当てられ、ポート衝突を回避する。

```yaml
ports:
  strategy: fixed
  base_offset: 100
```

例: `base_offset: 100` の場合:
- `main`: オフセット 0 → api: 3000, bff: 4000
- `feature/auth`: オフセット 100 → api: 3100, bff: 4100
- `feature/payment`: オフセット 200 → api: 3200, bff: 4200

## Include 設定

ワークツリー作成時にファイルを自動配置する。

```yaml
worktree:
  include:
    root:
      # worktreeルートに配置（共有フォルダのsymlink等）
      - source: ../.claude
        target: .claude
        strategy: symlink
        required: true
    per_service:
      api:
        - source: ../templates/api.env.tmpl
          target: .env
          strategy: template
          required: true
        - source: ../shared/tsconfig.json
          target: tsconfig.json
          strategy: symlink
```

### エントリの種類

| セクション | 配置先 | 説明 |
|-----------|--------|------|
| `root` | worktreeルートディレクトリ | 全サービスで共有するファイル/ディレクトリ |
| `per_service` | 各サービスディレクトリ | サービス固有のファイル |

### Strategy

| 値 | 説明 | ディレクトリ対応 |
|---|------|---------------|
| `copy` | ファイルをコピー | 不可 |
| `symlink` | シンボリックリンクを作成 | 可（ディレクトリ丸ごとsymlink） |
| `template` | テンプレートとして展開（変数置換あり） | 不可 |

### Required フラグ

`required: true` を指定すると、そのエントリの展開に失敗した場合にworktree作成が中止される。
デフォルトは `false`（失敗しても警告のみ）。

## Include 検証

作成済みworktreeのinclude状態を検証する。

```bash
# アクティブなworktreeを検証
sango worktree verify

# 指定ブランチのworktreeを検証
sango worktree verify feature/auth
```

検証結果:
- `ok`: 正常
- `missing`: ターゲットが存在しない
- `mismatch`: 内容やリンク先が不一致
- `broken_link`: symlinkのリンク先が存在しない

終了コード: 全OK=0、required失敗あり=1

## Hooks

ワークツリーのライフサイクルにフックを設定できる。

```yaml
worktree:
  hooks:
    post_create:
      - command: npm install
        per_service: true    # 各サービスディレクトリで実行
      - command: echo "done" # プロジェクトルートで実行
    pre_remove:
      - command: docker compose down
```

- `per_service: true`: 各サービスのワークツリーディレクトリで実行
- `per_service: false`（デフォルト）: プロジェクトルートで一度だけ実行

## Shared サービス

`shared: true` のサービス（DB等）はワークツリー間で共有され、一つのインスタンスのみ起動する。

```yaml
services:
  postgres:
    type: docker
    image: postgres:16
    port: 5432
    shared: true
```

shared サービスのポートはオフセットの影響を受けない。
