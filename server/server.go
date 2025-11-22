// server_best.go
// TCP Network Programming Best Practices 1~16 Applied

package main

import (
	"bufio"
	"context"
	"expvar"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// [Best #14] 설정 유연성: 모든 설정값은 플래그나 상수로 관리
var (
	listenPort     = flag.String("port", "9000", "Service Port")
	adminPort      = flag.String("admin", "9001", "Admin Port")
	maxConnections = flag.Int("max-conn", 1000, "Max concurrent connections") // [Best #7] 최대 연결 수 제한
	readTimeout    = 60 * time.Second                                         // [Best #4] 읽기 타임아웃 (좀비 방지)
	writeTimeout   = 5 * time.Second                                          // [Best #4] 쓰기 타임아웃 (블로킹 방지)
	maxConnAge     = 1 * time.Hour                                            // [Best #10] 장기 연결 TTL (Rebalancing 유도)                                     // 유휴 연결 정리 시간
)

// [Best #16] 모니터링 지표 (expvar 사용 -> /debug/vars 자동 노출)
var (
	currentConns = expvar.NewInt("tcp_current_connections")
	totalConns   = expvar.NewInt("tcp_total_connections")
	timeoutErrs  = expvar.NewInt("tcp_timeout_errors")
)

func main() {
	flag.Parse()

	// [Best #3] 관리 포트 분리
	go startAdminServer(*adminPort)

	// [Best #2] 포트 규격 준수 (9000번 사용)
	// [Best #1] SO_REUSEADDR은 Go net 패키지 기본 적용됨
	ln, err := net.Listen("tcp", ":"+*listenPort)
	if err != nil {
		log.Fatalf("Failed to bind: %v", err)
	}
	defer ln.Close()

	log.Printf("🛡️ Best Server listening on :%s (MaxConn: %d)", *listenPort, *maxConnections)

	// [Best #7] 연결 제한을 위한 세마포어 채널
	sem := make(chan struct{}, *maxConnections)

	for {
		// 연결 수락
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(10 * time.Millisecond) // 일시적 에러 시 백오프
				continue
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		// [Best #7] 최대 연결 수 체크 (Non-blocking)
		select {
		case sem <- struct{}{}:
			// 슬롯 확보 성공 -> 처리
			currentConns.Add(1)
			totalConns.Add(1)
			go func() {
				handleConnection(conn)
				<-sem // 처리 완료 후 슬롯 반납
				currentConns.Add(-1)
			}()
		default:
			// [Best #7] 연결 초과 시 즉시 거절 (Overload 방지)
			log.Printf("Connection rejected: Server full")
			conn.Close()
		}
	}
}

func handleConnection(conn net.Conn) {
	// [Best #9] 자원 해제 보장 (CLOSE_WAIT 방지)
	defer conn.Close()

	// [Best #10] 장기 연결 강제 종료 (TTL) 타이머
	// 1시간 지나면 무조건 끊어서 클라이언트가 다시 로드밸런싱되게 함
	ctx, cancel := context.WithTimeout(context.Background(), maxConnAge)
	defer cancel()

	// TTL 만료 시 소켓 닫는 고루틴
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			// log.Println("Connection closed due to Max TTL")
			conn.SetReadDeadline(time.Now()) // 강제로 Read 에러 유발하여 종료
		}
	}()

	// [Best #12] 버퍼링 사용 (시스템 콜 감소)
	// [Best #13] TCP 버퍼는 Go가 자동으로 BDP에 맞춰 최적화함 (수동 설정 불필요)
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// remoteAddr := conn.RemoteAddr().String()
	// log.Printf("Accepted: %s", remoteAddr)

	for {
		// [Best #4] 타임아웃 설정 (Deadlines)
		// 클라이언트가 60초 동안 아무 말 없으면 연결 끊음 (좀비 킬러)
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		// 데이터 읽기
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				// 타임아웃 에러인지 확인
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					timeoutErrs.Add(1)
					// log.Printf("Timeout from %s", remoteAddr)
				}
			}
			return // 루프 탈출 -> defer conn.Close() 실행
		}

		line = strings.TrimSpace(line)

		// [Best #5] 애플리케이션 하트비트 처리
		if line == "PING" {
			conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			writer.WriteString("PONG\n")
			writer.Flush()
			continue
		}

		// 비즈니스 로직 (Echo)
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		writer.WriteString("ECHO: " + line + "\n")
		writer.Flush()
	}
}

func startAdminServer(port string) {
	// [Best #16] 모니터링 엔드포인트 제공
	// /debug/vars 접속 시 JSON 메트릭 반환
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	log.Printf("🚑 Admin Server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
