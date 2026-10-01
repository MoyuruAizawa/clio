# Clio

[English](README.md)

Clioはcoding agent向けのコマンド実行wrapperです。出力をcaptureし、失敗の手がかりになる元ログをTypeSafe Jevで選択します。
Clioはログを保存せず、ログの書き換えや診断文の生成もしません。

開発はAIが担当し、設計は人間が担当しています。

## buildとinstall

Go 1.26が必要です。macOS／Linuxで、プロジェクトのルートから実行してください。

```sh
go build -o ./clio ./cmd/clio
mkdir -p "$HOME/.local/bin"
install -m 755 ./clio "$HOME/.local/bin/clio"
export PATH="$HOME/.local/bin:$PATH"
```

別のterminalでも使うには、`export PATH`の行をshellの設定ファイルに追加してください。
installせず使う場合は、プロジェクトのルートで`./clio`を実行できます。

## 使い方

対話型terminalで`clio auth login`を実行し、TypeSafeのAPI tokenを入力します。
入力内容は表示されません。loginはtokenを保存するだけで、APIでの有効性確認はしません。
認証情報が必要なのは、失敗したコマンドのログをfilteringするときだけです。

```sh
clio auth login
clio exec -- go test ./...
clio exec --mode status --timeout 2m -- go build ./...
clio exec --mode full -- git diff
clio exec --context "Verify that the pendant changes compile into the debug APK." -- ./gradlew assembleDevelopDebug
```

| mode | コマンド成功時 | コマンド失敗時 |
| --- | --- | --- |
| `on-error`（既定値） | メタデータのみ | 選択された元ログとメタデータ |
| `status` | メタデータのみ | メタデータのみ |
| `full` | captureしたstdout／stderrとメタデータ | captureしたstdout／stderrとメタデータ |

どのmodeでも出力をメモリにcaptureし、コマンドの終了後に返します。`full`も実行中の進捗をリアルタイムには表示しません。stdoutとstderrは別々にcaptureするため、両者をまたぐ時系列は再現しません。`full`はそれぞれを元のstreamに返します。filteringが成功した場合は、`[stdout]`と`[stderr]`のセクションを含むログをstdoutに返します。

メタデータはstderrに出力し、exit code、子processの実行時間、該当する場合のsignal、timeout、中断状態を含みます。子processのexit codeは維持します。macOS／Linuxのsignal終了では`128 + signal`を返します。
CLIの引数エラーは2、Clio内部のエラーは1です。

`--`以降の引数は境界を保ったまま直接実行します。stdin、working directory、環境変数を継承しますが、`TYPESAFE_API_KEY`は除外します。shellの構文は解釈しないため、`clio exec -- 'echo 1'`ではなく`clio exec -- echo 1`と書いてください。
shellの構文が必要な場合は、明示的にshellを実行します。

```sh
clio exec -- sh -c 'printf "hello\n" && printf "world\n"'
```

`--timeout`は子processの実行時間だけを制限します。省略時、または`--timeout 0`では無制限なので、数分以上かかるbuildも実行できます。filteringには別の30秒の期限があり、すべてのbatchとretryを合わせた時間に適用します。
HTTP 429／529は期限内で最大2回retryし、backoffと`Retry-After`に従って待機します。
macOS／Linuxでは、中断やtimeout時に子processのprocess groupを終了します。

必要に応じて`--`の前に`--context "<目的>"`を指定し、agentの目的を1〜2文で伝えられます。Clioはこれを`agent_context`として、実行コマンド、exit code、ログのchunkと一緒に送信します。Jevは失敗の診断と目的に向けた次の行動に役立つ情報を判定します。別のmoduleのエラーでも、コマンドを妨げるものは判定対象です。contextを省略すると、従来の失敗診断向けの質問を使います。contextは子processの引数・環境変数やメタデータには追加しません。TypeSafeに送るのはfilteringが必要なときだけです。認証情報や会話全体は含めないでください。

filterの失敗、不正な回答、認証情報の未設定、非空ログからの選択ゼロなどは、理由とともに`filtering failed`を表示します。fallbackはstdout／stderrの末尾をそれぞれ最大4 KiB、元のstreamに返し、子processのexit codeを維持します。
`status`、`full`、成功した`on-error`は認証情報を必要としません。

