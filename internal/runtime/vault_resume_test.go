package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/flynn/noise"

	"github.com/derekvery666/dsmctl/internal/credentials"
	"github.com/derekvery666/dsmctl/internal/gateway/state"
)

func TestExpiredVaultSessionResumesAndPersistsWithoutRevisionChange(t *testing.T) {
	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)
	serverKey, err := suite.GenerateKeypair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	localKey, err := suite.GenerateKeypair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	resumeCalls := 0
	dsm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = req.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch req.Form.Get("api") + "." + req.Form.Get("method") {
		case "SYNO.API.Info.query":
			fmt.Fprint(w, `{"success":true,"data":{"SYNO.API.Auth":{"path":"entry.cgi","minVersion":1,"maxVersion":7},"SYNO.Core.System":{"path":"entry.cgi","minVersion":1,"maxVersion":3}}}`)
		case "SYNO.Core.System.info":
			if req.Form.Get("_sid") == "expired-sid" {
				fmt.Fprint(w, `{"success":false,"error":{"code":119}}`)
				return
			}
			if req.Form.Get("_sid") != "resumed-sid" {
				t.Errorf("system info SID = %q", req.Form.Get("_sid"))
			}
			fmt.Fprint(w, `{"success":true,"data":{"model":"DS1821+"}}`)
		case "SYNO.API.Auth.resume":
			resumeCalls++
			message, err := base64.URLEncoding.DecodeString(req.Form.Get("kk_message"))
			if err != nil {
				t.Error(err)
			}
			handshake, err := noise.NewHandshakeState(noise.Config{CipherSuite: suite, Random: rand.Reader, Pattern: noise.HandshakeKK, Initiator: false, StaticKeypair: serverKey, PeerStatic: localKey.Public})
			if err != nil {
				t.Error(err)
			}
			if _, _, _, err := handshake.ReadMessage(nil, message); err != nil {
				t.Error(err)
			}
			reply, _, _, err := handshake.WriteMessage(nil, nil)
			if err != nil {
				t.Error(err)
			}
			fmt.Fprintf(w, `{"success":true,"data":{"account":"operator","sid":"resumed-sid","synotoken":"rotated-token","device_id":"device","kk_message":%q}}`, base64.URLEncoding.EncodeToString(reply))
		default:
			t.Errorf("unexpected DSM call %s.%s", req.Form.Get("api"), req.Form.Get("method"))
			fmt.Fprint(w, `{"success":false,"error":{"code":102}}`)
		}
	}))
	defer dsm.Close()

	databasePath := filepath.Join(t.TempDir(), "gateway.db")
	masterKey := bytes.Repeat([]byte{4}, 32)
	repository, err := state.Open(databasePath, masterKey)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	ctx := context.Background()
	if _, err := repository.CreateProfile(ctx, state.ProfileInput{Name: "office", URL: dsm.URL, Username: "operator"}); err != nil {
		t.Fatal(err)
	}
	revision, err := repository.EnrollSession(ctx, "office", credentials.SessionCredential{
		SID: "expired-sid", SynoToken: "expired-token", Account: "operator", DeviceID: "device",
		ServerPublicKey: serverKey.Public, LocalPublicKey: localKey.Public, LocalPrivateKey: localKey.Private,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	repository, err = state.Open(databasePath, masterKey)
	if err != nil {
		t.Fatalf("reopen vaulted session after gateway restart: %v", err)
	}
	defer repository.Close()
	cfg, _ := repository.Snapshot(ctx)
	manager := NewManager(cfg, repository, WithConfigSource(repository), WithSessionStore(repository))
	defer manager.Close(ctx)
	_, client, err := manager.Client(ctx, "office")
	if err != nil {
		t.Fatal(err)
	}
	info, err := client.SystemInfo(ctx)
	if err != nil || info.Model != "DS1821+" {
		t.Fatalf("SystemInfo() = %#v, %v", info, err)
	}
	stored, err := repository.Session(ctx, "office")
	if err != nil || stored.SID != "resumed-sid" || stored.SynoToken != "rotated-token" {
		t.Fatalf("stored resumed session = %#v, %v", stored, err)
	}
	profile, _ := repository.Profile(ctx, "office")
	if profile.Revision != revision {
		t.Fatalf("resume advanced profile revision from %d to %d", revision, profile.Revision)
	}
	if resumeCalls != 1 || !manager.SessionInfo("office").ClientCached {
		t.Fatalf("resumeCalls=%d sessionInfo=%#v", resumeCalls, manager.SessionInfo("office"))
	}
}
