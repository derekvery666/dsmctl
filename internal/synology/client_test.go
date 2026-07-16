package synology

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSystemInfoLoginAndLogout(t *testing.T) {
	var loginCount, logoutCount, infoCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
		}
		api := r.Form.Get("api")
		method := r.Form.Get("method")
		w.Header().Set("Content-Type", "application/json")
		switch api + "." + method {
		case "SYNO.API.Info.query":
			fmt.Fprint(w, `{"success":true,"data":{"SYNO.API.Auth":{"path":"entry.cgi","minVersion":1,"maxVersion":7},"SYNO.Core.System":{"path":"entry.cgi","minVersion":1,"maxVersion":3}}}`)
		case "SYNO.API.Auth.login":
			loginCount++
			if got := r.Form.Get("passwd"); got != "secret" {
				t.Errorf("passwd = %q", got)
			}
			if r.URL.Query().Get("passwd") != "" {
				t.Error("password was placed in URL query")
			}
			fmt.Fprint(w, `{"success":true,"data":{"sid":"test-sid","synotoken":"test-token"}}`)
		case "SYNO.Core.System.info":
			infoCount++
			if r.Form.Get("_sid") != "test-sid" || r.Form.Get("SynoToken") != "test-token" {
				t.Errorf("missing session credentials: %#v", r.Form)
			}
			fmt.Fprint(w, `{"success":true,"data":{"hostname":"office-nas","model":"DS923+","serial":"SERIAL","firmware_ver":"DSM 7.2","cpu_vendor":"AMD","cpu_series":"R1600","cpu_cores":"2","ram_size":4096,"up_time":"3 days","time_zone":"Asia/Taipei","sys_temp":41,"sys_tempwarn":false}}`)
		case "SYNO.API.Auth.logout":
			logoutCount++
			fmt.Fprint(w, `{"success":true,"data":{}}`)
		default:
			t.Errorf("unexpected request %s.%s", api, method)
			fmt.Fprint(w, `{"success":false,"error":{"code":102}}`)
		}
	}))
	defer server.Close()

	client, err := NewClient(Options{
		BaseURL:    server.URL,
		Username:   "automation",
		Password:   "secret",
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	info, err := client.SystemInfo(context.Background())
	if err != nil {
		t.Fatalf("SystemInfo() error = %v", err)
	}
	if info.Model != "DS923+" || info.Hostname != "office-nas" || info.MemoryMiB != 4096 {
		t.Fatalf("SystemInfo() = %#v", info)
	}
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if loginCount != 1 || infoCount != 1 || logoutCount != 1 {
		t.Fatalf("request counts login=%d info=%d logout=%d", loginCount, infoCount, logoutCount)
	}
}
