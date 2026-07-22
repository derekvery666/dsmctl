package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ychiu1211/dsmctl/internal/gateway/state"
)

func newBookDSM(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = req.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch req.Form.Get("api") + "." + req.Form.Get("method") {
		case "SYNO.API.Info.query":
			fmt.Fprint(w, `{"success":true,"data":{"SYNO.API.Auth":{"path":"entry.cgi","minVersion":1,"maxVersion":7}}}`)
		case "SYNO.API.Auth.login":
			fmt.Fprint(w, `{"success":true,"data":{"sid":"temporary-sid"}}`)
		case "SYNO.API.Auth.logout":
			fmt.Fprint(w, `{"success":true,"data":{}}`)
		default:
			fmt.Fprint(w, `{"success":false,"error":{"code":102}}`)
		}
	}))
}

func TestPasswordBookAccountsAndAccountReveal(t *testing.T) {
	dsm := newBookDSM(t)
	defer dsm.Close()
	handler, repository, manager, adminSession := newTestHandler(t)
	defer manager.Close(context.Background())
	ctx := context.Background()

	profile, err := repository.CreateProfile(ctx, state.ProfileInput{Name: "book", URL: dsm.URL})
	if err != nil {
		t.Fatal(err)
	}
	primary := fmt.Sprintf(`{"account":"admin","expected_revision":%d,"password":"admin-pw"}`, profile.Revision)
	if r := performJSON(handler, http.MethodPost, "/admin/api/profiles/book/credentials/password", primary, adminSession); r.Code != http.StatusOK {
		t.Fatalf("primary enroll = %d %s", r.Code, r.Body.String())
	} else if !strings.Contains(r.Body.String(), `"session_stored":true`) {
		t.Fatalf("primary enrollment did not retain its DSM session: %s", r.Body.String())
	}
	if meta, err := repository.SessionMeta(ctx, "book"); err != nil || !meta.Present || meta.Account != "admin" {
		t.Fatalf("primary session metadata = %#v, err=%v", meta, err)
	}
	after, _ := repository.Profile(ctx, "book")
	secondary := fmt.Sprintf(`{"account":"backup","expected_revision":%d,"password":"backup-pw"}`, after.Revision)
	if r := performJSON(handler, http.MethodPost, "/admin/api/profiles/book/credentials/password", secondary, adminSession); r.Code != http.StatusOK {
		t.Fatalf("secondary enroll = %d %s", r.Code, r.Body.String())
	} else if !strings.Contains(r.Body.String(), `"session_stored":false`) {
		t.Fatalf("secondary enrollment gained a runtime session: %s", r.Body.String())
	}

	connected := performJSON(handler, http.MethodPost, "/admin/api/profiles/book/credentials/password/connect", `{}`, adminSession)
	if connected.Code != http.StatusOK || !strings.Contains(connected.Body.String(), `"session_stored":true`) || strings.Contains(connected.Body.String(), "temporary-sid") {
		t.Fatalf("connect stored password = %d %s", connected.Code, connected.Body.String())
	}

	// The accounts endpoint lists both entries (primary first) and never a password.
	accts := performJSON(handler, http.MethodGet, "/admin/api/profiles/book/credentials/accounts", "", adminSession)
	if accts.Code != http.StatusOK {
		t.Fatalf("accounts = %d %s", accts.Code, accts.Body.String())
	}
	body := accts.Body.String()
	if strings.Contains(body, "admin-pw") || strings.Contains(body, "backup-pw") {
		t.Fatalf("accounts listing leaked a password: %s", body)
	}
	if !strings.Contains(body, `"account":"admin"`) || !strings.Contains(body, `"account":"backup"`) {
		t.Fatalf("accounts listing missing entries: %s", body)
	}

	// Reveal resolves the requested account, not just the primary.
	reveal := performJSON(handler, http.MethodPost, "/admin/api/profiles/book/credentials/password/reveal", `{"account":"backup"}`, adminSession)
	if reveal.Code != http.StatusOK {
		t.Fatalf("reveal backup = %d %s", reveal.Code, reveal.Body.String())
	}
	var out struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(reveal.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Account != "backup" || out.Password != "backup-pw" {
		t.Fatalf("reveal backup = %#v", out)
	}

	// Deleting one account by name leaves the other.
	del := performJSON(handler, http.MethodDelete, "/admin/api/profiles/book/credentials/password?account=backup", "", adminSession)
	if del.Code != http.StatusOK || !strings.Contains(del.Body.String(), `"removed":true`) {
		t.Fatalf("delete backup = %d %s", del.Code, del.Body.String())
	}
	if remaining, _ := repository.PasswordAccounts(ctx, "book"); len(remaining) != 1 || remaining[0].Account != "admin" {
		t.Fatalf("post-delete book = %#v", remaining)
	}
}

func TestConnectStoredPasswordRequiresPrimaryCredential(t *testing.T) {
	dsm := newBookDSM(t)
	defer dsm.Close()
	handler, repository, manager, adminSession := newTestHandler(t)
	defer manager.Close(context.Background())
	if _, err := repository.CreateProfile(context.Background(), state.ProfileInput{Name: "empty", URL: dsm.URL}); err != nil {
		t.Fatal(err)
	}
	response := performJSON(handler, http.MethodPost, "/admin/api/profiles/empty/credentials/password/connect", `{}`, adminSession)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "no primary password") {
		t.Fatalf("missing password connect = %d %s", response.Code, response.Body.String())
	}
}

func TestCreateTargetProfileThroughAdminAPI(t *testing.T) {
	dsm := newBookDSM(t)
	defer dsm.Close()
	handler, _, manager, adminSession := newTestHandler(t)
	defer manager.Close(context.Background())

	created := performJSON(handler, http.MethodPost, "/admin/api/profiles", fmt.Sprintf(`{"name":"dst","url":%q,"role":"target"}`, dsm.URL), adminSession)
	if created.Code != http.StatusCreated {
		t.Fatalf("create target = %d %s", created.Code, created.Body.String())
	}
	if !strings.Contains(created.Body.String(), `"role":"target"`) {
		t.Fatalf("created profile did not persist the target role: %s", created.Body.String())
	}
}
