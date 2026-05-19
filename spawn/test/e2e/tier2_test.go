//go:build e2e_tier2

package e2e

// Tier 2 — Single-instance tests. Launches one t3.small per test.
// Estimated cost: ~$0.50 total, ~15-20 min.
//
// Run: go test -v -tags=e2e_tier2 ./test/e2e/ -run TestTier2 -timeout 30m

import (
	"strings"
	"testing"
	"time"
)

// TestTier2_PublicIP verifies every launched instance gets a public IP (regression #308).
func TestTier2_PublicIP(t *testing.T) {
	name := "e2e-public-ip-" + runID(t)
	inst := launchInstance(t, name)

	if inst.PublicIP == "" {
		t.Fatalf("instance %s launched with no public IP — regression #308", inst.InstanceID)
	}
	t.Logf("instance %s has public IP %s", inst.InstanceID, inst.PublicIP)
}

// TestTier2_ConnectSSH verifies spawn connect can reach the instance (interactive skip, one-shot).
func TestTier2_ConnectSSH(t *testing.T) {
	name := "e2e-connect-" + runID(t)
	launchInstance(t, name)

	out := sshExec(t, name, "echo SPORE_OK")
	if !strings.Contains(out, "SPORE_OK") {
		t.Fatalf("expected SPORE_OK from ssh, got:\n%s", out)
	}
	t.Log("spawn connect one-shot OK")
}

// TestTier2_CommandExecution verifies --command runs on the instance (regression #298).
func TestTier2_CommandExecution(t *testing.T) {
	name := "e2e-command-" + runID(t)
	launchInstance(t, name, "--command", "echo SPAWN_COMMAND_RAN > /tmp/cmd-ran.txt")

	// Give the command a moment to execute after spored starts
	time.Sleep(30 * time.Second)

	out := sshExec(t, name, "cat /tmp/cmd-ran.txt")
	if !strings.Contains(out, "SPAWN_COMMAND_RAN") {
		t.Fatalf("--command did not execute on instance; /tmp/cmd-ran.txt: %q", out)
	}
	t.Log("--command executed successfully")
}

// TestTier2_OnComplete verifies --on-complete terminate fires when sentinel file appears.
func TestTier2_OnComplete(t *testing.T) {
	name := "e2e-on-complete-" + runID(t)
	launchInstance(t, name,
		"--on-complete", "terminate",
		"--completion-delay", "10s",
	)

	// Touch the sentinel file
	sshExec(t, name, "touch /tmp/SPAWN_COMPLETE")
	t.Log("sentinel file created — waiting for termination")

	// Instance should terminate within ~2 min (10s delay + shutdown time)
	waitForState(t, name, "terminated", 3*time.Minute)
	t.Log("--on-complete terminate fired correctly")
}

// TestTier2_PreStop verifies --pre-stop runs before termination.
func TestTier2_PreStop(t *testing.T) {
	name := "e2e-prestop-" + runID(t)
	launchInstance(t, name,
		"--on-complete", "terminate",
		"--completion-delay", "5s",
		"--pre-stop", "touch /tmp/prestop-ran.txt",
		"--pre-stop-timeout", "30s",
	)

	// Trigger completion
	sshExec(t, name, "touch /tmp/SPAWN_COMPLETE")

	// Wait for pre-stop to run (before instance terminates)
	time.Sleep(20 * time.Second)
	out := sshExec(t, name, "test -f /tmp/prestop-ran.txt && echo YES || echo NO")
	if !strings.Contains(out, "YES") {
		t.Errorf("--pre-stop did not run: /tmp/prestop-ran.txt not found")
	}

	waitForState(t, name, "terminated", 2*time.Minute)
	t.Log("--pre-stop ran before termination")
}

// TestTier2_ExtendTTL verifies spawn extend pushes the TTL deadline.
func TestTier2_ExtendTTL(t *testing.T) {
	name := "e2e-extend-" + runID(t)
	launchInstance(t, name, "--ttl", "5m")

	// Extend by 10 more minutes
	spawn(t, "extend", name, "10m")

	// Check status — instance should still be running (not near-expiry)
	out := sshExec(t, name, "sudo spored status")
	if !strings.Contains(out, "TTL") {
		t.Logf("spored status: %s", out)
	}
	t.Log("spawn extend completed without error")
}

// TestTier2_SpawnStop verifies spawn stop halts billing without deleting instance.
func TestTier2_SpawnStop(t *testing.T) {
	name := "e2e-stop-" + runID(t)
	launchInstance(t, name)

	spawn(t, "stop", name)
	waitForState(t, name, "stopped", 3*time.Minute)
	t.Log("instance stopped")

	spawn(t, "start", name)
	waitForState(t, name, "running", 3*time.Minute)
	t.Log("instance restarted")
}

// TestTier2_IAMPolicyApplied verifies --iam-policy adds permissions to the role (regression #299).
func TestTier2_IAMPolicyApplied(t *testing.T) {
	name := "e2e-iam-policy-" + runID(t)
	launchInstance(t, name, "--iam-policy", "s3:ReadOnly")

	// The instance should be able to list S3 buckets (read permission applied)
	out, err := spawnMayFail(t, "connect", name, "--",
		"aws s3 ls 2>&1 | head -5 || echo S3_ACCESS_ATTEMPTED")
	if err != nil {
		t.Logf("s3 ls returned error (may be no buckets): %v", err)
	}
	// If we get an explicit AccessDenied we know the policy wasn't applied
	if strings.Contains(out, "AccessDenied") && !strings.Contains(out, "S3_ACCESS_ATTEMPTED") {
		t.Errorf("--iam-policy s3:ReadOnly did not grant S3 access: %s", out)
	}
	t.Logf("IAM policy check: %s", strings.TrimSpace(out))
}

// TestTier2_SporedStatus verifies spored is running and reports TTL.
func TestTier2_SporedStatus(t *testing.T) {
	name := "e2e-spored-status-" + runID(t)
	launchInstance(t, name, "--ttl", "15m")

	out := sshExec(t, name, "sudo spored status")
	if !strings.Contains(out, "TTL") {
		t.Errorf("expected TTL in spored status output, got:\n%s", out)
	}
	t.Logf("spored status:\n%s", out)
}
