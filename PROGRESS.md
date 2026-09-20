# carty 진행 상황

carty(README 참고)의 Go 구현 진행 상황 기록. 코드베이스에서 바로 알 수 없는
결정/이유 위주로 남긴다.

---

## 현재 상태 요약

- `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` 모두 통과
  (41개 테스트, 전부 네트워크 없이 실행됨 — `t.TempDir()`와 캐시 폴백만
  씀, 실제 GitHub에 접근하지 않음).
- 핵심 로직(마커 적용/재적용/제거, 백업/복구, depends_on 자동 추가, 충돌 검사,
  마커 손상 복구 y/n 플로우, 캐시 폴백, selfupdate 상태/체인지로그, TUI
  스크롤/최소 터미널 크기 가드)은 리포에 영구히 남는 `_test.go`로 검증됨
  (아래 "테스트" 참고).
- TUI를 실제 pty(가상 터미널) + `pyte` 터미널 에뮬레이터로 여러 번 구동해봄
  — naive한 ANSI 스트리핑 방식은 bubbletea의 부분 리드로우 때문에 오탐을
  낼 수 있다는 걸 겪은 뒤로는 `pyte`로 실제 화면 버퍼를 재구성해서 검증함
  (아래 "TUI 수동 검증" 참고).
- 릴리즈 파이프라인(`.github/workflows/release.yml`), CI(`ci.yml`), 설치
  스크립트(`scripts/install.sh`)를 추가함 (아래 "릴리즈/설치 파이프라인"
  참고).
- **TUI를 카드형 레이아웃으로 전면 재작업**: alt-screen 전체화면, 헤더/푸터
  바, 항목을 테두리 있는 카드로 표시(선택 여부는 테두리 색/굵기로 구분),
  목록에 스크롤 + 스크롤바, 터미널이 너무 작으면 레이아웃이 깨지는 대신
  안내 메시지 표시 (아래 "TUI 카드형 레이아웃 + 스크롤" 참고).
- 데이터 저장소(`carty-data`)에 **zsh 테마 30개 + fish 테마 8개**를 실제로
  채워 넣었고, index.json을 손으로 관리하던 걸 **frontmatter → index.json
  자동 생성 CI**로 바꿈 (실제 GitHub Actions에서 push/PR 양쪽 다 검증
  완료). 아래 "데이터 카탈로그" / "carty-data CI" 참고.
- 이 리포(`carty`, Go 프로그램 본체)는 **이번에 처음 git 저장소로 초기화하고
  GitHub에 올림** (`dvdsvds/carty`). 그 전까지는 로컬 디렉터리로만 작업하던
  상태였음.
- **커스텀 프롬프트 구성 기능** 추가: 완성된 테마를 고르는 대신, 항목(경로/
  git/user@host/프롬프트 문자)을 자유 순서·개수로 골라 담고 항목마다 색을
  직접 고르는 새 카테고리(`zsh-custom`, `apply_method: "compose"`). 색상은
  MS페인트 스타일 2D 그리드(색조×채도)를 hjkl/방향키로 움직이며 고름.
  실제 GitHub Actions CI까지 통과한 데이터와 함께 pty+개별 키 입력으로
  전체 플로우(항목 담기 → 색 선택 → 재정렬 → 적용) 끝까지 검증함 (아래
  "커스텀 프롬프트 구성" 참고).

---

## 패키지별 구현 내용

### `cmd/carty/main.go`
- 인자 없이 실행 → 락 획득 후 TUI 실행.
- `carty update` / `upgrade` / `uninstall` / `--version` 서브커맨드 라우팅.
- 시작 시 `selfupdate.CheckAndApply`를 best-effort로 호출 (네트워크 실패해도
  carty 실행 자체는 막지 않음).

### `internal/catalog`
- `Category`, `Item`, `Index` 구조체 + `ParseIndex`, `ExtractBody`.
- `Item`에는 별도 `id` 필드가 없음 — **`FilePath`를 항목의 고유 식별자로
  사용** (아래 스키마 결정 참고).
- `ResolveTargetFile(category, home)`: `target_file_env` 환경변수가 설정돼
  있으면 그 값을, 아니면 `target_file` 기본값을 쓰고 `~`를 홈으로 치환.

### `internal/apply`
- 마커 구역 적용(`Apply`), 삭제(`Remove`), 원자적 쓰기(rename 기반)는 기존
  코드 유지.
- **추가**: `Apply`가 파일을 건드리기 전에 `backup.EnsureBackup` 호출 (최초
  1회 백업 보장).
- **추가**: 시작 마커는 있는데 종료 마커가 없으면 `ErrCorruptMarker` 반환 —
  덮어쓰지 않고 호출자(TUI)가 사용자 동의를 받아 복구하도록 함.
- **추가**: `Remove` — 장바구니에서 빠진 카테고리의 구역을 파일에서 통째로
  제거.
