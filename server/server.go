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
	"os"
	"os/signal"
	"runtime/trace"
	"strings"
	"syscall"
	"time"
)

// [Best #14] 설정 유연성: 하드코딩 피하고 플래그/환경변수로 주입
var (
	listenPort     = flag.String("port", "9000", "Service Port")
	adminPort      = flag.String("admin", "9001", "Admin Port")
	maxConnections = flag.Int("max-conn", 1000, "Max concurrent connections") // [Best #7] 연결 수 제한 설정
	readTimeout    = 60 * time.Second                                         // [Best #4] 읽기 타임아웃 (좀비 방지)
	writeTimeout   = 5 * time.Second                                          // [Best #4] 쓰기 타임아웃 (블로킹 방지)
	maxConnAge     = 1 * time.Hour                                            // [Best #10] 연결 TTL (로드밸런싱 리밸런싱 유도)
)

// [Best #16] 가시성 확보: expvar를 통한 실시간 메트릭 노출 (/debug/vars)
var (
	currentConns = expvar.NewInt("tcp_current_connections")
	totalConns   = expvar.NewInt("tcp_total_connections")
	timeoutErrs  = expvar.NewInt("tcp_timeout_errors")
)

func main() {
	flag.Parse()

	// [Trace] 성능 분석을 위한 추적 시작
	f, err := os.Create("trace.out")
	if err != nil {
		log.Fatalf("failed to create trace output: %v", err)
	}
	defer f.Close()
	if err := trace.Start(f); err != nil {
		log.Fatalf("failed to start trace: %v", err)
	}
	defer trace.Stop()

	// [Best #15] 우아한 종료 (Graceful Shutdown): 시그널 핸들링
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\n🛑 Shutting down... saving trace.")
		trace.Stop() // 추적 데이터 저장 보장
		f.Close()
		os.Exit(0)
	}()

	// [Best #3] 관리 포트 분리: 서비스 포트가 막혀도 모니터링 가능하도록 함
	go startAdminServer(*adminPort)

	// [Best #1, #2] 포트 바인딩: Go는 SO_REUSEADDR 기본 적용, 포트 규격 준수
	ln, err := net.Listen("tcp", ":"+*listenPort)
	if err != nil {
		log.Fatalf("Failed to bind: %v", err)
	}
	defer ln.Close()

	log.Printf("🛡️ Server on :%s (MaxConn: %d)", *listenPort, *maxConnections)

	// [Best #7] 과부하 방지 (Backpressure): 세마포어 패턴 사용
	sem := make(chan struct{}, *maxConnections)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		// 연결 수락 전 용량 체크 (Non-blocking)
		select {
		case sem <- struct{}{}:
			// 슬롯 확보 성공 -> 고루틴 실행
			currentConns.Add(1)
			totalConns.Add(1)
			go func() {
				handleConnection(conn)
				<-sem // 작업 완료 후 슬롯 반납
				currentConns.Add(-1)
			}()
		default:
			// [Best #7] Fail Fast: 용량 초과 시 대기 없이 즉시 거절
			conn.Close()
		}
	}
}

func handleConnection(conn net.Conn) {
	// [Best #9] 자원 해제 보장: 함수 종료 시 소켓 닫기 (CLOSE_WAIT 방지)
	defer conn.Close()

	// [Best #10] 장기 연결 강제 종료 (TTL): 한 서버에 연결 고착화 방지
	ctx, cancel := context.WithTimeout(context.Background(), maxConnAge)
	defer cancel()

	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			conn.SetReadDeadline(time.Now()) // 강제로 IO 에러 유발하여 연결 끊기
		}
	}()

	// [Best #12, #13] 버퍼링 및 튜닝: 시스템 콜 감소 (Go가 내부 버퍼 자동 최적화)
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// [Best #4] 타임아웃 설정: 좀비 커넥션 및 Slowloris 공격 방어
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					timeoutErrs.Add(1)
				}
			}
			return
		}

		line = strings.TrimSpace(line)

		// [Best #5] 애플리케이션 하트비트: TCP Keepalive 외에 실제 서비스 생존 확인
		if line == "PING" {
			conn.SetWriteDeadline(time.Now().Add(writeTimeout)) // [Best #4] 쓰기 데드라인
			writer.WriteString("PONG\n")
			writer.Flush()
			continue
		}

		// 비즈니스 로직
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		writer.WriteString("ECHO: " + line + "\n")
		writer.Flush()
	}
}

func startAdminServer(port string) {
	// [Best #16] 모니터링 엔드포인트 제공
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	log.Printf("🚑 Admin Server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
