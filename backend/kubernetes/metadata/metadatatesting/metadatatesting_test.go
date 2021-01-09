package metadatatesting_test

import (
	"errors"
	"testing"

	"go.jonnrb.io/egress/backend/kubernetes/metadata"
	"go.jonnrb.io/egress/backend/kubernetes/metadata/metadatatesting"
)

func TestInstall(t *testing.T) {
	var iAmOld bool
	metadata.GetAttachments = func() (string, error) {
		iAmOld = true
		return "", nil
	}
	var s metadatatesting.Stub
	checkUninstall := func(t *testing.T) {
		s.Uninstall()
		if iAmOld {
			t.Error("old metadata.GetPodName was unexpectedly called")
		}
		iAmOld = false
		if _, _ = metadata.GetPodName(); iAmOld {
			t.Error("metadata.GetPodName not restored")
		}
	}

	t.Run("Install", func(t *testing.T) {
		s = metadatatesting.Stub{
			metadatatesting.Install(&metadata.GetPodName, "myawesomepod"),
		}
		defer checkUninstall(t)

		podName, err := metadata.GetPodName()

		if err != nil {
			t.Errorf("expected err == nil; got err: %v", err)
		}
		if podName != "myawesomepod" {
			t.Errorf(
				"expected podName == %q; got podName: %q",
				"myawesomepod",
				podName)
		}
	})

	t.Run("InstallError", func(t *testing.T) {
		stubErr := errors.New("stubErr")
		s = metadatatesting.Stub{
			metadatatesting.InstallError(&metadata.GetPodName, stubErr),
		}
		defer checkUninstall(t)

		_, err := metadata.GetPodName()

		if err != stubErr {
			t.Errorf("expected err == stubErr; got: %v", err)
		}
	})

	t.Run("InstallPanic", func(t *testing.T) {
		stubPanic := "oops"
		s = metadatatesting.Stub{
			metadatatesting.InstallPanic(&metadata.GetPodName, stubPanic),
		}
		defer checkUninstall(t)

		var v interface{}
		func() {
			defer func() {
				v = recover()
			}()

			_, _ = metadata.GetPodName()
			t.Error("should have panicked")
		}()

		if v != stubPanic {
			t.Errorf("expected v = %v; got: %v", stubPanic, v)
		}
	})
}