- **추가**: `RestoreFromBackup` — 손상 감지 후 사용자가 동의하면 백업으로
  되돌림.

### `internal/backup` (신규)
- `EnsureBackup(backupDir, targetFile)`: 대상 파일마다 **최초 1회만** 백업.
  이미 백업이 있으면 아무것도 안 함 (README의 "항상 진짜 원본 보존" 요구사항).
- `Restore`: 백업이 있으면 그 내용으로 복원, 없으면 (carty가 새로 만든
  파일이라는 뜻이므로) 파일 자체를 삭제.
- 파일명 매핑은 대상 경로의 `/`를 `_`로 치환한 flat 파일명 (예:
  `/home/u/.zshrc` → `_home_u_.zshrc`).

### `internal/resolve` (신규)
- `Resolve(selected, all)`: 선택된 항목들의 `depends_on`을 재귀적으로 펼쳐서
  자동 추가 (`Auto: true`로 표시). 순환 의존이 있으면 `ErrCyclicDependency`
  반환 (CI가 데이터 저장소 단에서 미리 막긴 하지만, 방어적으로 런타임에서도
  검사).
- `DetectConflicts(items, targetFileFor)`: 같은 대상 파일에 적용될 항목들
  사이에 alias/function 이름이 겹치면 충돌로 보고.
- 키는 `Item.FilePath` (README 예시의 "id" 대신 — 스키마 결정 참고).

### `internal/cache` (신규)
- `index.json`과 개별 설정 파일(`files/...`)을 `~/.carty/cache/`에 저장/조회.
- TUI의 `fetchIndexBytes` / `fetchItemBody`가 네트워크 실패 시 이걸로
  폴백해서 오프라인 브라우징/적용을 지원.

### `internal/selfupdate`
- 기존 `LatestVersion`/`Apply`에 있던 버그 수정: GitHub API 경로가
  `release/latest`(오타)였던 걸 `releases/latest`로 수정, 응답에서 `body`
  (체인지로그)와 `assets`도 파싱하도록 확장.
- `SelectAsset`: `carty_<goos>_<goarch>` 이름의 릴리즈 에셋을 찾음. **주의**:
  이 이름 규칙은 실제 릴리즈 워크플로우가 아직 없어서 확정된 게 아니라
  가정임 — 실제 CI 릴리즈 스크립트를 만들 때 맞춰야 함.
- `CheckAndApply(stateDir, currentVersion)`: 24시간에 한 번만 GitHub를
  확인(`internal/selfupdate/state.go`의 `ShouldCheck`/`recordChecked`), 새
  버전이 있으면 다운로드해서 자기 자신을 덮어쓰고, `SavePendingChangelog`로
  변경 사항을 기록. `currentVersion == "dev"`일 때는 개발 바이너리가 자기
  자신을 덮어쓰는 사고를 막기 위해 아예 스킵.
- `LoadPendingChangelog`: 다음 실행에서 TUI가 한 번만 보여주고 파일을
  지움 (재실행해도 다시 안 뜸).

### `internal/state`
- `Entry`에 `ItemPath`(구 `ItemID`, 스키마 결정으로 개명)와 `Version` 추가 —
  `upgrade`가 "설치된 버전 vs 최신 버전"을 비교할 수 있게.
- `LoadOrEmpty` 추가 (state.json이 아직 없어도 에러 대신 빈 State).

### `internal/appcmd` (신규)
- `Update()`: index.json만 새로 받아 캐시에 저장 (시스템 파일은 안 건드림,
  `apt update`와 동일한 감각).
- `Upgrade()`: 캐시된 인덱스(없으면 먼저 fetch)와 `state.json`을 비교해서
  버전이 바뀐 항목만 다시 fetch/apply.
- `Uninstall(confirm)`: `state.json`에 기록된 모든 target file을 백업으로
  복원하고 `~/.carty` 전체 삭제. `confirm` 콜백으로 실제 파괴적 동작 전에
  y/N 확인 (`ConfirmStdin`).

### `internal/tui`
전면 재작성. README의 "장바구니형 UI" 동작을 화면 상태 머신으로 구현:

- `screenCategories` → `screenItems` → `screenDetail` (←/→ 또는 h/l로 이동),
  카테고리당 라디오 버튼 선택(`space`/`enter`), 검색(`/`, 아무 화면에서나 —
  단 목록류 화면에서만), 수동 새로고침(`r`, 캐시 갱신 + 실패 시 오프라인
  안내).
- `screenConfirm`: `depends_on` 자동 추가 표시, 충돌 발견 시 적용 자체를
  막고 어떤 항목끼리 충돌인지 표시, 필수 카테고리 미선택 시 에러 표시.
