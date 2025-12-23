package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type smsReq struct {
	CustomerID int64    `json:"customer_id"`
	Text       string   `json:"text"`
	Recipients []string `json:"recipients"`
	Type       string   `json:"type"` // normal|express
}

type addBalanceReq struct {
	UserID      int64  `json:"user_id"`
	Balance     uint64 `json:"balance"`
	Description string `json:"description,omitempty"`
}

type result struct {
	d    time.Duration
	err  error
	code int
}

func main() {
	var (
		baseURL      = flag.String("base-url", "http://localhost:8080", "API base URL")
		rps          = flag.Int("rps", 200, "requests per second")
		duration     = flag.Duration("duration", 30*time.Second, "test duration (0 to skip traffic)")
		concurrency  = flag.Int("concurrency", 100, "number of worker goroutines")
		userStart    = flag.Int64("user-start", 1, "starting customer_id")
		users        = flag.Int64("users", 1000, "number of distinct users to rotate through")
		recipients   = flag.Int("recipients", 1, "recipients per request")
		expressRatio = flag.Float64("express-ratio", 0.1, "fraction of requests as express (0..1)")
		timeout      = flag.Duration("timeout", 3*time.Second, "HTTP client timeout")

		seedOnly        = flag.Bool("seed-only", false, "run seed phase only, then exit")
		seedBalance     = flag.Uint64("seed-balance", 0, "if >0, pre-add this balance for each user before sending traffic")
		seedConcurrency = flag.Int("seed-concurrency", 50, "parallelism for seeding balances")
		seedTimeout     = flag.Duration("seed-timeout", 2*time.Minute, "timeout for the balance seeding phase")
		seedDesc        = flag.String("seed-desc", "loadtest seed", "description used when seeding balances")

		// Fast seeding: bulk upsert directly into MySQL (no HTTP, no per-user tx).
		seedMethod = flag.String("seed-method", "db", "seed method: db|http")
		seedDBDSN  = flag.String("seed-db-dsn", "", "MySQL DSN used when seed-method=db (optional). If empty, built from DB_* env vars.")
	)
	flag.Parse()

	if *users <= 0 {
		panic("invalid args: users must be > 0")
	}
	if *seedOnly {
		if *seedBalance == 0 {
			panic("invalid args: -seed-only requires -seed-balance > 0")
		}
	} else {
		if *duration <= 0 {
			panic("invalid args: duration must be > 0 (or use -seed-only)")
		}
		if *rps <= 0 || *concurrency <= 0 || *recipients <= 0 {
			panic("invalid args")
		}
	}

	endpoint := *baseURL + "/sms/send"
	balanceEndpoint := *baseURL + "/balance/add"
	client := &http.Client{Timeout: *timeout}

	if *seedBalance > 0 {
		seedCtx, seedCancel := context.WithTimeout(context.Background(), *seedTimeout)
		defer seedCancel()
		switch *seedMethod {
		case "db":
			dsn := *seedDBDSN
			if dsn == "" {
				dsn = buildDSNFromEnv()
			}
			fmt.Printf("seeding balances (db): users=%d amount=%d\n", *users, *seedBalance)
			if err := seedBalancesDB(seedCtx, dsn, *userStart, *users, *seedBalance); err != nil {
				panic(fmt.Sprintf("seed balances (db) failed: %v", err))
			}
			fmt.Println("seeding balances: done")
		case "http":
			fmt.Printf("seeding balances (http): users=%d amount=%d endpoint=%s\n", *users, *seedBalance, balanceEndpoint)
			if err := seedBalances(seedCtx, client, balanceEndpoint, *userStart, *users, *seedBalance, *seedDesc, *seedConcurrency); err != nil {
				panic(fmt.Sprintf("seed balances (http) failed: %v", err))
			}
			fmt.Println("seeding balances: done")
		default:
			panic("invalid -seed-method (db|http)")
		}
	}

	if *seedOnly {
		return
	}

	// Start traffic timer AFTER seeding, so duration applies to the actual load phase.
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// token bucket by ticker
	tokens := make(chan struct{}, *rps)
	ticker := time.NewTicker(time.Second / time.Duration(*rps))
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(tokens)
				return
			case <-ticker.C:
				select {
				case tokens <- struct{}{}:
				default:
					// if channel is full, drop token (backpressure)
				}
			}
		}
	}()

	results := make(chan result, *rps)
	var sent uint64
	var ok uint64
	var httpErr uint64
	var bad uint64

	var wg sync.WaitGroup
	wg.Add(*concurrency)
	for i := 0; i < *concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			for range tokens {
				reqBody := smsReq{
					CustomerID: *userStart + (rng.Int63() % *users),
					Text:       "hello",
					Recipients: makeRecipients(*recipients),
					Type:       "normal",
				}
				if rng.Float64() < *expressRatio {
					reqBody.Type = "express"
				}
				b, _ := json.Marshal(reqBody)

				start := time.Now()
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				d := time.Since(start)
				atomic.AddUint64(&sent, 1)
				if err != nil {
					atomic.AddUint64(&httpErr, 1)
					results <- result{d: d, err: err}
					continue
				}
				_, _ = io.ReadAll(resp.Body)
				_ = resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					atomic.AddUint64(&ok, 1)
				} else {
					atomic.AddUint64(&bad, 1)
				}
				results <- result{d: d, code: resp.StatusCode}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	latencies := make([]time.Duration, 0, *rps*int(duration.Seconds()))
	startAll := time.Now()
	for r := range results {
		latencies = append(latencies, r.d)
	}
	elapsed := time.Since(startAll)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	p := func(q float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		idx := int(float64(len(latencies)-1) * q)
		return latencies[idx]
	}

	total := atomic.LoadUint64(&sent)
	fmt.Printf("endpoint=%s\n", endpoint)
	fmt.Printf("sent=%d ok=%d non2xx=%d http_err=%d elapsed=%s achieved_rps=%.1f\n",
		total,
		atomic.LoadUint64(&ok),
		atomic.LoadUint64(&bad),
		atomic.LoadUint64(&httpErr),
		elapsed,
		float64(total)/elapsed.Seconds(),
	)
	fmt.Printf("latency p50=%s p90=%s p95=%s p99=%s max=%s\n",
		p(0.50), p(0.90), p(0.95), p(0.99),
		func() time.Duration {
			if len(latencies) == 0 {
				return 0
			}
			return latencies[len(latencies)-1]
		}(),
	)
}

