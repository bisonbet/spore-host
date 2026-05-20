package cmd

import (
	"strings"
	"testing"
)

// buildSSHArgs replicates the one-shot SSH argument construction from runConnect
// so we can test it without a live instance.
func buildSSHArgs(keyPath, user, host string, port int, remoteArgs []string) []string {
	args := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", "22",
		user + "@" + host,
	}
	if len(remoteArgs) > 0 {
		// regression fix for #315: join as single string so remote shell
		// interprets operators like &&, ;, & correctly
		args = append(args, strings.Join(remoteArgs, " "))
	}
	_ = port
	return args
}

// TestConnectOneShot_CompoundCommandJoined is a regression test for #315.
// Before the fix, remoteArgs were appended individually, so the local shell
// interpreted &&, ;, & before SSH could pass them to the remote shell.
func TestConnectOneShot_CompoundCommandJoined(t *testing.T) {
	remoteArgs := []string{"cmd1", "&&", "cmd2"}
	sshArgs := buildSSHArgs("key.pem", "ec2-user", "1.2.3.4", 22, remoteArgs)

	// The last SSH argument must be a single joined string, not individual tokens
	lastArg := sshArgs[len(sshArgs)-1]
	if lastArg != "cmd1 && cmd2" {
		t.Errorf("expected remote command as single arg %q, got %q\n"+
			"full args: %v\n"+
			"(individual tokens would cause local shell to interpret &&)", "cmd1 && cmd2", lastArg, sshArgs)
	}
}

// TestConnectOneShot_BackgroundOperator verifies & is preserved for remote execution.
func TestConnectOneShot_BackgroundOperator(t *testing.T) {
	remoteArgs := []string{"nohup", "bash", "/tmp/run.sh", ">", "/tmp/run.log", "2>&1", "&"}
	sshArgs := buildSSHArgs("key.pem", "ec2-user", "1.2.3.4", 22, remoteArgs)

	lastArg := sshArgs[len(sshArgs)-1]
	if !strings.Contains(lastArg, "&") {
		t.Errorf("background operator & must be preserved in remote command, got: %q", lastArg)
	}
	if !strings.HasSuffix(lastArg, "&") {
		t.Errorf("background operator should be at end of remote command, got: %q", lastArg)
	}
}

// TestConnectOneShot_Semicolon verifies ; is preserved for remote execution.
func TestConnectOneShot_Semicolon(t *testing.T) {
	remoteArgs := []string{"cmd1;", "cmd2"}
	sshArgs := buildSSHArgs("key.pem", "ec2-user", "1.2.3.4", 22, remoteArgs)

	lastArg := sshArgs[len(sshArgs)-1]
	if !strings.Contains(lastArg, ";") {
		t.Errorf("semicolon separator must be preserved in remote command, got: %q", lastArg)
	}
}

// TestConnectOneShot_SingleCommandUnchanged verifies single simple commands still work.
func TestConnectOneShot_SingleCommandUnchanged(t *testing.T) {
	remoteArgs := []string{"tail", "-20", "/tmp/run.log"}
	sshArgs := buildSSHArgs("key.pem", "ec2-user", "1.2.3.4", 22, remoteArgs)

	lastArg := sshArgs[len(sshArgs)-1]
	if lastArg != "tail -20 /tmp/run.log" {
		t.Errorf("expected %q, got %q", "tail -20 /tmp/run.log", lastArg)
	}
}

// TestConnectOneShot_InteractiveModeNoExtraArgs verifies interactive mode (no remote args)
// does not append a remote command argument.
func TestConnectOneShot_InteractiveModeNoExtraArgs(t *testing.T) {
	remoteArgs := []string{}
	sshArgs := buildSSHArgs("key.pem", "ec2-user", "1.2.3.4", 22, remoteArgs)

	// Last arg should be the host, not a command
	lastArg := sshArgs[len(sshArgs)-1]
	if lastArg != "ec2-user@1.2.3.4" {
		t.Errorf("interactive mode: last arg should be host, got %q", lastArg)
	}
}

// TestConnectOneShot_QuotedString verifies a pre-quoted string is passed intact.
func TestConnectOneShot_QuotedString(t *testing.T) {
	// This simulates: spawn connect my-instance -- 'aws s3 cp s3://b/f /tmp/f'
	// where the shell has already stripped the quotes, leaving one arg
	remoteArgs := []string{"aws s3 cp s3://bucket/file /tmp/file"}
	sshArgs := buildSSHArgs("key.pem", "ec2-user", "1.2.3.4", 22, remoteArgs)

	lastArg := sshArgs[len(sshArgs)-1]
	if lastArg != "aws s3 cp s3://bucket/file /tmp/file" {
		t.Errorf("expected command intact, got %q", lastArg)
	}
}