- **적용은 한 번에 다 하지 않고 `planIndex` 커서로 한 항목씩 진행**
  (`startApply` → `advanceApply`). 이유: 마커 손상을 만나면 그 자리에서
  멈추고 `screenCorrupt`로 전환해 사용자에게 y(백업 복구 후 재시도)/n(이
  항목만 건너뛰기)를 물어야 해서, 전체를 한 함수로 동기 실행할 수 없었음.
- `screenChangelog`: `selfupdate`가 남긴 pending changelog가 있으면 TUI
  시작 화면보다 먼저 표시.
- 카트에서 빠진 카테고리는 적용 시 자동으로 파일에서 구역 제거
  (`finishApply`가 `state.json`과 diff).

### `internal/fetch`
- `http.Get` 대신 10초 타임아웃 있는 `http.Client` 사용, 비-200 응답은
  에러로 취급하도록 수정 (원래는 404 페이지 내용을 그대로 JSON 파싱
  시도해서 에러 메시지가 불친절했음).

---

## 스키마 관련 결정 (중요)

작업 중 실제 데이터 저장소(`raw.githubusercontent.com/dvdsvds/carty-data/main/index.json`)에
네트워크로 접근해보니, **README에 적힌 예시 스키마와 실제 배포된 데이터의
스키마가 다르다는 걸 발견함**:

| 항목 | README 예시 | 실제 데이터 저장소 |
| --- | --- | --- |
| 카테고리 기본 경로 필드 | `target_file_default` | `target_file` |
| 항목 고유 id | (frontmatter에 명시 안 됨, 그러나 depends_on이 "id 목록"이라 서술) | 항목에 `id` 필드 자체가 없음 |

이번 작업에서는 **실제 데이터 저장소 쪽을 기준으로 코드를 맞춤**:
- `Category.TargetFile` (json: `target_file`).
- `Item`에 `ID` 필드를 두지 않고, **`FilePath`를 항목의 고유 키로 사용**
  (`resolve`, `state.Entry.ItemPath`, TUI의 라디오 선택/체크 표시 등 전부
  FilePath 기준). `depends_on`도 다른 항목의 `file_path`를 가리킨다고
  가정함 — 실제 데이터에 `depends_on`이 채워진 예시가 아직 없어서 확정은
  아님.

**README도 실제 스키마에 맞게 갱신함**: "카테고리 메타데이터" 표의
`target_file_env`/`target_file_default` → `target_file_env`/`target_file`
로 수정, "항목 메타데이터" 표의 `depends_on` 설명을 "다른 항목의 id 목록"
→ "다른 항목들의 `file_path` 목록"으로 수정, `state.json` 예시를 실제
필드(`item_path`, `version`, 절대 경로 `target_file`)에 맞게 갱신.

---

## 테스트 (`go test ./...`, 리포에 영구히 남김)

지난 세션에서는 스모크 테스트를 돌려보고 지웠는데, 이번엔 회귀 방지를 위해
정식 `_test.go`로 남김. 전부 `t.TempDir()`만 쓰고 네트워크에 의존하지
않도록 신경 씀 (`internal/tui/apply_test.go`는 아이템의 `file_path`를
`carty-test-fixtures/...`처럼 절대 존재하지 않는 경로로 잡아서, 설령 실제
네트워크가 열려 있어도 fetch가 반드시 실패하고 캐시 폴백 경로를 타도록
함 — 안 그러면 실제 `carty-data` 저장소 콘텐츠가 우연히 테스트 값과
일치/불일치하면서 flaky해짐. 이 실수를 실제로 한 번 겪고 고침, 아래 참고).

- `internal/apply/apply_test.go`: 신규 파일 생성, 기존 콘텐츠 보존,
  구역만 교체, 다른 구역은 안 건드림, 마커 손상 감지, `Remove` 동작
  (있을 때/없을 때), `RestoreFromBackup`.
- `internal/backup/backup_test.go`: 최초 1회만 백업(재적용해도 안 덮임),
  존재하지 않는 파일은 백업 안 함, carty가 새로 만든 파일은 복구 시
  삭제됨.
- `internal/resolve/resolve_test.go`: `depends_on` 자동 추가, 이미 선택된
  의존성 중복 추가 안 함, 순환 의존 에러, 존재하지 않는 의존성 참조 에러,
  같은/다른 대상 파일에서의 충돌 감지·미감지.
- `internal/cache/cache_test.go`: index/개별 파일 저장·조회 라운드트립,
  중첩 경로, 캐시 미스 시 에러.
- `internal/state/state_test.go`: `LoadOrEmpty`, 저장·조회 라운드트립.
- `internal/selfupdate/state_test.go`: `ShouldCheck` 최초/직후 동작,
  pending changelog가 한 번 읽으면 지워지는지.
- `internal/catalog/catalog_test.go`: `ResolveTargetFile`(env 있음/없음),
  `ParseIndex`, `ExtractBody`.
