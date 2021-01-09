package metadatatesting_test

import (
	"testing"

	"go.jonnrb.io/egress/backend/kubernetes/metadata"
	metadatatesting "go.jonnrb.io/egress/backend/kubernetes/metadata/testing"
)

func TestInstall(t *testing.T) {
	var iAmOld bool
	metadata.GetAttachments = func() (string, error) {
		iAmOld = true
		return "", nil
	}
	s := metadatatesting.Stub{
		metadatatesting.Install(&metadata.GetPodName, "myawesomepod"),
	}

	podName, err := metadata.GetPodName()

	if err != nil {
		t.Errorf("expected err == nil; got err: %v", err)
	}
	if podName != "myawesomepod" {
		t.Errorf(
			"expected podName == %q; got podName: %q", "myawesomepod", podName)
	}
	s.Uninstall()
	if iAmOld {
		t.Error("old metadata.GetPodName was unexpectedly called")
	}
	if _, _ = metadata.GetPodName(); iAmOld {
		t.Error("metadata.GetPodName not restored")
	}
}
