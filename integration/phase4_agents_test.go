//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/cloud-coop/cloudcoop/internal/agent"
)

// TestPhase4_Agents tests agent CRUD operations via tmux.
func TestPhase4_Agents(t *testing.T) {
	if env.vmInfo == nil {
		t.Fatal("VM not created — earlier phases must pass first")
	}

	const sessionName = "agents"

	t.Run("list_empty", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		result, err := agent.ListSessions(client, sessionName)
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}

		if !result.NoSession {
			t.Errorf("Expected NoSession=true on fresh VM, got sessions: %v", result.Sessions)
		}
	})

	t.Run("add_first_agent", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		session, err := agent.CreateSession(client, sessionName, agent.CreateSessionOptions{
			Command: "echo 'test agent 1 running'; sleep 3600",
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		t.Logf("Created agent: index=%d name=%q", session.Index, session.Name)
	})

	t.Run("verify_one_agent", func(t *testing.T) {
		// Brief pause to let tmux stabilize
		time.Sleep(2 * time.Second)

		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		result, err := agent.ListSessions(client, sessionName)
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}

		if result.NoSession {
			t.Fatal("Expected session to exist after adding agent")
		}
		if len(result.Sessions) != 1 {
			t.Errorf("Expected 1 session, got %d: %v", len(result.Sessions), result.Sessions)
		}
	})

	t.Run("add_second_agent", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		session, err := agent.CreateSession(client, sessionName, agent.CreateSessionOptions{
			Command: "echo 'test agent 2 running'; sleep 3600",
		})
		if err != nil {
			t.Fatalf("CreateSession (second): %v", err)
		}
		t.Logf("Created second agent: index=%d name=%q", session.Index, session.Name)
	})

	t.Run("verify_two_agents", func(t *testing.T) {
		time.Sleep(2 * time.Second)

		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		result, err := agent.ListSessions(client, sessionName)
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}

		if len(result.Sessions) != 2 {
			t.Errorf("Expected 2 sessions, got %d: %v", len(result.Sessions), result.Sessions)
		}

		for _, s := range result.Sessions {
			t.Logf("  Window %d: name=%q cmd=%q", s.Index, s.Name, s.Command)
		}
	})

	t.Run("kill_agent", func(t *testing.T) {
		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		err := agent.KillSession(client, sessionName, agent.KillSessionOptions{
			Index: 0,
			Force: true,
		})
		if err != nil {
			t.Fatalf("KillSession: %v", err)
		}
		t.Log("Killed agent at window index 0")
	})

	t.Run("verify_after_kill", func(t *testing.T) {
		time.Sleep(2 * time.Second)

		client := env.connectSSH(t)
		defer func() { _ = client.Close() }()

		result, err := agent.ListSessions(client, sessionName)
		if err != nil {
			t.Fatalf("ListSessions after kill: %v", err)
		}

		if len(result.Sessions) != 1 {
			t.Errorf("Expected 1 session after kill, got %d: %v", len(result.Sessions), result.Sessions)
		}
	})
}
