package jobs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mikrotik-vpn-manager/internal/domain"
	"mikrotik-vpn-manager/internal/routers"
	"mikrotik-vpn-manager/internal/storage"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeAPI struct {
	records   map[string]routers.Record
	puts      int
	failAt    int
	uncertain bool
	mismatch  bool
	calls     int
}

func (f *fakeAPI) Call(_ context.Context, method, path string, body any) ([]routers.Record, error) {
	f.calls++
	switch method {
	case "PUT":
		f.puts++
		if f.puts == f.failAt {
			if f.uncertain {
				return nil, errors.New("connection lost after write")
			}
			return nil, &routers.APIError{Status: 400}
		}
		id := fmt.Sprintf("*%X", f.puts)
		r := routers.Record{".id": id}
		for k, v := range body.(map[string]string) {
			r[k] = v
		}
		f.records[path+"/"+id] = r
		return []routers.Record{r}, nil
	case "GET":
		r := f.records[path]
		if r == nil {
			if !strings.Contains(path, "/*") {
				rows := []routers.Record{}
				for key, row := range f.records {
					if strings.HasPrefix(key, path+"/") {
						rows = append(rows, row)
					}
				}
				return rows, nil
			}
			return nil, errors.New("not found")
		}
		if f.mismatch {
			return []routers.Record{{".id": r[".id"], "name": "changed"}}, nil
		}
		return []routers.Record{r}, nil
	case "DELETE":
		delete(f.records, path)
		return nil, nil
	case "POST":
		if strings.HasSuffix(path, "/set") {
			r := f.records[strings.TrimSuffix(path, "/set")]
			for key, value := range body.(map[string]string) {
				r[key] = value
			}
		}
		return nil, nil
	}
	return nil, nil
}

func TestSingletonRollbackAndPreviewDrift(t *testing.T) {
	e := testEngine(t)
	f := &fakeAPI{records: map[string]routers.Record{"interface/sstp-server/server": {"enabled": "yes", "certificate": "mvm-cert"}}}
	j := testJob()
	j.Owned = []domain.Resource{{Path: "interface/sstp-server/server", Desired: map[string]string{"enabled": "yes", "certificate": "mvm-cert"}, Previous: map[string]string{"enabled": "no", "certificate": "none"}}}
	if err := e.rollback(context.Background(), f, j); err != nil {
		t.Fatal(err)
	}
	if f.records["interface/sstp-server/server"]["enabled"] != "no" || len(j.Owned) != 0 {
		t.Fatal("singleton was not restored")
	}
	j = testJob()
	j.Steps = []domain.Step{{Path: "interface/sstp-server/server/set", Method: "POST", Values: map[string]string{"enabled": "yes"}, Previous: map[string]string{"enabled": "yes"}}}
	if err := e.Apply(context.Background(), f, j); err == nil || f.puts != 0 || f.records["interface/sstp-server/server"]["enabled"] != "no" {
		t.Fatal("stale preview changed a singleton")
	}
}

func TestTerminalIPsecSecretIsScrubbed(t *testing.T) {
	e := testEngine(t)
	j := testJob()
	j.State = "COMPLETED"
	j.Steps = []domain.Step{{Path: "interface/l2tp-server/server/set", Method: "POST", Values: map[string]string{"ipsec-secret": "do-not-retain", "enabled": "yes"}}}
	if err := e.Save(j); err != nil {
		t.Fatal(err)
	}
	var stored domain.Job
	if err := e.Repo.Get("jobs", j.ID, &stored); err != nil {
		t.Fatal(err)
	}
	if _, ok := stored.Steps[0].Values["ipsec-secret"]; ok {
		t.Fatal("terminal PSK retained")
	}
}

func TestListenerBlockedBeforeUnverifiedFirewall(t *testing.T) {
	e := testEngine(t)
	f := &fakeAPI{records: map[string]routers.Record{}}
	j := testJob()
	j.Server = &domain.Server{Spec: domain.ServerSpec{InputAnchor: "*A", ForwardAnchor: "*B"}}
	j.Steps = []domain.Step{{Path: "interface/ovpn-server/server", Method: "PUT", Values: map[string]string{"name": "listener", "disabled": "no"}}}
	if err := e.Apply(context.Background(), f, j); err == nil || f.puts != 0 || j.State != "MANUAL_INTERVENTION" {
		t.Fatal("listener activated without reviewed firewall anchors")
	}
}

func TestRollbackRefusesExternalDependency(t *testing.T) {
	e := testEngine(t)
	j := testJob()
	j.Owned = []domain.Resource{{Path: "ppp/profile", ID: "*1", Desired: map[string]string{"name": "profile", "comment": "mvm:job"}}}
	f := &fakeAPI{records: map[string]routers.Record{"ppp/profile/*1": {".id": "*1", "name": "profile", "comment": "mvm:job"}, "ppp/secret/*F": {".id": "*F", "profile": "profile", "comment": "external"}}}
	if err := e.rollback(context.Background(), f, j); err == nil {
		t.Fatal("deleted a referenced profile")
	}
	if f.records["ppp/profile/*1"] == nil {
		t.Fatal("profile removed")
	}
}

