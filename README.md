# TcpSocketPro

**TcpSocketPro**는 Go 언어로 구현한 견고하고 확장 가능한 TCP 소켓 서버/클라이언트 예제 프로젝트입니다.
Youzan 미들웨어 팀의 **TCP 네트워크 프로그래밍 모범 사례 16가지**를 기반으로 설계되었으며, 실제 운영 환경에서 발생할 수 있는 네트워크 이슈(유령 연결, 블로킹, 성능 저하 등)를 방지하기 위한 기법들이 적용되어 있습니다.

## 주요 특징 (Key Features)

이 프로젝트는 단순한 연결 예제를 넘어 다음과 같은 안정성 확보 기술을 포함합니다.

* **동시성 처리 (Concurrency)**
    * Go 루틴(Goroutine)을 활용하여 각 클라이언트 연결을 독립적으로 비동기 처리합니다.
* **타임아웃 관리 (Timeouts & Deadlines)**
    * `SetReadDeadline` (60초): 클라이언트로부터 데이터가 오지 않을 경우 연결을 자동 종료하여 리소스 누수를 방지합니다.
    * `SetWriteDeadline` (5초): 쓰기 작업 지연 시 블로킹을 방지합니다.
* **I/O 버퍼링 (Buffered I/O)**
    * `bufio` 패키지를 사용하여 시스템 호출(System Call) 횟수를 줄이고 네트워크 입출력 성능을 최적화했습니다.
* **애플리케이션 하트비트 (Application Heartbeat)**
    * TCP Keepalive에만 의존하지 않고, 애플리케이션 계층에서 `PING` - `PONG` 메커니즘을 통해 연결의 유효성을 능동적으로 검사합니다.
* **우아한 종료 (Graceful Resource Cleanup)**
    * `defer` 키워드를 사용하여 연결 종료 시 소켓 리소스를 확실하게 반환합니다.

## 설치 및 실행 (Getting Started)

### 전제 조건 (Prerequisites)
* Go 1.18 이상

### 프로젝트 구조
```bash
TcpSocketPro/
├── client/       # 클라이언트 구현체 (예정)
├── server/       # 서버 구현체
│   └── server.go
├── .gitignore    # 빌드 아티팩트 무시 설정
└── README.md     # 프로젝트 문서
```

실행 방법 (Usage)

1. 서버 실행 서버는 기본적으로 9000번 포트에서 수신 대기합니다.
   
```bash
go run server/server.go
```

2. 클라이언트 테스트 별도의 클라이언트 코드를 실행하거나, telnet 또는 nc 명령어로 테스트할 수 있습니다.
   
```bash
# 텔넷으로 접속
telnet localhost 9000

# PING 입력 시 PONG 응답 확인
Trying 127.0.0.1...
Connected to localhost.
Escape character is '^]'.
PING
PONG

# 일반 메시지 입력 시 Echo 확인
Hello World
ECHO:Hello World
```

3. server/server.go 상단에서 주요 설정을 변경할 수 있습니다.

| 상수명 | 기본값 | 설명 |
| :--- | :--- | :--- |
| `ListenPort` | `:9000` | 서버 리스닝 포트 |
| `ReadTimeout` | `60 * time.Second` | 읽기 작업 타임아웃 (하트비트 주기보다 길어야 함) |
| `WriteTimeout` | `5 * time.Second` | 쓰기 작업 타임아웃 |
| `MaxBufferBytes`| `4096` | 버퍼 크기 (byte) |


