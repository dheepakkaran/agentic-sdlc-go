package inventory

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStoreCreate(t *testing.T) {
	cases := []struct {
		name string
		in   Device
		want error
	}{
		{"ok", Device{ID: "r1", Hostname: "core-r1"}, nil},
		{"missing id", Device{Hostname: "x"}, ErrInvalid},
		{"bad status", Device{ID: "r2", Hostname: "x", Status: "maybe"}, ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore()
			_, err := s.Create(tc.in)
			if err != tc.want {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestStoreDuplicate(t *testing.T) {
	s := NewStore()
	if _, err := s.Create(Device{ID: "r1", Hostname: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(Device{ID: "r1", Hostname: "b"}); err != ErrExists {
		t.Fatalf("want ErrExists, got %v", err)
	}
}

func TestHTTPFlow(t *testing.T) {
	srv := httptest.NewServer(Handler(NewStore()))
	defer srv.Close()

	body, _ := json.Marshal(Device{ID: "r1", Hostname: "core-r1", Role: "router", Site: "sjc"})
	res, err := http.Post(srv.URL+"/devices", "application/json", bytes.NewReader(body))
	if err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create: %v %v", err, res.StatusCode)
	}
	res, _ = http.Get(srv.URL + "/devices/r1")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get: %d", res.StatusCode)
	}
	res, _ = http.Get(srv.URL + "/devices/nope")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing: %d", res.StatusCode)
	}
	res, _ = http.Get(srv.URL + "/health")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health: %d", res.StatusCode)
	}
}

func TestStoreListDelete(t *testing.T) {
	s := NewStore()
	for _, id := range []string{"r2", "r1"} {
		if _, err := s.Create(Device{ID: id, Hostname: id}); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.List(); len(got) != 2 || got[0].ID != "r1" {
		t.Fatalf("list not sorted: %+v", got)
	}
	if err := s.Delete("r1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("r1"); err != ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := s.Get("r1"); err != ErrNotFound {
		t.Fatalf("get deleted: %v", err)
	}
}

func TestHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(Handler(NewStore()))
	defer srv.Close()
	post := func(body string) int {
		res, err := http.Post(srv.URL+"/devices", "application/json", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode
	}
	cases := []struct {
		name, body string
		want       int
	}{
		{"bad json", "{", http.StatusBadRequest},
		{"invalid device", `{"id":""}`, http.StatusBadRequest},
		{"created", `{"id":"r1","hostname":"a"}`, http.StatusCreated},
		{"duplicate", `{"id":"r1","hostname":"a"}`, http.StatusConflict},
	}
	for _, c := range cases {
		if got := post(c.body); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
	for path, want := range map[string]int{"/devices/r1": http.StatusNoContent, "/devices/zz": http.StatusNotFound} {
		req, _ := http.NewRequest(http.MethodDelete, srv.URL+path, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != want {
			t.Errorf("DELETE %s: %v %d want %d", path, err, res.StatusCode, want)
		}
	}
	res, _ := http.Get(srv.URL + "/devices")
	var list []Device
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil || len(list) != 0 {
		t.Errorf("list after delete: %v %v", err, list)
	}
}