func TestCertificateServerAndUserRejections(t *testing.T) {
	for _, menu := range []string{"certificate", "interface/ovpn-server/server", "ppp/secret"} {
		t.Run(menu, func(t *testing.T) {
			e := testEngine(t)
			j := testJob()
			j.Steps[1].Path = menu
			f := &fakeAPI{records: map[string]routers.Record{}, failAt: 2}
			if err := e.Apply(context.Background(), f, j); err == nil || j.State != "ROLLED_BACK" || len(f.records) != 0 {
				t.Fatalf("unsafe partial provisioning: %s", j.State)
			}
		})
	}
}

type failingSign struct{ *fakeAPI }

func (f failingSign) Call(ctx context.Context, method, path string, body any) ([]routers.Record, error) {
	if method == "POST" && path == "certificate/sign" {
		return nil, &routers.APIError{Status: 400}
	}
	return f.fakeAPI.Call(ctx, method, path, body)
}
func TestFailedCertificateSigning(t *testing.T) {
	e := testEngine(t)
	j := testJob()
	j.Steps = []domain.Step{{Method: "PUT", Path: "certificate", Values: map[string]string{"name": "cert", "comment": "mvm:job"}}, {Method: "POST", Path: "certificate/sign", Values: map[string]string{"numbers": "cert", "name": "cert"}}}
	f := failingSign{&fakeAPI{records: map[string]routers.Record{}}}
	if err := e.Apply(context.Background(), f, j); err == nil || j.State != "ROLLED_BACK" {
		t.Fatalf("signing failure not compensated: %s", j.State)
	}
}
func testEngine(t *testing.T) *Engine {
	t.Helper()
	s, e := storage.Open(filepath.Join(t.TempDir(), "test.db"), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return &Engine{Repo: s}
}
func testJob() *domain.Job {
	return &domain.Job{ID: "job", RouterID: "router", State: "READY", Created: time.Now(), Steps: []domain.Step{{Method: "PUT", Path: "ip/pool", Values: map[string]string{"name": "pool", "comment": "mvm:job"}}, {Method: "PUT", Path: "ppp/profile", Values: map[string]string{"name": "profile", "comment": "mvm:job"}}}}
}
func TestJobOutcomes(t *testing.T) {
	for _, v := range []struct {
		name                string
		failAt              int
		uncertain, mismatch bool
		state               string
	}{{"success", 0, false, false, "COMPLETED"}, {"partial rejection", 2, false, false, "ROLLED_BACK"}, {"uncertain", 2, true, false, "MANUAL_INTERVENTION"}, {"readback drift", 0, false, true, "MANUAL_INTERVENTION"}} {
		t.Run(v.name, func(t *testing.T) {
			e := testEngine(t)
			j := testJob()
			j.Steps[0].Values["password"] = "do-not-retain"
			f := &fakeAPI{records: map[string]routers.Record{}, failAt: v.failAt, uncertain: v.uncertain, mismatch: v.mismatch}
			_ = e.Apply(context.Background(), f, j)
			if j.State != v.state {
				t.Fatalf("%s %s", j.State, j.Error)
			}
			var saved domain.Job
			if err := e.Repo.Get("jobs", j.ID, &saved); err != nil {
				t.Fatal(err)
			}
			if _, ok := saved.Steps[0].Values["password"]; ok {
				t.Fatal("terminal job retained password")
			}
			if v.failAt != 1 && f.puts > 0 && v.state == "COMPLETED" && f.records["ip/pool/*1"]["password"] != "do-not-retain" {
				t.Fatal("password removed before router mutation")
			}
			if j.State == "ROLLED_BACK" && len(f.records) > 0 {
				t.Fatal("rollback left resources")
			}
			if j.State == "COMPLETED" {
				calls := f.calls
				if err := e.Apply(context.Background(), f, j); err != nil || calls != f.calls {
					t.Fatal("completed job replayed")
				}
			}
		})
	}
}
func TestRestartAndIdempotency(t *testing.T) {
	e := testEngine(t)
	j := testJob()
	j.Key = "unique-key"
	j.Fingerprint = "one"
	if err := e.Prepare(j); err != nil {
		t.Fatal(err)
	}
	other := testJob()
	other.ID = "other"
	other.Key = j.Key
	other.Fingerprint = "two"
	if err := e.Prepare(other); err == nil {
		t.Fatal("key collision accepted")
	}
	j.State = "APPLYING"
	j.InFlight = true
	if err := e.Save(j); err != nil {
		t.Fatal(err)
	}
	if err := e.Recover(); err != nil {
		t.Fatal(err)
	}
	var got domain.Job
	if err := e.Repo.Get("jobs", j.ID, &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "MANUAL_INTERVENTION" || !e.Blocked(j.RouterID, "") {
		t.Fatal("restart did not block")
	}
}
func TestExpiredPreview(t *testing.T) {
	e := testEngine(t)
	j := testJob()
	j.Created = time.Now().Add(-time.Hour)
	f := &fakeAPI{}
	if err := e.Apply(context.Background(), f, j); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatal("expired preview accepted")
	}
}
