package registry

import (
	"testing"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
)

func TestRegisterPushReceive(t *testing.T) {
	r := New()
	jobs, release := r.Register("c1")
	defer release()

	if !r.IsOnline("c1") {
		t.Fatal("c1 should be online after Register")
	}
	if r.IsOnline("c2") {
		t.Fatal("c2 should not be online")
	}

	if ok := r.Push("c1", &porukatorv1.Job{MessageId: "m1"}); !ok {
		t.Fatal("push to online client should succeed")
	}
	if got := <-jobs; got.MessageId != "m1" {
		t.Fatalf("got %q, want m1", got.MessageId)
	}
}

func TestPushOfflineFails(t *testing.T) {
	r := New()
	if r.Push("ghost", &porukatorv1.Job{MessageId: "x"}) {
		t.Fatal("push to offline client must fail")
	}
}

func TestReleaseMarksOffline(t *testing.T) {
	r := New()
	_, release := r.Register("c1")
	release()
	if r.IsOnline("c1") {
		t.Fatal("c1 should be offline after release")
	}
}

func TestReconnectSupersedesOldStream(t *testing.T) {
	r := New()
	oldJobs, oldRelease := r.Register("c1")
	defer oldRelease()

	// Second connect for same client supersedes the first; old channel closes.
	newJobs, newRelease := r.Register("c1")
	defer newRelease()

	if _, ok := <-oldJobs; ok {
		t.Fatal("old channel should be closed on reconnect")
	}
	if ok := r.Push("c1", &porukatorv1.Job{MessageId: "m"}); !ok {
		t.Fatal("push should reach the new stream")
	}
	if got := <-newJobs; got.MessageId != "m" {
		t.Fatalf("got %q, want m", got.MessageId)
	}
}

func TestOnlineSet(t *testing.T) {
	r := New()
	_, rel1 := r.Register("a")
	defer rel1()
	_, rel2 := r.Register("b")
	defer rel2()

	set := r.OnlineSet()
	if !set["a"] || !set["b"] || len(set) != 2 {
		t.Fatalf("unexpected online set: %v", set)
	}
}