- **`internal/tui/apply_test.go`** (가장 중요): 마커 손상 → `screenCorrupt`
  로 멈춤 → `y` → 백업 복구 후 같은 항목 재시도 → 성공 및 원본 보존까지
  전체 플로우 검증. `n` 입력 시 해당 항목만 실패 처리하고 파일은 안 건드린
  채로 결과 화면으로 넘어가는 것도 별도 테스트로 검증.
- **`internal/appcmd/appcmd_test.go`**: `IndexURL`/`RawBaseURL`을 `const`에서
  `var`로 바꿔서 `httptest.NewServer`로 띄운 가짜 데이터 저장소를 가리키게
  하고, `$HOME`을 `t.TempDir()`로 돌려서(`t.Setenv`) `config.*Path()`가 전부
  임시 디렉터리 안에서 놀도록 함. `Update`가 캐시에 index.json을 저장하는지,
  `Upgrade`가 버전이 바뀐 항목만 다시 fetch/apply하고 `state.json`을
  갱신하는지(이미 최신이면 파일이 전혀 안 바뀌는지), `Uninstall`이 백업으로
  복원 후 `~/.carty`를 지우는지와 `confirm`이 `false`면 아무것도 안
  건드리는지까지 확인. 이 테스트를 짜면서 `Update`가 `cache.SaveIndex`
  (내부에서 `MkdirAll`) 덕에 `~/.carty`가 없어도 스스로 만드는 반면,
  `Upgrade`/`Uninstall`은 `main.go`가 미리 불러주는 `config.EnsureDir()`에
  의존한다는 걸 확인함 — 실제 사용에선 문제없지만(항상 `main()` 진입 시
  한 번 호출됨) 테스트 헬퍼(`withHome`)에서 직접 `EnsureDir()`을 불러줘야
  했음.

## TUI 수동 검증

지난 세션엔 "실제 터미널에서 사람이 조작해본 적 없음"이 미검증 항목으로
남아 있었음. 이번엔 Python의 `pty` 모듈로 가상 터미널을 만들어 실제
빌드된 바이너리를 그 안에서 구동해봄:

- 기동 직후 터미널 배경색 질의(OSC 11)/커서 위치 질의(CPR)에 응답을
  안 해주면 bubbletea가 렌더링 전에 멈춰 있다는 걸 확인함 (더미 응답을
  보내주니 정상 진행).
- 실제 `carty-data`의 `index.json`을 fetch해서 카테고리 목록("Shell (필수)",
  "Neovim Colorscheme")과 장바구니 패널("장바구니 / (비어 있음)"), 하단
  키 안내(`→/enter 진입 · / 검색 · r 새로고침 · a 적용 확인 · q 종료`)가
  기대대로 렌더링되는 걸 raw 출력으로 확인.
- `q` 입력 시 alt-screen/마우스 트래킹 해제 시퀀스까지 정상적으로 나오며
  깨끗하게 종료되는 것 확인.

카테고리 진입/상세 화면/장바구니 담기/적용 확인 화면까지 전부 사람이
눌러본 건 아니라서(자동화된 키 입력 시퀀스로 시작 화면만 확인), 실제
사용 중 레이아웃이 좁은 터미널에서 깨지는지 등은 여전히 미확인.

---

## 릴리즈/설치 파이프라인

- **`scripts/install.sh`** (POSIX sh, `sh -n`/`dash -n` 문법 검증만 함,
  실제 실행은 안 해봄 — GitHub Release가 아직 없어서 실행하면
  404): OS(Linux만 지원)/아키텍처 감지 → `releases/latest`에서 태그 조회 →
  `carty_<os>_<arch>` 바이너리와 `SHA256SUMS` 다운로드 → 체크섬 검증 →
  `~/.carty/bin/carty`에 설치. README가 문서화한
  `curl -fsSL https://carty.sh/install.sh | sh` 한 줄 설치의 실제 스크립트.
  다만 `carty.sh` 도메인에 이 스크립트를 실제로 올리는 건 이번 작업
  범위 밖 (별도 인프라 작업 필요).
- **`.github/workflows/release.yml`**: `v*` 태그 push 시 `go test` →
  `linux/amd64`, `linux/arm64` 빌드(`-ldflags -X main.version=$TAG`로
  `cmd/carty/main.go`의 `version` 변수에 태그 주입) → `SHA256SUMS` 생성 →
  `softprops/action-gh-release`로 GitHub Release 생성 + 에셋 업로드.
  에셋 이름(`carty_linux_amd64` 등)은 `internal/selfupdate.SelectAsset`과
  `scripts/install.sh`가 기대하는 이름과 정확히 맞춤 — 로컬에서
  `GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=..."`로
  똑같이 빌드해서 `--version` 출력이 반영되는지 확인함.
- **`.github/workflows/ci.yml`**: main 브랜치 push/PR마다 build/vet/gofmt/
  test 게이트.

## (지난 세션에) 검증한 것 — 스모크 테스트, 리포에는 안 남겼던 것들

