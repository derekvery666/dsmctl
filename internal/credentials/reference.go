package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var environmentReferenceName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type ReferenceResolver interface {
	ResolveSecret(ctx context.Context, reference string) (string, error)
}

type EnvironmentReferenceResolver struct{}

func NewEnvironmentReferenceResolver() *EnvironmentReferenceResolver {
	return &EnvironmentReferenceResolver{}
}

// ResolveSecret deliberately supports references rather than literal secret
// values. The first implementation accepts env:NAME so CLI automation and MCP
// hosts can inject a password without transporting it in a tool argument or
// persisting it in a plan.
func (*EnvironmentReferenceResolver) ResolveSecret(ctx context.Context, reference string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	reference = strings.TrimSpace(reference)
	if !strings.HasPrefix(reference, "env:") {
		return "", errors.New("credential reference must use env:NAME")
	}
	name := strings.TrimPrefix(reference, "env:")
	if !environmentReferenceName.MatchString(name) {
		return "", errors.New("credential environment variable name is invalid")
	}
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", fmt.Errorf("credential environment variable %s is unavailable or empty", name)
	}
	return value, nil
}

// MemoryReferenceResolver resolves in-process secret references of the form
// mem:<id>. It lets a command that generates a secret (provision) hand it to a
// downstream request builder by reference, so the plaintext is never placed in
// a plan, a command argument, an environment variable, or a log line. The
// secrets live only in this process's memory and should be forgotten as soon as
// they have been used.
type MemoryReferenceResolver struct {
	mu     sync.Mutex
	values map[string]string
	seq    uint64
}

func NewMemoryReferenceResolver() *MemoryReferenceResolver {
	return &MemoryReferenceResolver{values: make(map[string]string)}
}

// Put stores a secret and returns its mem:<id> reference.
func (r *MemoryReferenceResolver) Put(secret string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	reference := "mem:" + strconv.FormatUint(r.seq, 10)
	r.values[reference] = secret
	return reference
}

// Forget removes a stored secret. Callers should Forget as soon as the secret
// has been consumed so it does not linger in memory.
func (r *MemoryReferenceResolver) Forget(reference string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, reference)
}

func (r *MemoryReferenceResolver) ResolveSecret(ctx context.Context, reference string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	reference = strings.TrimSpace(reference)
	if !strings.HasPrefix(reference, "mem:") {
		return "", errors.New("credential reference must use mem:ID")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[reference]
	if !ok {
		return "", errors.New("in-memory credential reference is unavailable")
	}
	return value, nil
}

// ChainReferenceResolver tries each resolver in order and returns the first
// that recognizes the reference's scheme. It lets a provisioning invocation
// inject a mem: resolver while keeping env: available for every other request.
func ChainReferenceResolver(resolvers ...ReferenceResolver) ReferenceResolver {
	return chainResolver(resolvers)
}

type chainResolver []ReferenceResolver

func (c chainResolver) ResolveSecret(ctx context.Context, reference string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var lastErr error
	for _, resolver := range c {
		value, err := resolver.ResolveSecret(ctx, reference)
		if err == nil {
			return value, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no credential resolver is configured")
	}
	return "", lastErr
}
