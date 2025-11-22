// client_best.go
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
	// [Best #8] 클라이언트 사이드 로드 밸런싱 (목록)
	serverList  = []string{"localhost:9000"}
	connTimeout = 5 * time.Second
)

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	// 무한 재접속 루프
	for {
		connectAndWork()

		// [Best #6] 연결 끊기면 바로 붙지 않고 Backoff + Jitter 적용
		// 1초 ~ 3초 사이 랜덤 대기 (Thundering Herd 방지)
		backoff := time.Duration(1000+rand.Intn(2000)) * time.Millisecond
		fmt.Printf("Waiting %v before reconnect...\n", backoff)
		time.Sleep(backoff)
	}
}

func connectAndWork() {
	// [Best #8] 서버 목록 중 랜덤 선택 (Simple LB)
	target := serverList[rand.Intn(len(serverList))]

	// [Best #11] DNS 갱신 (ResolveTCPAddr를 매번 호출)
	// IP가 바뀌었을 경우를 대비해 연결 시마다 다시 해석함
	tcpAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		fmt.Printf("DNS Resolve failed: %v\n", err)
		return
	}

	// [Best #4] 연결 타임아웃 설정
	conn, err := net.DialTimeout("tcp", tcpAddr.String(), connTimeout)
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("✅ Connected to %s\n", target)

	// [Best #5] 하트비트 루프 (별도 고루틴)
	// 서버가 죽었는지 살았는지 능동적으로 체크
	stopHeartbeat := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// PING 전송
				conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				_, err := fmt.Fprintf(conn, "PING\n")
				if err != nil {
					return // 에러 나면 루프 종료
				}
			case <-stopHeartbeat:
				return
			}
		}
	}()

	// 메인 작업 루프 (데이터 수신)
	reader := bufio.NewReader(conn)
	for {
		// 서버 응답 대기 (여기서도 ReadDeadline 필요하면 설정)
		conn.SetReadDeadline(time.Now().Add(65 * time.Second)) // 서버 하트비트(60s) 고려
		msg, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Disconnected: %v\n", err)
			close(stopHeartbeat) // 하트비트 중단
			return               // 함수 리턴 -> 재접속 대기(Backoff)로 이동
		}

		// PONG 응답은 로그만 찍고 무시
		if msg == "PONG\n" {
			// fmt.Println("Received PONG")
			continue
		}

		fmt.Print("Server: " + msg)
	}
}