아래는 지난 세션에서 임시로 돌려보고 지운 것들 기록 (지금은 위의
"테스트" 섹션처럼 정식 테스트로 대체됨. 마커 손상 복구 플로우는
`internal/tui/apply_test.go`로, 나머지는 각 패키지의 `_test.go`로
남아 있음):

1. `apply.Apply` → 재적용(구역만 교체) → `apply.Remove` → `backup.Restore`
   순서로 실행해서 각 단계의 파일 내용이 기대대로 나오는지 확인.
2. `resolve.Resolve`: `depends_on` 자동 추가, alias/function 충돌 감지,
   순환 의존 에러 반환 확인.
3. `cache.SaveIndex`/`LoadIndex`, `SaveFile`/`LoadFile` 라운드트립 확인.
4. `selfupdate.SavePendingChangelog`/`LoadPendingChangelog` — 한 번 읽으면
   파일이 지워져서 다음 실행엔 다시 안 뜨는지 확인.
5. **마커 손상 복구 전체 플로우**: 정상 적용 → 사용자가 종료 마커를 수동
   삭제 → `advanceApply`가 `screenCorrupt`로 멈춤 → `updateCorrupt`에 y
   입력 → 백업 복구 후 같은 인덱스 재시도 → 성공적으로 재적용되고 원본
   내용(`export EDITOR=vim`)도 보존되는지 확인. (이 과정에서 테스트
   하네스가 `startApply()`를 두 번 불러 실제 `~/.carty` 경로로 필드가
   리셋되는 테스트 설정 실수를 한 번 겪었음 — 실제 프로덕션 코드 버그는
   아니었고, `advanceApply()`를 직접 부르는 방식으로 테스트를 고쳐서 확인.)

---

## TUI 카드형 레이아웃 + 스크롤

사용자가 "BIOS 셋업 화면처럼" 화면 전체를 채우는 UI를 원해서 전면
재작업함:

- `tea.WithAltScreen()` 켜서 진짜 전체화면 전환 (터미널 스크롤백 안 씀,
  `q` 누르면 원래 화면으로 복귀).
- `internal/tui/view.go` 새로 작성: 상단 헤더 바(현재 화면 이름), 하단
  힌트 바, 본문은 `lipgloss.RoundedBorder()`로 감싼 패널.
- 항목은 `○`/`●` 체크박스 대신 **카드**로 표시 — 이름을 제목으로, 구분선
  아래 설명/색상 스와치/적용 예시 프롬프트/의존성을 한 카드 안에 전부
  표시. 선택 여부는 **테두리 색/굵기**로만 구분 (커서: 분홍 테두리,
  장바구니에 담김: `lipgloss.ThickBorder()` 굵은 분홍 테두리) — 사용자가
  "테두리 색 강조로 가자"고 명시적으로 정한 방향.
- 색상 미리보기는 그냥 색 점 3개가 아니라, 그 팔레트로 만든 가짜 zsh
  프롬프트(`user@host ~/project ❯` 스타일 powerline 청크)를 실제 ANSI
  색으로 렌더링 — 사용자가 "터미널에서 예시로 보여달라"고 요청해서 만든
  `renderPromptPreview`.
- **상세 화면(`screenDetail`)을 완전히 제거**했음 — 처음엔 항목 목록에서
  `→`/`l`을 한 번 더 눌러야 상세 정보가 보였는데, 사용자가 "그렇게 하지
  말고 처음부터 다 보이게 하자"고 해서 목록 화면 자체가 카드 전체를
  보여주도록 바뀌었고, 그러자 상세 화면이 완전히 중복이 돼서 없앴음.
- **스크롤 + 스크롤바**: zsh 테마가 30개로 늘어나면서 화면보다 목록이
  길어져 커서가 화면 밖으로 나가는 실제 버그가 터짐. `itemScroll`/
  `catScroll`을 모델 상태로 추가하고, 커서가 움직일 때마다
  `ensureItemVisible`/`ensureCategoryVisible`(model.go)이 스크롤 오프셋을
  갱신. 처음엔 "▲ 위에 N개 더 있음" 텍스트로 했다가 사용자가 "공식
  프로그램처럼" 만들라고 해서 오른쪽에 진짜 세로 스크롤바(트랙 `│` +
  강조 썸 `┃`, `renderScrollbar`)로 바꿈.
- **버그 2개는 pty의 naive ANSI 스트리핑 검증으로는 못 잡고, `pyte`
  터미널 에뮬레이터를 도입한 뒤에야 발견함** (아래 "TUI 수동 검증" 참고):
  1. `ensureCategoryVisible`이 아이템 카드용 "최소 3줄" 바닥값을 그대로
     복붙해서, 한 줄짜리 카테고리 행에는 안 맞았음 — 작은 터미널에서
     스크롤이 안 먹히고 헤더가 화면 밖으로 밀려남.
  2. `panelBaseOverhead`/`panelCardOverhead` 상수가 실제 `panel()`
     구현(`Height(height-3)` + `Padding(1,2)`)이랑 계산이 1줄 어긋나
     있었고, `renderCartPanel`은 높이 예산을 아예 검사 안 하고 있었음.
     → 상수를 정확히 고치고, 60×12보다 작은 터미널에서는 레이아웃을
     억지로 구겨 넣는 대신 "터미널이 너무 작습니다" 메시지를 보여주는
     최소 크기 가드(`minTermWidth`/`minTermHeight`)를 추가함.
