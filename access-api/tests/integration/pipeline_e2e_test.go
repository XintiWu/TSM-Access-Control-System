//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestPipelineE2E(t *testing.T) {
	if os.Getenv("E2E_PIPELINE") != "1" {
		t.Skip("set E2E_PIPELINE=1 and run 'make up' first")
	}

	apiURL := envOr("API_URL", "http://localhost:8080")
	dsn := envOr("DB_DSN", "access:access@tcp(127.0.0.1:3307)/access_control?parseTime=true")
	userID := envOr("DEMO_USER", "22222222-2222-2222-2222-222222222222")
	doorID := envOr("DEMO_DOOR", "11111111-1111-1111-1111-111111111111")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("access-api not reachable at %s: %v (run make up)", apiURL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health check status %d", resp.StatusCode)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("mariadb not reachable: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"userId":    userID,
		"doorId":    doorID,
		"direction": "IN",
		"cardUid":   "CARD001",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	swipeCtx, swipeCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer swipeCancel()
	swipeReq, err := http.NewRequestWithContext(swipeCtx, http.MethodPost, apiURL+"/access/swipe", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	swipeReq.Header.Set("Content-Type", "application/json")
	swipeResp, err := http.DefaultClient.Do(swipeReq)
	if err != nil {
		t.Fatal(err)
	}
	defer swipeResp.Body.Close()
	if swipeResp.StatusCode != http.StatusOK {
		t.Fatalf("swipe status %d", swipeResp.StatusCode)
	}

	var swipe struct {
		Decision string `json:"decision"`
		EventID  string `json:"eventId"`
	}
	if err := json.NewDecoder(swipeResp.Body).Decode(&swipe); err != nil {
		t.Fatal(err)
	}
	if swipe.EventID == "" {
		t.Fatal("swipe response missing eventId")
	}

	deadline := time.Now().Add(30 * time.Second)
	var (
		employeeID string
		direction  string
		status     string
	)
	for time.Now().Before(deadline) {
		row := db.QueryRowContext(ctx, `
			SELECT employee_id, direction, status
			FROM inout_events WHERE id = ?`, swipe.EventID)
		err = row.Scan(&employeeID, &direction, &status)
		if err == nil {
			break
		}
		if err != sql.ErrNoRows {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
	}

	if employeeID == "" {
		t.Fatalf("event %s not found in inout_events within 30s", swipe.EventID)
	}
	if employeeID != userID {
		t.Fatalf("employee_id: got %s want %s", employeeID, userID)
	}
	if direction != "IN" {
		t.Fatalf("direction: got %s want IN", direction)
	}
	if status != swipe.Decision {
		t.Fatalf("status: got %s want %s", status, swipe.Decision)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
