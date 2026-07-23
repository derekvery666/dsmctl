package synology

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/derekvery666/dsmctl/internal/domain/snapshotreplication"
	snapshotops "github.com/derekvery666/dsmctl/internal/synology/operations/snapshotreplication"
)

// PairingEndpoint is the destination NAS management endpoint a source NAS
// connects to when it authenticates the DR pairing by account. It carries only
// the address, port, and TLS flag — no session material and no password.
type PairingEndpoint struct {
	Addr  string
	Port  int
	HTTPS bool
}

// ReplicationDestinationEndpoint returns this (destination) client's management
// endpoint (address, port, TLS), derived from its base URL without logging in.
// The source NAS's account-based pairing authenticates to it directly; it needs
// no destination session, so no credential is resolved here.
func (c *Client) ReplicationDestinationEndpoint(_ context.Context) (PairingEndpoint, error) {
	endpoint := PairingEndpoint{
		Addr:  c.baseURL.Hostname(),
		HTTPS: c.baseURL.Scheme == "https",
	}
	if endpoint.HTTPS {
		endpoint.Port = 5001
	} else {
		endpoint.Port = 5000
	}
	if raw := c.baseURL.Port(); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			endpoint.Port = parsed
		}
	}
	if endpoint.Addr == "" {
		return PairingEndpoint{}, fmt.Errorf("destination endpoint address is empty")
	}
	return endpoint, nil
}

// ReplicationAccountCredential is a destination profile's account credential,
// resolved from the vault only at apply time for account-based DR pairing
// (temp_create auth:"account"). It is passed to the source NAS's pairing call
// and never enters a plan, its hash, logs, or MCP output.
type ReplicationAccountCredential struct {
	Account  string
	Password string
	OTPCode  string
}

// ReplicationAccountCredential resolves this (destination) client's account
// credential from its vault profile for an apply-time account-based pairing.
// The password is resolved lazily (the same resolver a login would use) and
// returned only to the in-process apply path. A destination account that
// requires interactive 2FA is not supported for headless pairing: no OTP is
// resolved here, and DSM answers such a pairing with an "OTP required" error.
func (c *Client) ReplicationAccountCredential(ctx context.Context) (ReplicationAccountCredential, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	account := strings.TrimSpace(c.username)
	if account == "" {
		return ReplicationAccountCredential{}, fmt.Errorf("destination profile has no account for replication pairing")
	}
	password := c.password
	if password == "" && c.passwordFunc != nil {
		resolved, err := c.passwordFunc(ctx)
		if err != nil {
			return ReplicationAccountCredential{}, fmt.Errorf("resolve destination password: %w", err)
		}
		password = resolved
	}
	if password == "" {
		return ReplicationAccountCredential{}, fmt.Errorf("destination profile %q has no password available for account-based replication pairing", account)
	}
	return ReplicationAccountCredential{Account: account, Password: password}, nil
}

// Re-exported types for the application layer.
type SnapshotReplicationRelationCreate = snapshotreplication.RelationCreate
type SnapshotReplicationPairEndpoint = snapshotops.PairEndpoint
type SnapshotReplicationCreateResult = snapshotops.CreateResult

// PairReplicationCredential establishes a durable DR credential on this (source)
// client for the given destination endpoint, authenticating by the destination
// account credential (temp_create auth:"account"), and returns the cred_id the
// create call consumes.
func (c *Client) PairReplicationCredential(ctx context.Context, endpoint SnapshotReplicationPairEndpoint, cred ReplicationAccountCredential) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return "", err
	}
	credID, _, err := snapshotops.ExecuteReplicationTempCredential(ctx, c.target, lockedExecutor{client: c}, endpoint, cred.Account, cred.Password, cred.OTPCode)
	if err != nil {
		return "", fmt.Errorf("pair replication credential: %w", err)
	}
	return credID, nil
}

// CheckReplicationRemoteConn verifies source→destination reachability for the
// given destination endpoint + credential before any relation is created.
func (c *Client) CheckReplicationRemoteConn(ctx context.Context, endpoint SnapshotReplicationPairEndpoint, credID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return err
	}
	if _, err := snapshotops.ExecuteReplicationCheckRemoteConn(ctx, c.target, lockedExecutor{client: c}, endpoint, credID); err != nil {
		return fmt.Errorf("check replication remote connection: %w", err)
	}
	return nil
}

// CreateReplicationPlan creates a share replication relation from this (source)
// client to the destination described by the credential + endpoint, returning
// the async task id.
func (c *Client) CreateReplicationPlan(ctx context.Context, input snapshotreplication.RelationCreate, endpoint SnapshotReplicationPairEndpoint, credID string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return "", err
	}
	taskID, _, err := snapshotops.ExecuteReplicationCreate(ctx, c.target, lockedExecutor{client: c}, input, endpoint, credID)
	if err != nil {
		return "", fmt.Errorf("create replication plan: %w", err)
	}
	return taskID, nil
}

// PollReplicationTask reads one poll of an in-flight create task.
func (c *Client) PollReplicationTask(ctx context.Context, taskID string) (snapshotreplication.RelationTaskStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return snapshotreplication.RelationTaskStatus{}, err
	}
	status, _, err := snapshotops.ExecuteReplicationPollTask(ctx, c.target, lockedExecutor{client: c}, taskID)
	if err != nil {
		return snapshotreplication.RelationTaskStatus{}, fmt.Errorf("poll replication task: %w", err)
	}
	return status, nil
}

// DeleteReplicationPlan removes a replication relation by plan id (teardown).
func (c *Client) DeleteReplicationPlan(ctx context.Context, planID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return err
	}
	if _, err := snapshotops.ExecuteReplicationDelete(ctx, c.target, lockedExecutor{client: c}, planID); err != nil {
		return fmt.Errorf("delete replication plan: %w", err)
	}
	return nil
}

// DeleteReplicationCredential removes a temporary DR credential (cleanup after
// a failed create).
func (c *Client) DeleteReplicationCredential(ctx context.Context, credID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return err
	}
	if _, err := snapshotops.ExecuteReplicationDeleteCredential(ctx, c.target, lockedExecutor{client: c}, credID); err != nil {
		return fmt.Errorf("delete replication credential: %w", err)
	}
	return nil
}

// SyncReplicationPlan triggers a manual sync of an existing relation by plan id.
func (c *Client) SyncReplicationPlan(ctx context.Context, planID string, sendEncrypted bool, description string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return err
	}
	if _, err := snapshotops.ExecuteReplicationSync(ctx, c.target, lockedExecutor{client: c}, snapshotops.SyncInput{
		PlanID: planID, SnapshotLocked: false, SendEncrypted: sendEncrypted, Description: description,
	}); err != nil {
		return fmt.Errorf("sync replication plan: %w", err)
	}
	return nil
}

// PauseReplicationPlan stops (pauses) replication for an existing relation.
func (c *Client) PauseReplicationPlan(ctx context.Context, planID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.prepareSnapshotReplicationTargetLocked(ctx); err != nil {
		return err
	}
	if _, err := snapshotops.ExecuteReplicationPause(ctx, c.target, lockedExecutor{client: c}, planID); err != nil {
		return fmt.Errorf("pause replication plan: %w", err)
	}
	return nil
}