- 회귀 테스트: `internal/tui/scroll_test.go` — 카테고리/아이템 목록 둘 다
  "렌더링 결과가 화면 예산을 절대 안 넘는지"를 실제로 검증하고, 최소
  크기 가드가 뜨는지도 테스트함.

## 데이터 카탈로그 (`carty-data`)

- **`shell` 필수 카테고리를 없앰**: carty는 로그인 쉘(`chsh`/`$SHELL`) 자체는
  안 건드리고 각 카테고리의 `target_file`에만 쓰기 때문에, "쉘을 먼저
  골라야 한다"는 강제 단계가 실질적 의미가 없다고 판단 (사용자와 논의 후
  결정). 기존에 `shell` 카테고리에 있던 기본 zsh 설정은 `zsh-theme`
  카테고리 안의 "기본 (테마 없음)" 항목으로 옮김.
- **zsh 테마 30개** 추가: oh-my-zsh 공식 15개(robbyrussell, agnoster,
  af-magic, avit, bira, dst, gnzh, kennethreitz, kolo, mortalscumbag,
  pygmalion, sunrise, sunaku, wedisagree, ys — 실제 소스 그대로, MIT)
  + 유명 비공식 15개(Dracula/Gruvbox/Catppuccin/Tokyo Night/Rose
  Pine/Everforest/Kanagawa/Solarized/One Dark/Nightfox는 실제 공식
  팔레트로 직접 작성, Powerlevel10k/Pure/Spaceship/Starship/Sorin은
  간단화 버전임을 설명에 명시하고 원본 링크 포함).
- **`fish-theme` 카테고리** 신설, fish 프롬프트 8개(기본, oh-my-zsh
  robbyrussell/agnoster의 fish 포트, Tide/Bob The Fish/Pure/Hydro/
  Spacefish 간단화 버전).
- 작업 중 실제 버그 발견: oh-my-zsh의 `sunrise` 테마가 본문에
  `PROMPTPREFIX="---"`를 리터럴로 갖고 있어서, carty의
  `catalog.ExtractBody`가 `strings.Split(content, "---")`로 **모든**
  `---` 출현에서 쪼개던 기존 구현이 이 파일의 본문을 잘라먹는 버그였음.
  `strings.SplitN(..., 3)`으로 고치고 회귀 테스트 추가.
- 모든 항목이 `resolve.DetectConflicts`/`resolve.Resolve`로 검증됨 (같은
  카테고리 안에서는 한 번에 하나만 선택된다는 실제 제약을 시뮬레이션해서
  충돌 없음 확인).

## `carty-data` CI

README가 원래 "`files/` 안 frontmatter를 CI가 스캔해서 index.json을
자동 생성한다"고 주장했지만 실제로는 지금까지 계속 손으로 고쳐온
상태였음 (모순 발견). 두 옵션(진짜 CI 구현 vs README 문구만 정직하게
수정) 중 사용자가 진짜 CI 구현을 선택함:

- `carty-data/scripts/generate_index.py`: `files/` 재귀 스캔 → 각 파일의
  YAML frontmatter 파싱 → `categories.json`(새로 분리한 카테고리 정의
  소스) 참조해서 `index.json` 생성. 검증: 필수 필드 존재, 카테고리
  존재, `depends_on` 참조가 실제 `file_path`인지, `depends_on` 순환
  의존(DFS) — 하나라도 걸리면 exit 1.
- `.github/workflows/generate-index.yml`: push는 재생성된 index.json을
  자동 커밋(`[skip ci]`로 무한 루프 방지), PR은 index.json이 최신이
  아니면 실패.
- **로컬 검증 + 실제 GitHub Actions 둘 다 확인함**: 생성된 index.json이
  기존 수동 관리본과 (순서/해시 제외) 완전히 동일한지 diff로 확인, 검증
  실패 경로 4가지(모르는 카테고리/순환 의존/끊긴 의존/필드 누락) 전부
  로컬에서 실제로 터뜨려봄, 그리고 진짜 GitHub Actions에서 push 성공 +
  일부러 stale하게 만든 테스트 PR이 정확히 실패하는 것까지 확인하고 그
  테스트 PR/브랜치는 정리함.
