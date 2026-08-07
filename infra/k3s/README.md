# Chinwag on k3s

Kubernetes(k3s) 기반 배포 매니페스트입니다. 기존 애플리케이션 코드는 수정하지 않고,
`backend/Dockerfile`, `frontend/Dockerfile`, `infra/k3s/*.yaml` 만으로 배포합니다.

## 구성 요소

| 컴포넌트 | 종류 | 포트 | 설명 |
|---|---|---|---|
| `postgres` | StatefulSet | 5432 | 4개 스키마(`chinwag_auth`, `chinwag_room`, `chinwag_chat`, `chinwag_chat_projection`) 생성, `local-path` PVC |
| `redis` | Deployment | 6379 | AOF 영속화, Secret 비밀번호 |
| `nats` | StatefulSet | 4222 | JetStream 단일 노드, `local-path` PVC (`CHAT_EVENTS` 스트림은 chat-command가 자동 생성) |
| `gateway` | Deployment (2) | 8000 | Echo 리버스 프록시 |
| `auth` | Deployment | 8081 | 인증/JWKS/OAuth |
| `room` | Deployment | 8082 | 룸/멤버/초대 |
| `chat-command` | Deployment | 8083 | 메시지 쓰기 + WebSocket |
| `chat-query` | Deployment | 8084 | 메시지 조회(projection) |
| `frontend` | Deployment (2) | 3000 | Next.js standalone 서버 |
| `chinwag` Ingress | Traefik | 80 | `/auth /rooms /users /chat` → gateway, `/` → frontend |

모든 리소스는 `chinwag` Namespace에 배치됩니다.

## 사전 요구사항

- 로컬 k3s 클러스터 (`curl -sfL https://get.k3s.io | sh -`)
- Docker (`docker build` / `docker save`용)
- `kubectl` (k3s 설치 시 함께 제공)

## 0. 원클릭 배포 (권장)

빌드 → k3s 로드 → apply → 롤아웃 대기 → 헬스 체크를 한 번에 수행합니다.

```bash
cd infra/k3s
./deploy.sh                  # 전체 배포
./deploy.sh --no-build       # 이미지 빌드/로드 생략 (이미 로드된 이미지 사용)
./deploy.sh --apply          # 매니페스트만 적용 (검증 생략)
```

전제: `secret.yaml`이 디스크에 있어야 합니다 (`cp secret.yaml.example secret.yaml` 후 값 입력).

## 1. 이미지 빌드 및 로드

k3s는 containerd를 사용하므로 `docker build`한 이미지를 k3s 런타임으로 import해야 합니다.
Docker와 k3s가 같은 호스트에 있으면 `--load` 옵션으로 한 번에 처리됩니다.

```bash
# 이미지 빌드 + k3s로 import
cd infra/k3s
./build-images.sh --load

# (이미지만 빌드)
./build-images.sh
# (k3s로 import만)
./load-images.sh
```

> WebSocket 주소는 브라우저가 현재 접속한 origin에서 자동 파생됩니다
> (`src/services/websocket-client.ts`, Ingress가 `/chat`을 gateway로 라우팅).
> 별도의 WS 주소 설정이나 빌드 인자는 필요하지 않습니다.

## 2. 적용 (apply)

```bash
cd infra/k3s

# 권장: kustomize 일괄 적용 (리소스 순서 자동 처리)
kubectl apply -k .

# 또는 개별 파일 순서대로
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml -f secret.yaml
kubectl apply -f postgres.yaml -f redis.yaml -f nats.yaml
kubectl apply -f gateway.yaml -f auth.yaml -f room.yaml
kubectl apply -f chat-command.yaml -f chat-query.yaml -f frontend.yaml
kubectl apply -f ingress.yaml
```

## 3. 접속 호스트 설정

Ingress의 host는 `chinwag.local`로 되어 있습니다. `/etc/hosts`에 k3s 노드 IP를 등록합니다.

```bash
# k3s가 실행 중인 노드의 IP로 교체
echo "<k3s-node-ip> chinwag.local" | sudo tee -a /etc/hosts
```

- 웹 UI: http://chinwag.local
- 게이트웨이 health: http://chinwag.local/health (또는 내부적으로 `gateway:8000/health`)
- Swagger UI: `http://chinwag.local/auth/docs`, `http://chinwag.local/rooms/docs`, `http://chinwag.local/chat/docs`

> 다른 호스트명을 쓰려면 `ingress.yaml`의 `host`와 `configmap.yaml`의 `FRONTEND_URL`을
> 함께 바꾸면 됩니다. WebSocket은 브라우저 origin을 그대로 따르므로 별도 설정이 필요 없습니다.

## 4. 검증

```bash
# 파드 상태 확인 (모두 Running/Ready)
kubectl -n chinwag get pods

# 로그 확인
kubectl -n chinwag logs -l app=auth

# 게이트웨이를 통해 각 서비스 health 확인
curl -s http://chinwag.local/health                      # gateway
curl -s http://chinwag.local/auth/health                 # auth
curl -s http://chinwag.local/rooms/health                # room
curl -s http://chinwag.local/chat/health                 # chat-command / chat-query

# 내부 DNS/SVC 검증 (파드 안에서)
kubectl -n chinwag exec deploy/gateway -- wget -qO- http://auth:8081/health

# 이벤트 확인
kubectl -n chinwag get events --sort-by=.lastTimestamp
```

회원가입/로그인 → 룸 생성 → 메시지 전송(WebSocket 실시간 수신) 흐름으로 종단 검증합니다.

## 5. 환경변수/시크릿 변경

| 파일 | 내용 |
|---|---|
| `configmap.yaml` | 비밀 아닌 공통 설정 (포트, 서비스 URL, `FRONTEND_URL`, `NATS_URL` 등) |
| `secret.yaml` | DB 비밀번호, Redis 비밀번호, Google OAuth (로컬 개발용 기본값 포함) |

Secret 교체 후 재배포:

```bash
kubectl -n chinwag create secret generic chinwag-secrets \
  --from-literal=POSTGRES_PASSWORD='<new>' \
  --from-literal=REDIS_PASSWORD='<new>' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n chinwag rollout restart deploy auth room chat-command chat-query redis
```

> ⚠️ Google OAuth를 켜려면 `secret.yaml`의 `GOOGLE_CLIENT_ID/SECRET/REDIRECT_URL`을
> 설정하고, `GOOGLE_REDIRECT_URL`이 `FRONTEND_URL`과 일치하는지 확인하세요.

## 6. 삭제

```bash
kubectl delete -k infra/k3s
```

PVC(`local-path`) 데이터는 삭제 후에도 남습니다. 완전 삭제:

```bash
kubectl -n chinwag delete pvc --all
```

## 7. 이미지 태그 변경

`image: chinwag/<service>:latest` 태그를 원하는 버전으로 바꾸려면 각 deployment 파일의
`image` 필드와 `build-images.sh`의 태그를 함께 수정하세요. 사설 레지스트리를 쓰는 경우
`imagePullPolicy: IfNotPresent`를 `Always`로 바꾸고 레지스트리 주소를 포함한 태그를 사용합니다.

## 8. 스케일링 참고

- `gateway`, `frontend`는 무상태라 복제본을 늘려도 안전합니다.
- `auth`, `room`, `chat-command`, `chat-query`는 **시작 시 DB 마이그레이션을 실행**하므로
  복제본을 1로 유지합니다(마이그레이션 레이스 방지). `chat-command`는 파드당 고정된
  JetStream durable consumer(`chat-ws-<pod>`)를 사용하므로 파드 재시작 시에도 유실 없이
  이어받습니다.
