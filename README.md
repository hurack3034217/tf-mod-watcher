# Terraform Module Analyzer

Gitリポジトリ内の2つのコミット間で変更があったTerraformのルートモジュールを特定するCLIツール。
子モジュールを再帰的に検索し、それらを参照しているルートモジュールを「変更あり」として検出します。

## 機能

- Gitの2つのコミット間の差分を検出
- Terraformモジュールの依存関係を解析
- 再帰的な変更検知
- JSON形式での結果出力

## インストール

### ビルド

```bash
go build -o tf-module-analyzer
```

### 依存関係

- Go 1.25以上
- github.com/go-git/go-git/v5
- github.com/hashicorp/hcl/v2
- github.com/urfave/cli/v3

## 使い方

### 基本的な使用方法

```bash
# 検索ディレクトリを指定(複数指定可能)
# 指定されたディレクトリ配下のすべてのルートモジュールが自動的に検出されます
./tf-module-analyzer \
  --root-module-dir terraform/environments
```

### オプション

| オプション | 必須/任意 | デフォルト | 説明 |
|-----------|----------|-----------|------|
| `--before-commit` | 任意 | `HEAD^` | 比較対象の古いコミットハッシュまたは参照 |
| `--after-commit` | 任意 | `HEAD` | 比較対象の新しいコミットハッシュまたは参照 |
| `--root-module-dir` | 必須 | なし | ルートモジュールを検索するディレクトリ（カレントディレクトリからの相対パスまたは絶対パス、複数指定可）。指定されたディレクトリ配下のすべてのサブディレクトリから.tfファイルを含むディレクトリを再帰的に検索します。 |
| `--git-repository-root-path` | 任意 | 自動検出されたGitリポジトリルート | Git操作に使用するGitリポジトリのルートパス |
| `--base-path` | 任意 | `--git-repository-root-path`と同じ | 出力パスの相対パス計算の基準パス |
| `--log-level` | 任意 | `info` | ログレベル（`debug`, `info`, `warn`, `error`） |

### 出力形式

出力はJSON配列形式で、更新されたルートモジュールの`--base-path`からの相対パスが含まれます。

```json
["environments/prod", "environments/dev"]
```

更新がない場合は空の配列が出力されます。

```json
[]
```

### 使用例

#### 例1: HEADと1つ前のコミットを比較（デフォルト設定）

```bash
# environments配下のすべてのルートモジュールを自動検出
# base-pathを省略すると、Gitリポジトリのルートが自動的に使用されます
./tf-module-analyzer \
  --root-module-dir terraform/environments
```

#### 例2: 特定のコミット間を比較

```bash
# 複数のディレクトリを指定してルートモジュールを検索
./tf-module-analyzer \
  --before-commit abc123 \
  --after-commit def456 \
  --root-module-dir terraform/environments \
  --root-module-dir terraform/environments-general
```

#### 例3: base-pathを明示的に指定

```bash
# 出力パスの基準を明示的に指定する場合
./tf-module-analyzer \
  --root-module-dir terraform/environments \
  --base-path /path/to/repo
```

#### 例4: git-repository-root-pathとbase-pathを別々に指定

```bash
# Git操作のルートパスと出力パスの基準を別々に指定する場合
# （Gitリポジトリのルートとは異なるパスを基準に出力したい場合）
./tf-module-analyzer \
  --root-module-dir terraform/environments \
  --git-repository-root-path /path/to/git/repo \
  --base-path /path/to/git/repo/terraform
```

#### 例5: デバッグモードで実行

```bash
# デバッグモードで詳細なログを出力
./tf-module-analyzer \
  --root-module-dir terraform/environments \
  --log-level debug
```

## アーキテクチャ

### ディレクトリ構造

```text
terraform-module-analyzer/
├── main.go                      # エントリポイント
├── go.mod
├── go.sum
├── internal/
│   ├── analyzer/                # モジュール分析ロジック
│   │   ├── analyzer.go
│   │   └── analyzer_test.go
│   ├── git/                     # Git操作
│   │   ├── git.go
│   │   └── git_test.go
│   └── terraform/               # HCLパースと依存関係解決
│       ├── parser.go
│       └── parser_test.go
└── pkg/
    └── cli/                     # CLIインターフェース
        ├── app.go
        └── app_test.go
```

### 主要コンポーネント

#### 1. Git操作 (`internal/git`)

- `GetChangedFiles()`: 2つのコミット間で変更されたファイルのリストを取得
- go-gitライブラリを使用してGitリポジトリを解析

#### 2. Terraformパーサー (`internal/terraform`)

- `FindChildModules()`: モジュールが参照する子モジュールを検出
- HCL v2を使用してTerraformファイルをパース
- ローカルモジュールのみをサポート（リモートモジュールは無視）

#### 3. アナライザー (`internal/analyzer`)

- `IsModuleUpdated()`: モジュールが更新されたかを再帰的に判定
- キャッシング機構により、同じモジュールの重複分析を回避
- 直接的な変更と間接的な変更（子モジュール経由）の両方を検知

#### 4. CLI (`pkg/cli`)

- urfave/cli v3を使用したコマンドラインインターフェース
- 引数のパースと検証
- 結果のJSON出力

## テスト

### すべてのテストを実行

```bash
go test ./... -v
```

### 特定のパッケージのテストを実行

```bash
go test ./internal/analyzer -v
go test ./internal/git -v
go test ./internal/terraform -v
go test ./pkg/cli -v
```

### テストカバレッジ

```bash
go test ./... -cover
```

### テスト用モックデータ

`mock-terraform/` ディレクトリには、テスト用のTerraformモジュール構造が含まれています。
このディレクトリは変更しないでください。

### デバッグモードの使用

詳細なログを出力するには、`--log-level debug` を使用してください。

```bash
./tf-module-analyzer \
  --root-module-dir environments/prod \
  --base-path /path/to/repo \
  --log-level debug 2> debug.log
```

## ライセンス

MIT License
