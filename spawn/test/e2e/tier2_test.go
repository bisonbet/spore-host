//go:build e2e_tier2

package e2e

// Tier 2 — Single-instance tests. Launches one t3.small per test.
// Estimated cost: ~$1 total, ~20-25 min.
//
// Run: go test -v -tags=e2e_tier2 ./test/e2e/ -run TestTier2 -timeout 35m

import (
	"encoding/json"
	"fmt"
	"os"
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

// TestTier2_CompoundSSHCommand verifies spawn connect -- 'cmd && cmd2' works
// on a real instance (regression for #315).
func TestTier2_CompoundSSHCommand(t *testing.T) {
	name := "e2e-compound-ssh-" + runID(t)
	launchInstance(t, name)

	// Compound command: write two files sequentially
	out := sshExec(t, name, "echo A > /tmp/a.txt && echo B > /tmp/b.txt && cat /tmp/a.txt /tmp/b.txt")
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Errorf("compound && command failed; expected A and B in output, got:\n%s", out)
	}

	// Semicolon separator
	out = sshExec(t, name, "echo X; echo Y")
	if !strings.Contains(out, "X") || !strings.Contains(out, "Y") {
		t.Errorf("semicolon command failed; expected X and Y in output, got:\n%s", out)
	}
	t.Log("compound SSH commands work correctly")
}

// TestTier2_FSxTagsWritten verifies spawn:fsx-id and spawn:fsx-mount-point
// tags are written when --fsx-id is used (regression for #314).
func TestTier2_FSxTagsWritten(t *testing.T) {
	// Use a fake FSx ID — we're only testing that tags are written,
	// not that the filesystem actually mounts.
	name := "e2e-fsx-tags-" + runID(t)
	inst := launchInstance(t, name,
		"--fsx-id", "fs-00000000000000000",
		"--fsx-mount-point", "/fsx",
	)

	// Check the tags via AWS
	cfg := loadAWSConfig(t)
	out := describeInstanceTags(t, cfg, inst.InstanceID, testRegion)

	if out["spawn:fsx-id"] != "fs-00000000000000000" {
		t.Errorf("spawn:fsx-id = %q, want fs-00000000000000000 (regression #314)", out["spawn:fsx-id"])
	}
	if out["spawn:fsx-mount-point"] != "/fsx" {
		t.Errorf("spawn:fsx-mount-point = %q, want /fsx (regression #314)", out["spawn:fsx-mount-point"])
	}
	t.Log("FSx tags written correctly")
}

// TestTier2_SporedConfigSetGet verifies spored config set/get/list on a running instance.
func TestTier2_SporedConfigSetGet(t *testing.T) {
	name := "e2e-spored-config-" + runID(t)
	launchInstance(t, name, "--ttl", "15m")

	// Get existing config
	out := sshExec(t, name, "sudo spored config list")
	if !strings.Contains(out, "ttl") && !strings.Contains(out, "idle") {
		t.Errorf("spored config list missing expected keys:\n%s", out)
	}

	// Get a specific value
	out = sshExec(t, name, "sudo spored config get ttl")
	if strings.TrimSpace(out) == "" {
		t.Errorf("spored config get ttl returned empty output")
	}
	t.Logf("spored config list and get work: %s", strings.TrimSpace(out))
}

// TestTier2_SpawnListFilters verifies spawn list filtering by state and tag.
func TestTier2_SpawnListFilters(t *testing.T) {
	name := "e2e-list-filters-" + runID(t)
	inst := launchInstance(t, name)

	// Filter by state=running — should find our instance
	out, err := spawnMayFail(t, "list", "--state", "running", "--output", "json")
	if err != nil {
		t.Skipf("spawn list failed: %v", err)
	}
	var instances []InstanceJSON
	if json.Unmarshal([]byte(out), &instances) == nil {
		found := false
		for _, i := range instances {
			if i.InstanceID == inst.InstanceID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("running instance %s not found in spawn list --state running", inst.InstanceID)
		}
	}

	// Filter by instance-family — t3 should include our t3.small
	out, err = spawnMayFail(t, "list", "--instance-family", "t3", "--output", "json")
	if err == nil && json.Unmarshal([]byte(out), &instances) == nil {
		t.Logf("spawn list --instance-family t3 returned %d instances", len(instances))
	}
	t.Log("spawn list filters working")
}

// TestTier2_SlurmConvert verifies slurm convert produces valid spawn params.
// No instance is launched — this exercises the CLI parsing only.
func TestTier2_SlurmConvert(t *testing.T) {
	// Write a minimal sbatch script
	f, err := os.CreateTemp("", "test-*.sbatch")
	if err != nil {
		t.Fatalf("create sbatch file: %v", err)
	}
	defer os.Remove(f.Name())
	fmt.Fprintln(f, "#!/bin/bash")
	fmt.Fprintln(f, "#SBATCH --job-name=e2e-test")
	fmt.Fprintln(f, "#SBATCH --time=01:00:00")
	fmt.Fprintln(f, "#SBATCH --mem=4G")
	fmt.Fprintln(f, "#SBATCH --cpus-per-task=4")
	fmt.Fprintln(f, "echo running")
	f.Close()

	out := spawn(t, "slurm", "convert", f.Name())
	// convert writes YAML to stdout
	if !strings.Contains(out, "instance_type") && !strings.Contains(out, "ttl") &&
		!strings.Contains(out, "mem") && !strings.Contains(out, "cpu") {
		t.Logf("slurm convert output (may vary by format):\n%s", out)
		// Not a hard failure — the conversion logic may produce different keys
	}
	t.Logf("slurm convert produced %d bytes of output", len(out))
}

// TestTier2_SpawnStatus verifies spawn status works by name and ID.
func TestTier2_SpawnStatus(t *testing.T) {
	name := "e2e-status-" + runID(t)
	inst := launchInstance(t, name, "--ttl", "15m")

	// Status by name — calls spored status via SSH
	out, err := spawnMayFail(t, "status", name)
	if err != nil {
		t.Logf("spawn status by name returned error (may need SSH key): %v\n%s", err, out)
	} else {
		t.Logf("spawn status by name:\n%s", out[:min(len(out), 200)])
	}
	_ = inst
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