## 認証情報とmodel

認証情報は`os.UserConfigDir()/clio/credentials.json`に保存します。

- macOS: `~/Library/Application Support/clio/credentials.json`
- Linux: 通常は `~/.config/clio/credentials.json`

Clioのdirectoryは0700、fileは0600の権限で作成します。同じdirectoryの一時fileからatomic renameで更新します。symlink、所有者の不一致、緩すぎる権限は拒否します。**保存内容は平文です。** 権限は他ユーザーのアクセスを制限しますが、同じユーザーで動くprocessからの読み取りは防げません。tokenはfilteringが必要なときだけ読み込み、Bearer認証で送信します。既存の`TYPESAFE_API_KEY`はClioの認証情報として使いません。
`TYPESAFE_MODEL`で既定の`jev-latest`を変更できます。

## filteringとTypeSafeに送るデータ

`on-error`でコマンドが失敗すると、captureしたstdoutとstderrを正規化し、chunkに分けてTypeSafe APIに送信します。送信対象はagentに返すchunkだけではなく、captureしたログ全体です。requestには実行コマンド、exit code、任意のcontext、前後のchunkの抜粋も含みます。Clioはログやコマンド引数に含まれる秘密情報をマスキングしません。`status`、`full`、成功した`on-error`はTypeSafeを呼び出しません。

判定用のテキストではANSI escapeを除去し、carriage returnを改行に変換します。
chunkは正規化後のテキストで最大4 KiBとし、行境界を優先して分割します。
上限を超える単一行は途中で分割します。返却する元ログはANSI escapeや元のcarriage returnを保持するため、元ログの範囲は4 KiBより大きくなる場合があります。

Jevは最大8 chunkのbatchごとにNoulの確率で判定します。0.5以上のchunkを丸ごと選択し、同じstreamの前のchunkの末尾10行、後ろのchunkの先頭10行が必要かどうかも別々に判定します。選択されたchunkに通常の進捗行が残ることがあります。
Jevにログの要約、書き換え、診断はさせません。

重複する範囲は統合します。選択されたchunkと必要な前後の抜粋をひとまとまりとして、関連度の高い順にstdout／stderr合計で最大16 KiBの元ログを採用します。残りの上限に収まらないまとまりは、丸ごと省略します。何も収まらない場合はfiltering failureとして扱い、前述の末尾ログを返します。返却時は各stream内の元順序を保ち、stdout、stderrの順に並べます。ラベルと省略マーカーのbyte数は上限とは別です。`[clio: output omitted]`は省略された出力を示します。後半のstderrセクションも含め、返却された内容全体を読んでから失敗を診断してください。

診断に必要な情報が不足するときは`full`を使ってください。Clioはログを保存しないため、過去の実行の全文を得るには再実行が必要です。install、deployなどを再実行するときは副作用を考慮してください。

## agent skill

[skills/clio/SKILL.md](skills/clio/SKILL.md)はmodeの選択、目的のcontext、省略マーカーの扱い、`full`で再実行する条件を説明します。`skills/clio` folderをagentのskills directoryにコピーするか、プロジェクトの指示から参照してください。
skillはagentに使い方を伝えるもので、`clio` binaryのinstallとPATH設定も必要です。

## 制限と開発時の確認

captureは出力量に比例するメモリを使用します。この版ではdisk spill、ログ保存、コマンド固有のheuristic、Windowsのprocess tree制御には対応していません。子processの終了後、子孫processが出力pipeを開いたままにしていても、200msの期限で読み取り待機を終えます。その期限より後の出力は、`full`でも失われる可能性があります。

Clioをinstallした状態で、プロジェクトのルートから実行してください。

```sh
clio exec -- go test -race ./...
clio exec -- go vet ./...
clio exec -- go build -o ./clio ./cmd/clio
```

testはfake Filter、fake Jev client、ローカルHTTP serverを使用し、live APIは不要です。必ず成功する、または意図的に失敗する軽量コマンドについては[tests/smoke/README.md](tests/smoke/README.md)を参照してください。
HTTP連携には[TypeSafe API](https://docs.typesafe.ai/api)を使用します。
