package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"time"
)

var (
	// [Best #8] 클라이언트 로드 밸런싱: 여러 서버 목록 관리
	serverList  = []string{"localhost:9000"}
	connTimeout = 5 * time.Second
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	for {
		connectAndWork()

		// [Best #6] 지터(Jitter)를 포함한 백오프: 동시 재접속 폭주(Thundering Herd) 방지
		backoff := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
		fmt.Printf("Waiting %v before reconnect...\n", backoff)
		time.Sleep(backoff)
	}
}

func connectAndWork() {
	// [Best #8] 접속 대상 랜덤 선택 (Simple LB)
	target := serverList[rand.Intn(len(serverList))]

	// [Best #11] DNS Late Binding: 연결 시마다 IP 재해석 (클라우드 환경 필수)
	tcpAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		fmt.Printf("DNS Resolve failed: %v\n", err)
		return
	}

	// [Best #4] 연결 타임아웃 설정: 무한 대기 방지
	conn, err := net.DialTimeout("tcp", tcpAddr.String(), connTimeout)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	defer conn.Close() // [Best #9] 자원 정리

	fmt.Printf("✅ Connected to %s\n", target)

	// [Best #5] 능동적 하트비트: 서버 상태 주기적 체크 (별도 고루틴)
	stopHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				_, err := fmt.Fprintf(conn, "PING\n")
				if err != nil {
					return
				}
			case <-stopHeartbeat:
				return
			}
		}
	}()

	reader := bufio.NewReader(conn)
	for {
		// [Best #4] 서버 응답 대기 타임아웃 (서버 하트비트 주기보다 약간 길게 설정)
		conn.SetReadDeadline(time.Now().Add(65 * time.Second))

		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Disconnected: %v\n", err)
			close(stopHeartbeat) // 하트비트 루프 종료
			return
		}

		if msg == "PONG\n" {
			continue
		}

		fmt.Print("Server: " + msg)
	}
}
