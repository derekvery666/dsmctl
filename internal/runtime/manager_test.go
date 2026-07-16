package runtime

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/config"
)

type resolverFunc func(context.Context, string, config.Profile) (string, error)

func (f resolverFunc) Password(ctx context.Context, name string, profile config.Profile) (string, error) {
	return f(ctx, name, profile)
}

func TestManagerMaintainsIndependentClientsPerNAS(t *testing.T) {
	office := newDSMServer(t, "DS923+", "office-sid")
	defer office.Close()
	lab := newDSMServer(t, "DS224+", "lab-sid")
	defer lab.Close()

	cfg := config.New()
	cfg.DefaultNAS = "office"
	cfg.NAS["office"] = config.Profile{URL: office.URL, Username: "user"}
	cfg.NAS["lab"] = config.Profile{URL: lab.URL, Username: "user"}
	manager := NewManager(cfg, resolverFunc(func(_ context.Context, name string, _ config.Profile) (string, error) {
		return name + "-password", nil
	}))

	_, officeClient, err := manager.Client(context.Background(), "office")
	if err != nil {
		t.Fatalf("office Client() error = %v", err)
	}
	_, labClient, err := manager.Client(context.Background(), "lab")
	if err != nil {
		t.Fatalf("lab Client() error = %v", err)
	}
	officeInfo, err := officeClient.SystemInfo(context.Background())
	if err != nil {
		t.Fatalf("office SystemInfo() error = %v", err)
	}
	labInfo, err := labClient.SystemInfo(context.Background())
	if err != nil {
		t.Fatalf("lab SystemInfo() error = %v", err)
	}
	if officeInfo.Model != "DS923+" || labInfo.Model != "DS224+" {
		t.Fatalf("models office=%q lab=%q", officeInfo.Model, labInfo.Model)
	}
	if err := manager.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func newDSMServer(t *testing.T, model, sid string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Form.Get("api") + "." + r.Form.Get("method") {
		case "SYNO.API.Info.query":
			fmt.Fprint(w, `{"success":true,"data":{"SYNO.API.Auth":{"path":"entry.cgi","minVersion":1,"maxVersion":7},"SYNO.Core.System":{"path":"entry.cgi","minVersion":1,"maxVersion":3}}}`)
		case "SYNO.API.Auth.login":
			fmt.Fprintf(w, `{"success":true,"data":{"sid":%q}}`, sid)
		case "SYNO.Core.System.info":
			if r.Form.Get("_sid") != sid {
				t.Errorf("system info SID = %q, want %q", r.Form.Get("_sid"), sid)
			}
			fmt.Fprintf(w, `{"success":true,"data":{"model":%q}}`, model)
		case "SYNO.API.Auth.logout":
			fmt.Fprint(w, `{"success":true,"data":{}}`)
		default:
			t.Errorf("unexpected API call %s.%s", r.Form.Get("api"), r.Form.Get("method"))
			fmt.Fprint(w, `{"success":false,"error":{"code":102}}`)
		}
	}))
}
