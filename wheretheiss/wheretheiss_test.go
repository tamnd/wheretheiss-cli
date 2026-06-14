package wheretheiss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

const positionJSON = `{
  "name": "iss",
  "id": 25544,
  "latitude": -21.426027,
  "longitude": -79.431573,
  "altitude": 421.692,
  "velocity": 27576.659,
  "visibility": "daylight",
  "footprint": 4539.2,
  "timestamp": 1718354400,
  "daynum": 2460477.5,
  "solar_lat": 23.4,
  "solar_lon": 180.2,
  "units": "kilometers"
}`

const historyJSON = `[
  {
    "name": "iss",
    "id": 25544,
    "latitude": -21.426027,
    "longitude": -79.431573,
    "altitude": 421.692,
    "velocity": 27576.659,
    "visibility": "daylight",
    "footprint": 4539.2,
    "timestamp": 1718354400,
    "daynum": 2460477.5,
    "solar_lat": 23.4,
    "solar_lon": 180.2,
    "units": "kilometers"
  },
  {
    "name": "iss",
    "id": 25544,
    "latitude": -22.0,
    "longitude": -80.0,
    "altitude": 422.0,
    "velocity": 27580.0,
    "visibility": "daylight",
    "footprint": 4540.0,
    "timestamp": 1718354500,
    "daynum": 2460477.6,
    "solar_lat": 23.5,
    "solar_lon": 181.0,
    "units": "kilometers"
  }
]`

func TestCurrentPosition(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/satellites/25544" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(positionJSON))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	c.Rate = 0

	pos, err := c.CurrentPosition(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pos.Name != "iss" {
		t.Errorf("Name = %q, want iss", pos.Name)
	}
	if pos.ID != ISSID {
		t.Errorf("ID = %d, want %d", pos.ID, ISSID)
	}
	if pos.Latitude != -21.426027 {
		t.Errorf("Latitude = %f, want -21.426027", pos.Latitude)
	}
	if pos.Units != "kilometers" {
		t.Errorf("Units = %q, want kilometers", pos.Units)
	}
}

func TestHistoricalPositions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/satellites/25544/positions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		ts := r.URL.Query().Get("timestamps")
		if ts == "" {
			t.Error("timestamps query param missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(historyJSON))
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	c.Rate = 0

	positions, err := c.HistoricalPositions(context.Background(), []int64{1718354400, 1718354500})
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 2 {
		t.Fatalf("got %d positions, want 2", len(positions))
	}
	if positions[0].Timestamp != 1718354400 {
		t.Errorf("positions[0].Timestamp = %d, want 1718354400", positions[0].Timestamp)
	}
	if positions[1].Timestamp != 1718354500 {
		t.Errorf("positions[1].Timestamp = %d, want 1718354500", positions[1].Timestamp)
	}
}

func TestHistoricalPositionsEmpty(t *testing.T) {
	c := NewClient()
	_, err := c.HistoricalPositions(context.Background(), nil)
	if err == nil {
		t.Error("expected error for empty timestamps, got nil")
	}
}