func makeRecipients(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("+98912%06d", i))
	}
	return out
}

func seedBalances(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	userStart int64,
	users int64,
	amount uint64,
	description string,
	concurrency int,
) error {
	if concurrency <= 0 {
		concurrency = 1
	}

	jobs := make(chan int64, concurrency*2)
	var wg sync.WaitGroup

	var seeded uint64
	var failed uint64

	worker := func() {
		defer wg.Done()
		for userID := range jobs {
			select {
			case <-ctx.Done():
				return
			default:
			}

			ok := false
			for attempt := 0; attempt < 6; attempt++ {
				if attempt > 0 {
					time.Sleep(time.Duration(30*(1<<attempt)) * time.Millisecond)
				}
				body := addBalanceReq{UserID: userID, Balance: amount, Description: description}
				b, _ := json.Marshal(body)
				req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
				req.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(req)
				if err != nil {
					continue
				}
				_, _ = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					ok = true
					break
				}
				// retry on server-side errors (includes deadlock)
				if resp.StatusCode < 500 {
					break
				}
			}
			if ok {
				atomic.AddUint64(&seeded, 1)
			} else {
				atomic.AddUint64(&failed, 1)
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go worker()
	}

	// progress printer
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s := atomic.LoadUint64(&seeded)
				f := atomic.LoadUint64(&failed)
				fmt.Printf("seed progress: ok=%d failed=%d/%d\r", s, f, users)
			}
		}
	}()

	for i := int64(0); i < users; i++ {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			<-doneCh
			return ctx.Err()
		case jobs <- userStart + i:
		}
	}
	close(jobs)
	wg.Wait()
	<-doneCh

	if atomic.LoadUint64(&failed) > 0 {
		return fmt.Errorf("seed failures: %d", atomic.LoadUint64(&failed))
	}
	return nil
}

// DB seeding: bulk upsert balances for a contiguous user range.
// This is orders of magnitude faster than issuing N HTTP calls and avoids creating user_transactions rows.
func seedBalancesDB(ctx context.Context, dsn string, userStart int64, users int64, amount uint64) error {
	if users <= 0 || amount == 0 {
		return nil
	}
	if userStart <= 0 {
		return fmt.Errorf("user-start must be > 0")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(2 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const chunk = 2000
	for offset := int64(0); offset < users; offset += chunk {
		n := int64(chunk)
		if users-offset < n {
			n = users - offset
		}

		placeholders := make([]string, 0, n)
		args := make([]any, 0, n*2)
		for i := int64(0); i < n; i++ {
			placeholders = append(placeholders, "(?,?)")
			args = append(args, userStart+offset+i, amount)
		}
		q := "INSERT INTO user_balances (user_id, balance) VALUES " + strings.Join(placeholders, ",") +
			" ON DUPLICATE KEY UPDATE balance = balance + VALUES(balance), last_updated = CURRENT_TIMESTAMP"

		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func buildDSNFromEnv() string {
	user := getenvDefault("DB_USER_NAME", "sms_user")
	pass := getenvDefault("DB_PASSWORD", "sms_pass")
	host := getenvDefault("DB_HOST", "localhost")
	port := getenvDefault("DB_PORT", "3306")
	db := getenvDefault("DB_NAME", "sms_gateway")
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		user, pass, host, port, db,
	)
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
