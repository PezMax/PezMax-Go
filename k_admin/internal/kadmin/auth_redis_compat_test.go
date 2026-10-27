package kadmin

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAuthRedisGetDelCompatibility(t *testing.T) {
	const key = "synthetic:compatibility:key"
	fallback := []string{"EVAL", redisGetDelScript, "1", key}
	for _, test := range []struct {
		name      string
		responses []string
		commands  [][]string
		want      interface{}
		wantError string
	}{
		{name: "modern Redis uses native GETDEL", responses: []string{"$7\r\nfixture\r\n"}, commands: [][]string{{"GETDEL", key}}, want: "fixture"},
		{name: "older Redis consumes with Lua", responses: []string{"-ERR unknown command `GETDEL`\r\n", "$7\r\nfixture\r\n"}, commands: [][]string{{"GETDEL", key}, fallback}, want: "fixture"},
		{name: "missing legacy key remains nil", responses: []string{"-ERR unknown command 'GETDEL'\r\n", "$-1\r\n"}, commands: [][]string{{"GETDEL", key}, fallback}, wantError: "redis nil"},
		{name: "missing modern key remains nil", responses: []string{"$-1\r\n"}, commands: [][]string{{"GETDEL", key}}, wantError: "redis nil"},
		{name: "permission errors do not trigger fallback", responses: []string{"-NOPERM permission denied\r\n"}, commands: [][]string{{"GETDEL", key}}, wantError: "NOPERM"},
		{name: "wrong types do not trigger fallback", responses: []string{"-WRONGTYPE incompatible value\r\n"}, commands: [][]string{{"GETDEL", key}}, wantError: "WRONGTYPE"},
		{name: "Lua errors remain failures", responses: []string{"-ERR unknown command `GETDEL`\r\n", "-NOPERM scripting denied\r\n"}, commands: [][]string{{"GETDEL", key}, fallback}, wantError: "NOPERM"},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				reader := bufio.NewReader(conn)
				commands := append([][]string{{"AUTH", "synthetic-password"}, {"SELECT", "2"}}, test.commands...)
				responses := append([]string{"+OK\r\n", "+OK\r\n"}, test.responses...)
				for i, expected := range commands {
					actual, err := readCompatibilityCommand(reader)
					if err != nil || !reflect.DeepEqual(actual, expected) {
						done <- fmt.Errorf("unexpected command at step %d: %v", i, err)
						return
					}
					if _, err := conn.Write([]byte(responses[i])); err != nil {
						done <- err
						return
					}
				}
				done <- nil
			}()
			client := &authRedisClient{address: listener.Addr().String(), password: "synthetic-password", db: 2, timeout: 3 * time.Second}
			value, err := client.do("GETDEL", key)
			if serverErr := <-done; serverErr != nil {
				t.Fatal(serverErr)
			}
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("expected error %q, got %v", test.wantError, err)
				}
			} else if err != nil || value != test.want {
				t.Fatalf("value=%v err=%v, want %v", value, err, test.want)
			}
		})
	}
}

func readCompatibilityCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(header, "*") {
		return nil, fmt.Errorf("invalid test command header: %v", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil || count < 1 || count > 20 {
		return nil, errors.New("invalid test command length")
	}
	args := make([]string, count)
	for i := range args {
		value, err := readRedisReply(reader)
		if err != nil {
			return nil, err
		}
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("test command argument is not a string")
		}
		args[i] = text
	}
	return args, nil
}

// Opt-in real Redis check. Only unique, explicitly synthetic, short-lived keys
// are created; no existing accounts, sessions or captcha challenges are read.
func TestAuthRedisSingleUseIntegration(t *testing.T) {
	address := os.Getenv("PEZMAX_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("set PEZMAX_TEST_REDIS_ADDR to test a local isolated Redis")
	}
	client := &authRedisClient{address: address, password: os.Getenv("PEZMAX_TEST_REDIS_PASSWORD"), timeout: 3 * time.Second}
	nonce, err := randomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "pezmax:synthetic-compat-test:" + nonce
	key := prefix + ":consume"
	if _, err := client.do("SET", key, "synthetic-value", "NX", "EX", "60"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = client.do("DEL", key) })
	start := make(chan struct{})
	results := make(chan error, 12)
	var workers sync.WaitGroup
	for i := 0; i < cap(results); i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			value, err := client.do("GETDEL", key)
			if err == nil && value != "synthetic-value" {
				err = errors.New("unexpected synthetic fixture value")
			}
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	consumed := 0
	for err := range results {
		if err == nil {
			consumed++
		} else if !errors.Is(err, errRedisNil) {
			t.Fatal(err)
		}
	}
	if consumed != 1 {
		t.Fatalf("synthetic value consumed %d times; want exactly once", consumed)
	}
	service := &securityService{keyPrefix: prefix, redis: client, secret: []byte("synthetic-compat-secret")}
	for _, wrongFirst := range []bool{false, true} {
		id := fmt.Sprintf("synthetic-challenge-%v", wrongFirst)
		if err := service.storeCaptcha(id, "synthetic-answer", time.Minute); err != nil {
			t.Fatal(err)
		}
		answer := "synthetic-answer"
		if wrongFirst {
			answer = "synthetic-wrong-answer"
		}
		err := service.verifyCaptcha(id, answer)
		if (wrongFirst && !errors.Is(err, errCaptchaInvalid)) || (!wrongFirst && err != nil) {
			t.Fatalf("synthetic challenge verification returned %v", err)
		}
		if !errors.Is(service.verifyCaptcha(id, "synthetic-answer"), errCaptchaInvalid) {
			t.Fatal("synthetic challenge must be consumed after either a correct or wrong answer")
		}
	}
	auth := &authService{redis: client, keyPrefix: prefix}
	if err := auth.storeRefreshToken("synthetic-refresh-token", 1, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if id, err := auth.consumeRefreshToken("synthetic-refresh-token"); err != nil || id != 1 {
		t.Fatalf("synthetic refresh record consumption failed: id=%d err=%v", id, err)
	}
	if _, err := auth.consumeRefreshToken("synthetic-refresh-token"); !errors.Is(err, errInvalidRefreshToken) {
		t.Fatal("synthetic refresh record must not be reusable")
	}
}
