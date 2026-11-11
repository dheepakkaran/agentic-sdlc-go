package inventory

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func seedFilter(t *testing.T, s *Store) {
	t.Helper()
	for _, d := range []Device{
		{ID: "d4", Hostname: "h", Role: "router", Site: "sea"},
		{ID: "d2", Hostname: "h", Role: "switch", Site: "sjc"},
		{ID: "d3", Hostname: "h", Role: "router", Site: "sjc"},
		{ID: "d1", Hostname: "h", Role: "router", Site: "sjc"},
		{ID: "d5", Hostname: "h", Role: "switch", Site: "sea"},
	} {
		if _, err := s.Create(d); err != nil {
			t.Fatal(err)
		}
	}
}

var filterCases = []struct {
	name, query string
	site, role  string
	want        []string
}{
	{"none", "", "", "", []string{"d1", "d2", "d3", "d4", "d5"}},
	{"site", "?site=sjc", "sjc", "", []string{"d1", "d2", "d3"}},
	{"role", "?role=router", "", "router", []string{"d1", "d3", "d4"}},
	{"both", "?site=sjc&role=router", "sjc", "router", []string{"d1", "d3"}},
	{"no match", "?site=nonexistent", "nonexistent", "", []string{}},
	{"case sensitive", "?site=SJC", "SJC", "", []string{}},
	{"empty site", "?site=", "", "", []string{"d1", "d2", "d3", "d4", "d5"}},
	{"empty site role", "?site=&role=router", "", "router", []string{"d1", "d3", "d4"}},
	{"unknown param", "?foo=bar", "", "", []string{"d1", "d2", "d3", "d4", "d5"}},
}

func idsOf(l []Device) []string {
	ids := make([]string, 0, len(l))
	for _, d := range l {
		ids = append(ids, d.ID)
	}
	return ids
}

func TestStoreListFiltered(t *testing.T) {
	for _, tc := range filterCases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore()
			seedFilter(t, s)
			got := s.ListFiltered(tc.site, tc.role)
			if got == nil {
				t.Fatal("nil slice")
			}
			if g, w := strings.Join(idsOf(got), ","), strings.Join(tc.want, ","); g != w {
				t.Fatalf("got %s want %s", g, w)
			}
		})
	}
}

func TestHTTPListFilter(t *testing.T) {
	for _, tc := range filterCases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore()
			seedFilter(t, s)
			srv := httptest.NewServer(Handler(s))
			defer srv.Close()
			res, err := http.Get(srv.URL + "/devices" + tc.query)
			if err != nil || res.StatusCode != http.StatusOK {
				t.Fatalf("get: %v %v", err, res)
			}
			defer func() { _ = res.Body.Close() }()
			var list []Device
			if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
				t.Fatal(err)
			}
			if list == nil {
				t.Fatal("want [] not null")
			}
			if g, w := strings.Join(idsOf(list), ","), strings.Join(tc.want, ","); g != w {
				t.Fatalf("got %s want %s", g, w)
			}
			for _, d := range list {
				if (tc.site != "" && d.Site != tc.site) || (tc.role != "" && d.Role != tc.role) {
					t.Fatalf("mismatch: %+v", d)
				}
			}
		})
	}
}

func TestStoreSetStatus(t *testing.T) {
	cases := []struct {
		name, id, initial, set string
		want                   error
	}{
		{"up to down", "r1", "up", "down", nil},
		{"down to up", "r1", "down", "up", nil},
		{"same status", "r1", "up", "up", nil},
		{"unknown id", "zz", "up", "down", ErrNotFound},
		{"maintenance", "r1", "up", "maintenance", ErrInvalid},
		{"uppercase", "r1", "up", "UP", ErrInvalid},
		{"empty", "r1", "up", "", ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStore()
			base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			n := 0
			s.now = func() time.Time {
				n++
				return base.Add(time.Duration(n) * time.Second)
			}
			before, err := s.Create(Device{ID: "r1", Hostname: "h", Role: "router", Site: "sjc", MgmtIP: "10.0.0.1", Status: tc.initial})
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.SetStatus(tc.id, tc.set)
			if err != tc.want {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			stored, _ := s.Get("r1")
			if len(s.List()) != 1 {
				t.Fatalf("list len %d", len(s.List()))
			}
			if tc.want != nil {
				if stored != before {
					t.Fatalf("device changed: %+v", stored)
				}
				return
			}
			if got != stored || stored.Status != tc.set {
				t.Fatalf("status: %+v", stored)
			}
			if !stored.UpdatedAt.After(before.UpdatedAt) {
				t.Fatalf("updated_at not later: %v %v", before.UpdatedAt, stored.UpdatedAt)
			}
			stored.Status, stored.UpdatedAt = before.Status, before.UpdatedAt
			if stored != before {
				t.Fatalf("other fields changed: %+v vs %+v", stored, before)
			}
		})
	}
}

func TestHTTPSetStatus(t *testing.T) {
	cases := []struct {
		name, id, body string
		want           int
		after          string
	}{
		{"down", "r1", `{"status":"down"}`, http.StatusOK, "down"},
		{"up", "r1", `{"status":"up"}`, http.StatusOK, "up"},
		{"unknown id", "zz", `{"status":"down"}`, http.StatusNotFound, ""},
		{"maintenance", "r1", `{"status":"maintenance"}`, http.StatusBadRequest, "up"},
		{"empty status", "r1", `{"status":""}`, http.StatusBadRequest, "up"},
		{"malformed", "r1", `{`, http.StatusBadRequest, "up"},
		{"empty body", "r1", ``, http.StatusBadRequest, "up"},
		{"trailing data", "r1", `{"status":"down"}x`, http.StatusBadRequest, "up"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(Handler(NewStore()))
			defer srv.Close()
			res, err := http.Post(srv.URL+"/devices", "application/json", strings.NewReader(`{"id":"r1","hostname":"a"}`))
			if err != nil || res.StatusCode != http.StatusCreated {
				t.Fatalf("create: %v", err)
			}
			req, _ := http.NewRequest(http.MethodPut, srv.URL+"/devices/"+tc.id+"/status", strings.NewReader(tc.body))
			res, err = http.DefaultClient.Do(req)
			if err != nil || res.StatusCode != tc.want {
				t.Fatalf("put: %v %v want %d", err, res.StatusCode, tc.want)
			}
			if tc.id == "zz" {
				res, _ = http.Get(srv.URL + "/devices/zz")
				if res.StatusCode != http.StatusNotFound {
					t.Fatalf("device created: %d", res.StatusCode)
				}
				return
			}
			res, _ = http.Get(srv.URL + "/devices/r1")
			var d Device
			if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
				t.Fatal(err)
			}
			if tc.want == http.StatusOK {
				if d.Status != tc.after {
					t.Fatalf("status %q want %q", d.Status, tc.after)
				}
				return
			}
			if d.Status != "up" {
				t.Fatalf("status changed: %q", d.Status)
			}
		})
	}
}