- 이 과정에서 버그 하나 더 발견: `themes/nvim/tokyonight.lua`의
  frontmatter가 예전 스키마의 `category: themes/nvim`으로 남아있었음
  (CI가 "존재 안 하는 카테고리"로 정확히 잡아냄) → `nvim-colorscheme`로
  수정.

---

## 커스텀 프롬프트 구성

zsh-theme처럼 완성된 테마 하나를 고르는 게 아니라, 프롬프트를 "항목"
단위로 자유롭게 구성하는 기능. 순서/사용 대수 완전 자유("자유 순서/개수
드래그앤드롭 구성"), 색은 항목마다 개별로 고르게(전역 팔레트 아님) —
둘 다 사용자가 명시적으로 선택한 방향. 터미널이라 진짜 드래그앤드롭은
안 되니, 항목을 고르고 커서로 위/아래 이동시키는 키보드 재배치로 구현.

### 데이터 (`carty-data`)

- 새 카테고리 `zsh-custom` (`apply_method: "compose"`, 타겟
  `~/.zshrc`, `zsh-theme`와는 별도 마커 구역이라 동시에 존재 가능).
  `generate_index.py`는 `apply_method` 값 자체를 검증하지 않아서
  스키마 변경 없이 그대로 통과함.
- 항목 4개(`user-host.zsh`, `path.zsh`, `git-branch.zsh`,
  `prompt-char.zsh`): 각 파일 본문에 실제 색상 hex 대신 `{{color}}`
  플레이스홀더를 넣어두고, carty가 구성 시점에 사용자가 고른 hex로
  치환. 여러 항목이 `PROMPT+="..."` 식으로 이어붙는 걸 전제로 작성함
  (oh-my-zsh 자체의 멀티라인 테마들이 쓰는 관용구와 동일) — 그래서
  어떤 항목을 어떤 순서로 골라도 항상 유효한 zsh 프롬프트가 됨.

### carty 본체 (Go)

- **색상 피커** (`internal/tui/colorpicker.go`): 그림판 스타일 2D
  그리드 — 가로 32칸(색조 0~360°), 세로 12칸(채도, 위 진함→아래
  흐림), 밝기는 0.95 고정. hjkl/방향키로 커서 이동. 커서 마커는 처음
  거의 검은 다이아몬드(◆)로 만들었다가, 밝은 파스텔 칸(`#b8b0f2`,
  `#f29aab`) 위에서 "이 칸이 검게 보인다"는 실사용 피드백을 받고
  얇은 점(●)+배경 밝기 기반 대비색(검/흰 자동 선택, WCAG 상대
  휘도 공식)으로 고침. 따로 실행해볼 수 있는 미니 바이너리도 만듦
  (`cmd/colorpicker-preview`, `go run ./cmd/colorpicker-preview`).
- **구성 화면** (`internal/tui/compose.go`): "compose" 카테고리에
  들어가면 `screenParts`로 진입 — 왼쪽은 사용 가능한 항목 카드
  목록(라디오 아님, 중복 추가 가능), 오른쪽은 구성된 순서. `tab`으로
  두 패널 포커스 전환, 항목 추가 시 즉시 색상 피커(`screenPartColor`)
  로 넘어가서 바로 색을 고르게 함(빈 색으로 남겨두는 상태 자체가
  없음). `J`/`K`로 구성된 순서 재배치, `x`로 제거, `esc`는 "방금
  새로 추가한 항목"이면 삭제하고(취소 의미) 기존 항목 재편집 중이면
  이전 색을 그대로 유지.
- **적용 로직**: `planEntry`에 `Parts []assembledPart` 필드 추가.
  compose 카테고리는 `resolve.Resolve`(라디오 카트) 경로를 안 타고,
  `m.assembled[categoryID]`를 그대로 하나의 plan 항목으로 묶어서
  `composePartsBody`가 `PROMPT=""` 한 줄 + 항목들을 순서대로
  이어붙여 하나의 body로 만들고, 그걸 그대로 기존 `apply.Apply`(마커
  구역 메커니즘)에 넘김 — apply.go 자체는 전혀 안 건드림.

### 검증

- `internal/tui/compose_test.go` (영구 테스트): `{{color}}` 치환 +
  `PROMPT=""` 선두 삽입, 항목 추가 시 색상 피커로 진입하는지, 새
  항목에서 esc하면 삭제되는지, 기존 항목 재편집 중 esc하면 색이
  그대로인지, `J`/`K` 재배치, `x` 제거(+ 목록이 비면 포커스가 자동으로
  항목 목록으로 돌아가는지)까지 커버.
- **pty 검증에서 진짜 아닌 "가짜 버그"를 두 번 겪음** — 둘 다 코드가
  아니라 테스트 하네스 문제였음:
  1. pty child에 `jjj`를 한 번에 여러 키 버스트로 보내면 일부 키가
     드롭됨 (터미널이 아니라 bubbletea의 입력 읽기 타이밍 관련 —
     `colorpicker-preview`처럼 이미 검증된 독립 컴포넌트로도 재현
     됨). 개별 키를 살짝 간격 두고 보내면 문제없음. 이후 pty 테스트는
     전부 개별 키 입력 방식으로 작성.
  2. `raw.githubusercontent.com`의 `main` 브랜치 경로가 CDN 캐시로
     몇 분 지연됨 (`refs/heads/main` 경로는 먼저 갱신됨) — 카테고리
     4개가 안 보인다고 생각했는데 알고 보니 캐시 지연 + 내 디버그
     스크립트가 화면 앞 6줄만 출력해서 4번째 카테고리를 실제로 못
     본 것이었음(이중으로 헷갈림).
- 실제 pty + 개별 키 입력으로 끝까지 재현: 항목 2개 추가(각각 다른
  색 선택) → 재배치(`K`) → 적용 → `.zshrc`에 실제로 쓰인 내용까지
  확인. 두 항목이 각자 고른 색(`#003df2`, `#9af242`)으로 정확히
  치환됐고, 재배치한 순서가 파일에도 그대로 반영됨.

---

## 아직 안 된 것 / 다음에 손댈 것

- **릴리즈 워크플로우가 아직 한 번도 실행된 적 없음**. `.github/workflows/
  release.yml`은 작성/로컬 빌드 검증만 했고, 실제 GitHub Actions에서 태그를
  push해서 릴리즈가 정말 만들어지는지는 확인 못 함 (태그 push는 파괴적/
  공개적 행동이라 사용자 승인 필요 — `carty-data` 쪽 CI는 이번에 실제로
  push/PR까지 다 검증했지만, carty 본체의 릴리즈 태그는 아직 안 건드림).
- 마찬가지로 **`scripts/install.sh`는 실행 검증을 못 함** — 문법 검사
  (`sh -n`)만 했고, 실제로 릴리즈가 없어서 끝까지 돌려보면 404가 남.
  `carty.sh` 도메인에 이 스크립트를 실제로 올리는 것도 별도 작업.
- `depends_on`이 실제로 채워진 데이터가 없어서 자동 추가/순환 검사 로직이
  실제 데이터로는 아직 한 번도 안 돌아가 봄 (테스트의 합성 데이터로만
  검증 — `resolve` 패키지 테스트, `generate_index.py`의 순환 검사 로직은
  검증했지만 실제 항목끼리의 의존관계는 없음). `shell` 카테고리를 없애면서
  유일했던 자연스러운 의존관계(Nord→기본 zsh 설정)도 같이 사라짐 — 지금
  데이터엔 억지로 만들 이유가 있는 의존관계가 없어서 보류 중.
- TUI를 사람이 직접 키보드로 조작해본 적은 없음 — pty + `pyte`로 자동화된
  검증은 이번에 많이 했지만(카드 레이아웃, 스크롤, 선택, 확인 화면, 최소
  크기 가드 등), 실제 사람 손으로 타이핑하면서 느끼는 반응성/타이밍은
  아직 미확인.
- `carty update`/`upgrade`/`uninstall`은 로컬 `httptest` 서버 + 가짜
  `$HOME`으로 통합 테스트까지 했지만(`internal/appcmd/appcmd_test.go`),
  **실제 `carty` 바이너리를 실제 GitHub 릴리즈 대상으로 커맨드라인에서
  돌려본 적은 없음** (릴리즈가 아직 없어서 불가능 — 위 항목 참고).
- **bash 지원 없음** (사용자가 "일단 fish만" 하기로 함, bash는 보류).
- **라이선스가 `TBD`로 비어있음**.
- 이 리포(`carty`)는 이번에 처음 git으로 초기화하고 GitHub에 올라간
  것이라, PR 리뷰/이슈 트래킹 같은 협업 흐름은 아직 한 번도 실제로 안
  써봄.
- **커스텀 프롬프트 구성의 남은 한계**:
  - `resolve.DetectConflicts`가 compose 카테고리의 항목은 아예 검사
    대상에 안 넣음 (각 항목이 `provides.functions`를 선언 안 해서
    지금은 문제없지만, 나중에 항목이 전역 함수를 정의하게 되면 충돌
    검사가 비어있는 채로 지나감).
  - 항목 목록 화면(`viewParts`)엔 스크롤이 없음 — 지금 항목이 4개뿐이라
    문제없지만, 항목이 많아지면 항목 목록에서 고친 것과 같은 문제가
    다시 생길 수 있음.
  - 같은 카테고리 안에서 "완성된 테마"(`zsh-theme`)와 "커스텀 구성"
    (`zsh-custom`)을 동시에 켜면 둘 다 `PROMPT=...`/`PROMPT+=...`를
    건드리므로 서로 간섭할 수 있음 — 지금은 서로 다른 마커 구역이라
    막지 않고, 알려진 한계로만 남겨둠.
  - fish용 구성 기능은 없음 (zsh만).
