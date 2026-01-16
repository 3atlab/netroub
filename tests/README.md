# Netroub Tests

このディレクトリにはnetroubのテストスイートが含まれています。

## テスト構造

```
tests/
├── integration/          # 統合テスト
│   ├── main_test.go             # TestMain（トポロジの一括デプロイ/破棄）
│   ├── delay_event_test.go      # 遅延イベントテスト
│   └── shell_copy_test.go       # シェル・コピーイベントテスト
├── topology/            # テスト用トポロジ定義
│   └── minimal_delay_test.yaml
├── data/                # テスト用データファイル
│   ├── minimal_data.json
│   └── test_input.txt           # コピーテスト用入力ファイル
├── scenarios/           # テストシナリオ
│   ├── *_nodeploy.json          # noDeploy用シナリオ（topoなし）
│   └── *.json                   # 通常シナリオ（topoあり）
└── README.md           # このファイル
```

## テスト効率化: TestMain + noDeploy

テスト実行時間を短縮するため、以下の構成を採用しています：

1. **TestMain**: テストスイート全体で一度だけトポロジをデプロイ/破棄
2. **noDeploy シナリオ**: `topo`フィールドなしのシナリオ（既存のコンテナを使用）

これにより、各テストでのデプロイ/破棄のオーバーヘッド（約10-15秒/回）を削減できます。

### noDeploy シナリオの例

```json
{
    "scenarioName": "delay-test",
    "logPath": "./test_logs",
    "duration": "10s",
    "hosts": ["r1", "r2"],
    "event": [...]
}
```

`topo`と`data`フィールドがないため、netroubはデプロイ/破棄をスキップし、イベントのみ実行します。

## 前提条件

テストを実行する前に、以下のツールがインストールされていることを確認してください：

1. **Docker** - コンテナ実行環境
2. **Containerlab** - ネットワークトポロジ展開
3. **Pumba** - カオス障害注入ツール
4. **sudo権限** - Docker操作に必要

### 前提条件チェック

```bash
make check-prereq
```

## テスト実行方法

### 1. 全テスト実行

```bash
make test
```

### 2. 統合テストのみ実行

```bash
make test-integration
```

### 3. 遅延イベントテストのみ実行

```bash
make test-delay
```

### 4. 並列遅延シナリオテスト

```bash
make test-delay-parallel
```

## テスト詳細

### DelayEventテスト

`tests/integration/delay_event_test.go`では、以下のテストを実行します：

1. **TestDelayEvent**: 50ms遅延の基本テスト
   - 2ノード間でベースライン RTT を測定
   - 50ms遅延を注入
   - 遅延適用後の RTT を測定
   - 統計的に遅延が適用されたことを検証

2. **TestMultipleDelayScenarios**: 複数遅延値の並列テスト
   - 10ms, 50ms, 100ms の3つのシナリオを並列実行
   - 実行時間を短縮しつつ包括的にテスト

### Shell/Copyイベントテスト

`tests/integration/shell_copy_test.go`では、以下のテストを実行します：

1. **TestShellAndCopyEvent**: シェルコマンド実行とファイル回収
   - コンテナ内でshellイベントを実行
   - 出力をファイルに書き込み
   - copyイベント（fromContainer）でファイルを回収
   - ファイル内容を検証

2. **TestCopyBidirectional**: 双方向コピー
   - copyイベント（toContainer）でファイルを配置
   - shellイベントでファイルを処理
   - copyイベント（fromContainer）で結果を回収
   - 入力と出力の内容を検証

3. **TestCopyWithPermissions**: パーミッション付きコピー
   - ファイルのコピーと権限設定を検証

### テスト実行フロー

```
1. 環境セットアップ (0-30秒)
   ├── Containerlab でトポロジ展開
   ├── コンテナ起動・ネットワーク設定
   └── 疎通確認・安定化待機

2. ベースライン測定 (30-40秒)
   └── ping による RTT 測定 (10回)

3. 遅延注入 (40-45秒)
   └── netroub でシナリオ実行

4. 遅延測定 (50-60秒)
   └── 遅延適用後の RTT 測定 (10回)

5. 検証・クリーンアップ (60-90秒)
   ├── 統計解析・アサーション
   └── 環境破棄
```

### 測定精度と許容誤差

- **測定方法**: ping による Round-Trip Time
- **測定回数**: 10回の平均値で統計処理
- **許容誤差**: 期待値の±20% または ±10ms (大きい方)
- **ジッター制御**: 標準偏差が元の3倍以下であることを確認

## リモート環境でのテスト実行

開発環境（macOS等）ではDocker/Containerlabが動作しない場合、専用のLinux環境でテストを実行します。

### 環境同期

```bash
# ローカルからリモートへ同期（~/project/go/から実行）
./rsync-project netroub <remote-host>

# 例
./rsync-project netroub glados
```

### リモートでのテスト実行

```bash
# リモート環境にSSH接続
ssh <remote-host>
cd ~/netroub

# goコマンドがPATHにない場合はフルパスで実行
sudo -E /usr/local/go/bin/go test -v -timeout 10m ./tests/integration/...

# または
sudo env "PATH=$PATH" go test -v -timeout 10m ./tests/integration/...
```

### 特定テストのみ実行

```bash
# Delayテストのみ
sudo -E /usr/local/go/bin/go test -v -timeout 5m -run TestDelayEvent ./tests/integration/

# Shell/Copyテストのみ
sudo -E /usr/local/go/bin/go test -v -timeout 5m -run TestShellAndCopyEvent ./tests/integration/
```

## トラブルシューティング

### よくある問題

1. **権限エラー**
   ```bash
   sudo -E go test -v ./tests/integration/
   ```

2. **Docker接続エラー**
   ```bash
   sudo systemctl start docker
   sudo usermod -aG docker $USER  # 再ログイン後有効
   ```

3. **Containerlab not found**
   ```bash
   # Ubuntu/Debian
   curl -sL https://containerlab.dev/setup | sudo bash
   
   # macOS
   brew install containerlab
   ```

4. **テスト環境が残る場合**
   ```bash
   make clean
   ```

### デバッグ情報

テスト実行時に詳細なログを確認したい場合：

```bash
sudo -E go test -v -timeout 10m ./tests/integration/ -args -test.v
```

## 今後の拡張予定

- [ ] パケットロステスト
- [ ] 帯域制限テスト
- [ ] ストレステスト
- [ ] 設定変更テスト (configイベント)
- [ ] マルチホップトポロジテスト
- [x] シェルイベントテスト
- [x] コピーイベントテスト