# sango-tree

ポリレポ開発オーケストレーター。複数のリポジトリで構成されるプロジェクトを、一つの設定ファイルで起動・管理する。

**sango** = サンゴ（珊瑚）。海中に根を張るサンゴの木（coral tree）のように、複数のサービスを一つの幹から分岐させて育てる。

## 特徴

- **ポリレポ対応**: 複数リポジトリのサービスを `sango.yaml` 一つで定義・起動
- **Worktreeベースの並行開発**: Git worktree を活用してブランチごとに独立した開発環境を構築
- **依存関係の自動解決**: DAGベースのサービス起動順序制御
- **ポートの自動管理**: Worktreeごとにオフセットを加算し、ポート衝突を回避
- **ヘルスチェック**: HTTP / コマンドベースのヘルスチェックと自動リストア
- **構造化ログ**: サービスごとのJSONLログ収集、フィルタリング、フォロー
- **Doctor**: 環境チェック＆自動修復
- **Troubleshoot**: サービス単位の自動診断チェック
- **Runbook**: 症状→原因→手順のナレッジベース検索

## インストール

```bash
go install github.com/ryugen04/sango-tree@latest
```

## クイックスタート

```bash
# 1. 設定ファイルを生成
sango init

# 2. sango.yaml を編集してサービスを定義
# 3. サービスを起動
sango up

# 4. 状態を確認
sango status

# 5. 停止
sango down
```

## コマンドリファレンス

| コマンド | 説明 |
|---------|------|
| `sango init` | `sango.yaml` テンプレートを生成 |
| `sango up [services...] [--profile name]` | サービスを起動 |
| `sango down [--all]` | サービスを停止（`--all` でsharedも含む） |
| `sango restart [services...] [--profile name]` | サービスを再起動 |
| `sango status` | サービスの状態を表示 |
| `sango doctor [--fix]` | 環境チェックを実行 |
| `sango logs [services...] [-f] [-n N] [--since 5m] [--grep pattern] [--level error] [--json]` | ログを表示 |
| `sango clone [--shallow]` | リポジトリをbare clone＆初期worktree作成 |
| `sango worktree create <branch>` | ワークツリーを作成 |
| `sango worktree list` | ワークツリー一覧 |
| `sango worktree status` | 全ワークツリーの状態を表示 |
| `sango worktree switch <branch>` | アクティブワークツリーを切替 |
| `sango worktree remove <branch>` | ワークツリーを削除 |
| `sango worktree verify [branch]` | ワークツリーのinclude状態を検証 |
| `sango troubleshoot [service] [--fix]` | サービスのトラブルシュートチェックを実行 |
| `sango runbook search <keyword>` | Runbookをキーワード検索 |
| `sango runbook list [--service name]` | Runbook一覧を表示 |

## 設定ファイル

`sango.yaml` の基本構造:

```yaml
name: my-project
version: "1.0"

services:
  api:
    type: process
    port: 3000
    command: npm start
    working_dir: ./api
    depends_on: [postgres]
    healthcheck:
      url: http://localhost:3000/health
      interval: 5s
      retries: 3

  postgres:
    type: docker
    image: postgres:16
    port: 5432
    shared: true
    volumes:
      - pgdata:/var/lib/postgresql/data

ports:
  strategy: fixed
  base_offset: 100
  range: [3000, 9999]

profiles:
  backend:
    services: [api, postgres]

doctor:
  checks:
    - name: Node.js
      command: node --version
      expect: "v"
      fix: "brew install node"
```

詳細は [docs/configuration.md](docs/configuration.md) を参照。

## ドキュメント

- [設定リファレンス](docs/configuration.md) - `sango.yaml` の全スキーマ
- [Worktreeガイド](docs/worktree.md) - Worktreeベースの並行開発ワークフロー

## ライセンス

MIT
