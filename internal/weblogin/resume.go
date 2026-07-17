package weblogin

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/flynn/noise"
)

// ResumeInput is the durable material persisted from a web login that lets a
// session be refreshed without a browser. The Noise static keypair (local) and
// the server's static public key drive a Noise_KK handshake that re-establishes
// a session for the same account.
type ResumeInput struct {
	Account         string
	SessionName     string
	ClientID        string
	DeviceID        string
	ServerPublicKey []byte
	LocalPublicKey  []byte
	LocalPrivateKey []byte
}

// Resume refreshes a DSM session using stored resume keys, with no browser and
// no password. It returns a new live session; the resume keys are unchanged and
// can be reused for the next refresh.
func Resume(ctx context.Context, baseURL string, in ResumeInput, httpClient *http.Client) (Result, error) {
	base, _, err := normalizeBase(baseURL)
	if err != nil {
		return Result{}, err
	}
	if len(in.LocalPrivateKey) == 0 || len(in.ServerPublicKey) == 0 {
		return Result{}, fmt.Errorf("session has no resume keys")
	}
	clientID := firstNonEmpty(in.ClientID, defaultClientID)
	sessionName := firstNonEmpty(in.SessionName, defaultSession)
	if httpClient == nil {
		httpClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	}

	suite := noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)
	handshake, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeKK,
		Initiator:     true,
		StaticKeypair: noise.DHKey{Private: in.LocalPrivateKey, Public: in.LocalPublicKey},
		PeerStatic:    in.ServerPublicKey,
	})
	if err != nil {
		return Result{}, fmt.Errorf("init noise resume handshake: %w", err)
	}
	kkMessage, _, _, err := handshake.WriteMessage(nil, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build noise resume message: %w", err)
	}

	form := url.Values{
		"api":               {"SYNO.API.Auth"},
		"method":            {"resume"},
		"version":           {"7"},
		"client_id":         {clientID},
		"session":           {sessionName},
		"kk_message":        {base64.URLEncoding.EncodeToString(kkMessage)},
		"enable_syno_token": {"yes"},
	}
	if in.Account != "" {
		form.Set("account", in.Account)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/webapi/entry.cgi", strings.NewReader(form.Encode()))
	if err != nil {
		return Result{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return Result{}, fmt.Errorf("resume session: %w", err)
	}
	defer response.Body.Close()

	var decoded struct {
		Success bool `json:"success"`
		Error   *struct {
			Code int `json:"code"`
		} `json:"error"`
		Data struct {
			Account   string `json:"account"`
			SID       string `json:"sid"`
			SynoToken string `json:"synotoken"`
			DeviceID  string `json:"device_id"`
			KKMessage string `json:"kk_message"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxBodySize)).Decode(&decoded); err != nil {
		return Result{}, fmt.Errorf("decode resume response: %w", err)
	}
	if !decoded.Success || decoded.Data.SID == "" {
		code := 0
		if decoded.Error != nil {
			code = decoded.Error.Code
		}
		return Result{}, fmt.Errorf("DSM rejected the session resume (error %d)", code)
	}
	if decoded.Data.KKMessage != "" {
		if serverMessage, err := decodeB64URL(decoded.Data.KKMessage); err == nil {
			_, _, _, _ = handshake.ReadMessage(nil, serverMessage)
		}
	}
	account := decoded.Data.Account
	if account == "" {
		account = in.Account
	}
	deviceID := decoded.Data.DeviceID
	if deviceID == "" {
		deviceID = in.DeviceID
	}
	return Result{
		Account:         account,
		SID:             decoded.Data.SID,
		SynoToken:       decoded.Data.SynoToken,
		DeviceID:        deviceID,
		ServerPublicKey: in.ServerPublicKey,
		LocalPublicKey:  in.LocalPublicKey,
		LocalPrivateKey: in.LocalPrivateKey,
	}, nil
}
